// Package api contains HTTP handlers for the cloud-provisioner service.
package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/pragatinarote/cloud-provisioner/internal/model"
	"github.com/pragatinarote/cloud-provisioner/internal/store"
)

// Handler serves the environment-management HTTP endpoints. It depends
// on the store.Store interface rather than on *store.MemoryStore
// directly, so the same handler code keeps working unchanged if
// MemoryStore is ever replaced with a database-backed store later.
type Handler struct {
	store store.Store
}

// NewHandler builds a Handler backed by the given store. The store is
// passed in (dependency injection) rather than created here, so every
// request served by this Handler shares the exact same store instance.
func NewHandler(environmentStore store.Store) *Handler {
	return &Handler{store: environmentStore}
}

// generateID creates a short, random environment ID such as
// "env-a3f91c20". crypto/rand gives us cryptographically random bytes,
// which we render as hexadecimal text so the ID is safe to put in a URL.
// Random IDs make accidental collisions with an existing ID astronomically
// unlikely, though not mathematically impossible — the caller still
// handles a "already exists" error from the store just in case.
func generateID() (string, error) {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "env-" + hex.EncodeToString(raw), nil
}

// CreateEnvironment handles POST /environments. It decodes and
// validates the request, generates a server-side ID, builds an
// Environment starting at PENDING, and saves it through the store.
func (h *Handler) CreateEnvironment(w http.ResponseWriter, r *http.Request) {
	var req model.CreateEnvironmentRequest

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	// decoder.More reports whether there is unread data left after the
	// first JSON value — that catches trailing garbage like
	// `{"name":"x"} oops` that Decode alone would silently ignore.
	if decoder.More() {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	id, err := generateID()
	if err != nil {
		log.Printf("failed to generate environment id: %v", err)
		writeError(w, http.StatusInternalServerError, "unable to create environment")
		return
	}

	now := time.Now().UTC()
	environment := model.Environment{
		ID:        id,
		Name:      req.Name,
		Region:    req.Region,
		Services:  req.Services,
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// The store clones environment.Services internally when saving, so
	// the request's original slice can never later reach in and modify
	// stored data — we don't need to clone it again here ourselves.
	if err := h.store.Create(environment); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		log.Printf("failed to create environment %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "unable to create environment")
		return
	}

	log.Printf("environment created: id=%s name=%s", environment.ID, environment.Name)
	writeJSON(w, http.StatusAccepted, environment)
}

// ListEnvironments handles GET /environments. It always returns 200 OK,
// even when there are no environments yet — an empty collection is not
// an error, just an empty result.
func (h *Handler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
	environments := h.store.List()
	log.Printf("environments listed: count=%d", len(environments))
	writeJSON(w, http.StatusOK, environments)
}

// GetEnvironment handles GET /environments/{id}.
func (h *Handler) GetEnvironment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "environment id is required")
		return
	}

	environment, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		log.Printf("failed to get environment %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "unable to retrieve environment")
		return
	}

	log.Printf("environment retrieved: id=%s", id)
	writeJSON(w, http.StatusOK, environment)
}

// DeleteEnvironment handles DELETE /environments/{id}. It removes the
// in-memory record immediately — no real cloud resources exist yet, so
// there is nothing external to tear down. A future task may instead
// mark the environment DELETING and remove it only after asynchronous
// cleanup finishes; this handler does not do that yet.
func (h *Handler) DeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "environment id is required")
		return
	}

	if err := h.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		log.Printf("failed to delete environment %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "unable to delete environment")
		return
	}

	log.Printf("environment deleted: id=%s", id)
	// 204 No Content must not have a body, so we only write the status.
	w.WriteHeader(http.StatusNoContent)
}
