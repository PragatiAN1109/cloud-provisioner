// Package api contains HTTP handlers for the cloud-provisioner service.
package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// errorResponse is the consistent JSON shape returned for every API
// error, so a client always knows to look for the same "error" key.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON sets the response Content-Type, writes the given status
// code, and encodes value as the JSON response body. Headers and the
// status must be set before any body bytes are written — HTTP only
// allows one status code per response, and Go locks it in as soon as
// the first byte is written (implicitly calling WriteHeader(200) if it
// hasn't been called yet).
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if value == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to write JSON response: %v", err)
	}
}

// writeError writes a consistent {"error": "..."} JSON body with the
// given HTTP status code.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
