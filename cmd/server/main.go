// Command server starts the cloud-provisioner HTTP server.
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/pragatinarote/cloud-provisioner/internal/api"
	"github.com/pragatinarote/cloud-provisioner/internal/store"
)

func main() {
	// One shared store, created once here at startup. Every request,
	// regardless of which handler method serves it, uses this exact
	// same instance — that's what lets a POST-created environment
	// actually show up in a later GET.
	environmentStore := store.NewMemoryStore()
	handler := api.NewHandler(environmentStore)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", api.HealthHandler)
	mux.HandleFunc("POST /environments", handler.CreateEnvironment)
	mux.HandleFunc("GET /environments", handler.ListEnvironments)
	mux.HandleFunc("GET /environments/{id}", handler.GetEnvironment)
	mux.HandleFunc("DELETE /environments/{id}", handler.DeleteEnvironment)

	server := &http.Server{
		Addr:         ":8081",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("starting server on port 8081")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}
