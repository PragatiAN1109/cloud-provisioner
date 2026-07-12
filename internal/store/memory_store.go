// Package store provides a thread-safe, in-memory place to keep
// Environment records while the server process is running. There is no
// database yet — everything lives in a Go map, and disappears when the
// program stops. That's fine for this stage of the project.
package store

import (
	"errors"
	"fmt"
	"sync"

	"github.com/pragatinarote/cloud-provisioner/internal/model"
)

// Sentinel errors describe the general category of a storage failure.
// Callers (and, later, an HTTP handler) can check for these with
// errors.Is to decide how to respond, without parsing error text.
var (
	ErrEmptyID       = errors.New("environment ID is required")
	ErrAlreadyExists = errors.New("environment already exists")
	ErrNotFound      = errors.New("environment not found")
)

// Store describes the storage operations a cloud environment needs.
// It only lists behavior (method signatures), not how that behavior is
// implemented, so callers can depend on "something that behaves like a
// Store" rather than on a specific storage technology.
type Store interface {
	Create(environment model.Environment) error
	Get(id string) (model.Environment, error)
	List() []model.Environment
	Update(environment model.Environment) error
	Delete(id string) error
}

// MemoryStore is an in-memory implementation of Store. It keeps every
// Environment in a Go map, guarded by a read-write mutex so it can be
// safely used from multiple goroutines at once (for example, multiple
// HTTP requests being handled concurrently in a later task).
type MemoryStore struct {
	mu           sync.RWMutex
	environments map[string]model.Environment
}

// NewMemoryStore creates a MemoryStore that is ready to use immediately.
// The map is initialized here with make, so callers never have to worry
// about writing into a nil map.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		environments: make(map[string]model.Environment),
	}
}

// cloneEnvironment returns a copy of env whose Services slice does not
// share the same underlying array as env.Services. Copying the struct
// alone would copy the slice header (pointer, length, capacity), but not
// the data it points to — so both copies would still see changes made
// through either one. Cloning the slice's contents breaks that link.
func cloneEnvironment(env model.Environment) model.Environment {
	clone := env
	clone.Services = append([]string(nil), env.Services...)
	return clone
}

// Create stores a new environment. It rejects an empty ID and a
// duplicate ID; it does not validate the rest of the fields, since that
// belongs to CreateEnvironmentRequest.Validate() in the model package,
// not to storage.
func (s *MemoryStore) Create(environment model.Environment) error {
	if environment.ID == "" {
		return ErrEmptyID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.environments[environment.ID]; exists {
		return fmt.Errorf("%w: %s", ErrAlreadyExists, environment.ID)
	}

	s.environments[environment.ID] = cloneEnvironment(environment)
	return nil
}

// Get retrieves one environment by ID. It returns a cloned copy, not a
// reference into the store's internal map, so callers can never change
// stored data except by calling Update.
func (s *MemoryStore) Get(id string) (model.Environment, error) {
	if id == "" {
		return model.Environment{}, ErrEmptyID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	environment, ok := s.environments[id]
	if !ok {
		return model.Environment{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}

	return cloneEnvironment(environment), nil
}

// List returns every stored environment as a slice of clones. It never
// returns nil — if the store is empty, it returns an empty slice, so
// callers can safely range over the result without a nil check. Order
// is not guaranteed, because Go map iteration order is randomized.
func (s *MemoryStore) List() []model.Environment {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]model.Environment, 0, len(s.environments))
	for _, environment := range s.environments {
		result = append(result, cloneEnvironment(environment))
	}
	return result
}

// Update replaces an existing environment. It does not create a new
// entry if the ID is missing — that is the difference between Update
// and an "upsert" (which would create-or-replace). Rejecting missing
// IDs here keeps Create as the only way a new environment comes to exist.
func (s *MemoryStore) Update(environment model.Environment) error {
	if environment.ID == "" {
		return ErrEmptyID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.environments[environment.ID]; !exists {
		return fmt.Errorf("%w: %s", ErrNotFound, environment.ID)
	}

	s.environments[environment.ID] = cloneEnvironment(environment)
	return nil
}

// Delete removes an environment from memory by ID. This only erases the
// in-memory record — no real cloud infrastructure exists in this
// project yet, so there is nothing external to tear down. A later task
// may instead mark an environment DELETING/DELETED before removing it,
// to reflect that real cloud teardown takes time; this store does not
// do that yet.
func (s *MemoryStore) Delete(id string) error {
	if id == "" {
		return ErrEmptyID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.environments[id]; !exists {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}

	delete(s.environments, id)
	return nil
}
