package provisioner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"github.com/pragatinarote/cloud-provisioner/internal/model"
)

// MockProvisioner simulates infrastructure provisioning: it waits
// briefly per service, logs its progress, and can be configured to
// sometimes fail — all without creating any real cloud resources.
type MockProvisioner struct {
	serviceDelay time.Duration
	failureRate  float64
}

// NewMockProvisioner builds a MockProvisioner. serviceDelay is how long
// it pretends to take per service; failureRate must be between 0.0
// (never fail) and 1.0 (always fail) — values outside that range are
// clamped to the nearest valid bound, so misconfiguration can't produce
// nonsensical behavior.
func NewMockProvisioner(serviceDelay time.Duration, failureRate float64) *MockProvisioner {
	if failureRate < 0 {
		failureRate = 0
	}
	if failureRate > 1 {
		failureRate = 1
	}
	return &MockProvisioner{
		serviceDelay: serviceDelay,
		failureRate:  failureRate,
	}
}

// wait pauses for the configured per-service delay, but stops early and
// returns ctx.Err() if the context is cancelled or its deadline expires
// first. This is preferred over time.Sleep, which has no way to notice
// cancellation at all and would always wait the full duration regardless
// of what the caller wants.
func (p *MockProvisioner) wait(ctx context.Context) error {
	timer := time.NewTimer(p.serviceDelay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// shouldFail decides whether this call should simulate a temporary
// failure, based on the configured failure rate. rand.Float64 (from
// math/rand/v2) returns a value in [0.0, 1.0); comparing it against
// failureRate produces roughly the right proportion of failures across
// many calls. math/rand/v2's package-level functions are documented as
// safe to call from multiple goroutines at once, so no mutex is needed
// here even though this provisioner may later be called concurrently.
func (p *MockProvisioner) shouldFail() bool {
	if p.failureRate <= 0 {
		return false
	}
	if p.failureRate >= 1 {
		return true
	}
	return rand.Float64() < p.failureRate
}

// Create simulates provisioning every service an environment requests.
// It checks for cancellation before starting, walks through each
// service in order (logging and waiting for each one), and then
// sometimes reports a temporary failure according to the configured
// failure rate.
func (p *MockProvisioner) Create(ctx context.Context, env model.Environment) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(env.Services) == 0 {
		return errors.New("environment has no services to provision")
	}

	log.Printf("provisioning started: operation=create environment_id=%s environment_name=%s region=%s service_count=%d",
		env.ID, env.Name, env.Region, len(env.Services))

	for index, service := range env.Services {
		log.Printf("provisioning service: operation=create environment_id=%s service=%s service_index=%d service_count=%d",
			env.ID, service, index+1, len(env.Services))

		if err := p.wait(ctx); err != nil {
			log.Printf("provisioning cancelled: operation=create environment_id=%s error=%v", env.ID, err)
			return err
		}
	}

	if p.shouldFail() {
		err := TemporaryError{Message: fmt.Sprintf("temporary provisioning failure for environment %s", env.ID)}
		log.Printf("provisioning failed temporarily: operation=create environment_id=%s error=%v", env.ID, err)
		return err
	}

	log.Printf("provisioning completed: operation=create environment_id=%s", env.ID)
	return nil
}

// Delete simulates tearing down every service an environment has. It
// walks through the services in reverse order — real infrastructure
// teardown commonly undoes the last thing created first, so this is
// mainly a useful conceptual demonstration here, since these mock
// services don't actually depend on each other yet.
func (p *MockProvisioner) Delete(ctx context.Context, env model.Environment) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(env.Services) == 0 {
		return errors.New("environment has no services to delete")
	}

	log.Printf("deletion started: operation=delete environment_id=%s environment_name=%s region=%s service_count=%d",
		env.ID, env.Name, env.Region, len(env.Services))

	for i := len(env.Services) - 1; i >= 0; i-- {
		service := env.Services[i]
		log.Printf("deleting service: operation=delete environment_id=%s service=%s service_index=%d service_count=%d",
			env.ID, service, i+1, len(env.Services))

		if err := p.wait(ctx); err != nil {
			log.Printf("deletion cancelled: operation=delete environment_id=%s error=%v", env.ID, err)
			return err
		}
	}

	if p.shouldFail() {
		err := TemporaryError{Message: fmt.Sprintf("temporary provisioning failure for environment %s", env.ID)}
		log.Printf("deletion failed temporarily: operation=delete environment_id=%s error=%v", env.ID, err)
		return err
	}

	log.Printf("deletion completed: operation=delete environment_id=%s", env.ID)
	return nil
}
