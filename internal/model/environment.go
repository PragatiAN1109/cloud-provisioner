// Package model contains the core domain types for the cloud-provisioner
// service: the data we store about an environment, and the request shape
// a client sends when asking us to create one.
package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// EnvironmentStatus represents the lifecycle state of an Environment.
// It is a distinct type (based on string) rather than a plain string so
// the compiler can help catch typos and invalid values at compile time.
type EnvironmentStatus string

// The possible values of EnvironmentStatus. These cover the lifecycle of
// an environment from creation through to deletion.
const (
	// StatusPending means the environment has been recorded but
	// provisioning has not started yet.
	StatusPending EnvironmentStatus = "PENDING"

	// StatusProvisioning means the platform is actively creating the
	// requested cloud services.
	StatusProvisioning EnvironmentStatus = "PROVISIONING"

	// StatusReady means all requested services were provisioned
	// successfully and the environment is usable.
	StatusReady EnvironmentStatus = "READY"

	// StatusFailed means provisioning encountered an error and could
	// not complete. ErrorMessage should describe what went wrong.
	StatusFailed EnvironmentStatus = "FAILED"

	// StatusDeleting means the environment is in the process of being
	// torn down.
	StatusDeleting EnvironmentStatus = "DELETING"

	// StatusDeleted means the environment and its services have been
	// fully removed.
	StatusDeleted EnvironmentStatus = "DELETED"
)

// supportedServices is the set of service names the platform currently
// knows how to provision. It is implemented as a map with empty struct
// values because we only ever care whether a key is present, not what
// value is stored against it — struct{} takes up no memory.
var supportedServices = map[string]struct{}{
	"database": {},
	"queue":    {},
	"cache":    {},
	"storage":  {},
}

// isSupportedService reports whether name is one of the services the
// platform currently knows how to provision.
func isSupportedService(name string) bool {
	_, ok := supportedServices[name]
	return ok
}

// Environment represents a collection of cloud services provisioned (or
// being provisioned) for a user, as it is stored and returned by the API.
type Environment struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Region       string            `json:"region"`
	Services     []string          `json:"services"`
	Status       EnvironmentStatus `json:"status"`
	ErrorMessage string            `json:"error_message,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// CreateEnvironmentRequest is the shape of the JSON body a client sends
// when asking the API to create a new environment. It intentionally
// contains far fewer fields than Environment: only the client-supplied
// inputs, never server-managed fields like ID, Status, or timestamps.
type CreateEnvironmentRequest struct {
	Name     string   `json:"name"`
	Region   string   `json:"region"`
	Services []string `json:"services"`
}

// Validate checks that a CreateEnvironmentRequest contains acceptable
// values. It only checks the request; it does not modify r or trim its
// fields in place. It returns nil if the request is valid, or an error
// describing the first problem found.
func (r CreateEnvironmentRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("name is required")
	}

	if strings.TrimSpace(r.Region) == "" {
		return errors.New("region is required")
	}

	if len(r.Services) == 0 {
		return errors.New("at least one service is required")
	}

	seen := make(map[string]struct{}, len(r.Services))

	for _, service := range r.Services {
		if strings.TrimSpace(service) == "" {
			return errors.New("service name cannot be empty")
		}

		if !isSupportedService(service) {
			return fmt.Errorf("unsupported service: %s", service)
		}

		if _, alreadySeen := seen[service]; alreadySeen {
			return fmt.Errorf("duplicate service: %s", service)
		}
		seen[service] = struct{}{}
	}

	return nil
}
