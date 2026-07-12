// Command server starts the cloud-provisioner HTTP server.
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/pragatinarote/cloud-provisioner/internal/api"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", api.HealthHandler)

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
