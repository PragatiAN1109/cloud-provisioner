// Package api contains HTTP handlers for the cloud-provisioner service.
package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// healthResponse is the shape of the JSON body returned by the health check.
type healthResponse struct {
	Status string `json:"status"`
}

// HealthHandler responds to GET /health with a small JSON payload that
// tells the caller the service is up and able to respond to requests.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := healthResponse{Status: "ok"}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to write health response: %v", err)
	}
}
