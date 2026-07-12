package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pragatinarote/cloud-provisioner/internal/model"
	"github.com/pragatinarote/cloud-provisioner/internal/provisioner"
	"github.com/pragatinarote/cloud-provisioner/internal/store"
)

// Sentinel errors returned by Submit. Callers can compare against these
// with errors.Is, the same pattern already used by the store package.
var (
	ErrEmptyEnvironmentID = errors.New("environment ID is required")
	ErrQueueFull          = errors.New("job queue is full")
	ErrPoolStopped        = errors.New("worker pool is stopped")
)

// WorkerPool owns a buffered queue of jobs and a fixed number of worker
// goroutines that process them using the store and provisioner. It
// depends on the store.Store and provisioner.Provisioner interfaces
// (not their concrete types), so it works unchanged whether the store
// is in-memory or database-backed, and whether the provisioner is the
// mock or a real Terraform/AWS/Kubernetes implementation later.
type WorkerPool struct {
	jobs        chan Job
	store       store.Store
	provisioner provisioner.Provisioner
	workerCount int

	wg      sync.WaitGroup
	mu      sync.Mutex
	stopped bool
}

// NewWorkerPool builds a WorkerPool backed by the given store and
// provisioner. workerCount and queueSize are clamped to at least 1, so
// a misconfigured value (like 0 or a negative number) can't produce a
// pool that could never process anything.
func NewWorkerPool(environmentStore store.Store, infrastructureProvisioner provisioner.Provisioner, workerCount int, queueSize int) *WorkerPool {
	if workerCount < 1 {
		workerCount = 1
	}
	if queueSize < 1 {
		queueSize = 1
	}
	return &WorkerPool{
		jobs:        make(chan Job, queueSize),
		store:       environmentStore,
		provisioner: infrastructureProvisioner,
		workerCount: workerCount,
	}
}

// Start launches workerCount worker goroutines, each running until ctx
// is cancelled. Start returns immediately without blocking; call Wait
// afterward (usually after cancelling ctx) to block until every worker
// has actually exited.
func (p *WorkerPool) Start(ctx context.Context) {
	for i := 1; i <= p.workerCount; i++ {
		p.wg.Add(1)
		go p.runWorker(ctx, i)
	}
}

// Wait blocks until every worker goroutine started by Start has exited.
func (p *WorkerPool) Wait() {
	p.wg.Wait()
}

// Stop marks the pool as no longer accepting new submissions via
// Submit. It does not stop already-running workers by itself — cancel
// the context passed to Start for that, then call Wait.
func (p *WorkerPool) Stop() {
	p.mu.Lock()
	p.stopped = true
	p.mu.Unlock()
}

// Submit validates and enqueues a job. It never blocks: if the queue is
// already full, it returns ErrQueueFull immediately instead of waiting
// for space to free up — for this learning project, a fast, clear
// rejection is simpler and easier to reason about than making a client
// wait indefinitely.
func (p *WorkerPool) Submit(job Job) error {
	if job.Type != JobTypeCreate && job.Type != JobTypeDelete {
		return fmt.Errorf("unsupported job type: %s", job.Type)
	}
	if job.EnvironmentID == "" {
		return ErrEmptyEnvironmentID
	}

	p.mu.Lock()
	stopped := p.stopped
	p.mu.Unlock()
	if stopped {
		return ErrPoolStopped
	}

	select {
	case p.jobs <- job:
		return nil
	default:
		return ErrQueueFull
	}
}

// runWorker is the loop each worker goroutine runs for its entire
// lifetime: wait for either a new job or a cancellation signal, handle
// whichever arrives first, and repeat until told to stop.
func (p *WorkerPool) runWorker(ctx context.Context, workerID int) {
	defer p.wg.Done()

	log.Printf("worker started: worker_id=%d", workerID)

	for {
		select {
		case job, ok := <-p.jobs:
			if !ok {
				// This pool never closes p.jobs in Task 6 (it relies on
				// context cancellation instead), so this branch is not
				// currently reachable — it's included so the worker
				// still shuts down cleanly rather than spinning, if a
				// future change ever does close the channel.
				log.Printf("worker stopped: worker_id=%d reason=queue_closed", workerID)
				return
			}
			p.processJob(ctx, workerID, job)
		case <-ctx.Done():
			log.Printf("worker stopped: worker_id=%d reason=context_cancelled", workerID)
			return
		}
	}
}

// processJob dispatches a job to the right handler based on its type.
// One job failing must never crash the worker or stop it from picking
// up the next job — every error path below logs and returns, letting
// runWorker's loop continue.
func (p *WorkerPool) processJob(ctx context.Context, workerID int, job Job) {
	log.Printf("job received: worker_id=%d job_type=%s environment_id=%s", workerID, job.Type, job.EnvironmentID)

	switch job.Type {
	case JobTypeCreate:
		p.processCreate(ctx, workerID, job.EnvironmentID)
	case JobTypeDelete:
		p.processDelete(ctx, workerID, job.EnvironmentID)
	default:
		// Submit already rejects unknown types before they ever reach
		// the queue, so this should be unreachable — but a worker must
		// never panic on a job it doesn't recognize, so it just logs.
		log.Printf("job skipped: worker_id=%d reason=unsupported_job_type job_type=%s", workerID, job.Type)
	}
}

