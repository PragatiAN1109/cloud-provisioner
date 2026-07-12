// Package provisioner defines the abstraction for creating and deleting
// the real cloud infrastructure an Environment describes, plus a mock
// implementation that simulates that work for local development and
// learning — no real AWS, Terraform, or Kubernetes resources are ever
// touched by this package.
package provisioner

import (
	"context"

	"github.com/pragatinarote/cloud-provisioner/internal/model"
)

// Provisioner describes what any infrastructure provisioner must be
// able to do: create the services an Environment requests, and delete
// them again. It only lists behavior, not how that behavior happens, so
// a future worker can depend on "something that can provision" without
// caring whether that's a mock, Terraform, or a direct AWS/Kubernetes
// implementation underneath.
type Provisioner interface {
	Create(ctx context.Context, env model.Environment) error
	Delete(ctx context.Context, env model.Environment) error
}

// TemporaryError represents a provisioning failure that is expected to
// be transient — trying the same operation again later might succeed.
// This is different from a permanent error (like an invalid region or
// an unsupported service), which retrying would never fix on its own.
// A future task's retry logic can check for this behavior to decide
// whether a failure is worth retrying at all.
type TemporaryError struct {
	Message string
}

// Error satisfies Go's built-in error interface, which only requires an
// Error() string method — this is what makes TemporaryError usable
// anywhere a plain error is expected.
func (e TemporaryError) Error() string {
	return e.Message
}

// Temporary reports whether this error is safe to retry. It always
// returns true for TemporaryError, by definition.
func (e TemporaryError) Temporary() bool {
	return true
}

// IsTemporary reports whether err represents a temporary, retryable
// failure. It works with any error that implements a Temporary() bool
// method — not only TemporaryError specifically — using a type
// assertion against a small, unnamed interface describing just that one
// method.
func IsTemporary(err error) bool {
	temp, ok := err.(interface{ Temporary() bool })
	return ok && temp.Temporary()
}