// processCreate handles a CREATE job: PENDING (or a retried FAILED)
// moves to PROVISIONING, the provisioner is called, and the result
// becomes either READY or FAILED.
func (p *WorkerPool) processCreate(ctx context.Context, workerID int, environmentID string) {
	environment, err := p.store.Get(environmentID)
	if err != nil {
		log.Printf("environment not found: worker_id=%d environment_id=%s error=%v", workerID, environmentID, err)
		return
	}

	// FAILED is deliberately allowed to retry here: Task 6 has no
	// automatic retry logic yet, so manually resubmitting a CREATE job
	// is currently the only way to try again after a failure.
	if environment.Status != model.StatusPending && environment.Status != model.StatusFailed {
		log.Printf("job skipped: worker_id=%d environment_id=%s reason=already_in_progress_or_done status=%s",
			workerID, environmentID, environment.Status)
		return
	}

	oldStatus := environment.Status
	environment.Status = model.StatusProvisioning
	environment.ErrorMessage = ""
	environment.UpdatedAt = time.Now().UTC()

	if err := p.store.Update(environment); err != nil {
		log.Printf("store update failed: worker_id=%d environment_id=%s error=%v", workerID, environmentID, err)
		return
	}
	log.Printf("environment status changed: worker_id=%d environment_id=%s environment_name=%s old_status=%s new_status=%s",
		workerID, environmentID, environment.Name, oldStatus, environment.Status)

	// The store lock is already released by now — Update only holds it
	// long enough to write the map entry. The slow part (calling the
	// provisioner) happens with no store lock held at all, so it never
	// blocks other requests or other workers from reaching the store.
	log.Printf("provisioning started: worker_id=%d environment_id=%s", workerID, environmentID)
	provisionErr := p.provisioner.Create(ctx, environment)

	if provisionErr != nil {
		environment.Status = model.StatusFailed
		environment.ErrorMessage = provisionErr.Error()
		log.Printf("provisioning failed: worker_id=%d environment_id=%s error=%v", workerID, environmentID, provisionErr)
	} else {
		environment.Status = model.StatusReady
		environment.ErrorMessage = ""
		log.Printf("provisioning succeeded: worker_id=%d environment_id=%s", workerID, environmentID)
	}
	environment.UpdatedAt = time.Now().UTC()

	if err := p.store.Update(environment); err != nil {
		// A provisioning success (or failure) that can't be recorded is
		// a real consistency problem in a production system — the
		// store would disagree with reality. Task 6 doesn't attempt to
		// solve that here; it just logs clearly and moves on rather
		// than panicking or retrying blindly.
		log.Printf("store update failed: worker_id=%d environment_id=%s error=%v", workerID, environmentID, err)
		return
	}

	log.Printf("job completed: worker_id=%d job_type=CREATE environment_id=%s final_status=%s",
		workerID, environmentID, environment.Status)
}

// processDelete handles a DELETE job: any non-DELETED environment moves
// to DELETING, the provisioner is called, and the result becomes either
// DELETED or FAILED. The record is kept in the store either way — Task
// 6 does not remove it, so the DELETING → DELETED transition can still
// be observed manually afterward. This differs from Task 4's existing
// HTTP DELETE endpoint, which still removes the record immediately;
// that only changes once Task 7 connects the API to this worker pool.
func (p *WorkerPool) processDelete(ctx context.Context, workerID int, environmentID string) {
	environment, err := p.store.Get(environmentID)
	if err != nil {
		log.Printf("environment not found: worker_id=%d environment_id=%s error=%v", workerID, environmentID, err)
		return
	}

	if environment.Status == model.StatusDeleted {
		log.Printf("job skipped: worker_id=%d environment_id=%s reason=already_deleted", workerID, environmentID)
		return
	}

	oldStatus := environment.Status
	environment.Status = model.StatusDeleting
	environment.ErrorMessage = ""
	environment.UpdatedAt = time.Now().UTC()

	if err := p.store.Update(environment); err != nil {
		log.Printf("store update failed: worker_id=%d environment_id=%s error=%v", workerID, environmentID, err)
		return
	}
	log.Printf("environment status changed: worker_id=%d environment_id=%s environment_name=%s old_status=%s new_status=%s",
		workerID, environmentID, environment.Name, oldStatus, environment.Status)

	log.Printf("deletion started: worker_id=%d environment_id=%s", workerID, environmentID)
	deleteErr := p.provisioner.Delete(ctx, environment)

	if deleteErr != nil {
		environment.Status = model.StatusFailed
		environment.ErrorMessage = deleteErr.Error()
		log.Printf("deletion failed: worker_id=%d environment_id=%s error=%v", workerID, environmentID, deleteErr)
	} else {
		environment.Status = model.StatusDeleted
		environment.ErrorMessage = ""
		log.Printf("deletion succeeded: worker_id=%d environment_id=%s", workerID, environmentID)
	}
	environment.UpdatedAt = time.Now().UTC()

	if err := p.store.Update(environment); err != nil {
		log.Printf("store update failed: worker_id=%d environment_id=%s error=%v", workerID, environmentID, err)
		return
	}

	log.Printf("job completed: worker_id=%d job_type=DELETE environment_id=%s final_status=%s",
		workerID, environmentID, environment.Status)
}
