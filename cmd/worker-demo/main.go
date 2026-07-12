// Command worker-demo manually exercises the job queue and worker pool
// from the terminal: starting workers, successful and failed CREATE
// jobs, concurrent processing, a missing environment, an invalid job,
// an empty ID, a DELETE job, cancellation, and a full queue. Nothing
// here touches real infrastructure — everything is simulated.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/pragatinarote/cloud-provisioner/internal/model"
	"github.com/pragatinarote/cloud-provisioner/internal/provisioner"
	"github.com/pragatinarote/cloud-provisioner/internal/store"
	"github.com/pragatinarote/cloud-provisioner/internal/worker"
)

func heading(title string) {
	fmt.Println("========================================")
	fmt.Println(title)
	fmt.Println("========================================")
}

// waitForStatus polls the store for environmentID until its status
// matches one of terminalStatuses or the timeout elapses. Polling
// (rather than a single long time.Sleep) is used because processing
// happens asynchronously in a worker goroutine, and we don't know in
// advance exactly how long it will take. The caller specifies which
// statuses actually count as "done" for its scenario — a CREATE flow
// finishes at READY or FAILED, while a DELETE flow (which may start
// from READY) finishes at DELETED or FAILED instead.
func waitForStatus(environmentStore store.Store, environmentID string, timeout time.Duration, terminalStatuses ...model.EnvironmentStatus) (model.Environment, error) {
	deadline := time.Now().Add(timeout)

	isTerminal := func(status model.EnvironmentStatus) bool {
		for _, s := range terminalStatuses {
			if status == s {
				return true
			}
		}
		return false
	}

	for {
		environment, err := environmentStore.Get(environmentID)
		if err != nil {
			return model.Environment{}, err
		}

		if isTerminal(environment.Status) {
			return environment, nil
		}

		if time.Now().After(deadline) {
			return environment, fmt.Errorf("timed out waiting for a terminal status, last seen: %s", environment.Status)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func main() {
	sharedStore := store.NewMemoryStore()
	serviceDelay := 300 * time.Millisecond

	// Scenario 1: start three workers on a reliable (never-failing)
	// provisioner. This is the main pool used by most scenarios below.
	heading("Scenario 1: Starting Three Workers")
	reliableProvisioner := provisioner.NewMockProvisioner(serviceDelay, 0.0)
	mainPool := worker.NewWorkerPool(sharedStore, reliableProvisioner, 3, 10)
	mainCtx, mainCancel := context.WithCancel(context.Background())
	mainPool.Start(mainCtx)
	fmt.Println("Started 3 workers (see worker-started log lines above/below — order is not guaranteed, since goroutine scheduling isn't deterministic).")
	time.Sleep(100 * time.Millisecond)

	// Scenario 2: successful CREATE job.
	heading("Scenario 2: Successful CREATE Job")
	env1 := model.Environment{
		ID:       "env-worker-001",
		Name:     "payments-dev",
		Region:   "us-west-2",
		Services: []string{"database", "queue"},
		Status:   model.StatusPending,
	}
	if err := sharedStore.Create(env1); err != nil {
		fmt.Println("UNEXPECTED ERROR creating environment:", err)
	}
	if err := mainPool.Submit(worker.Job{Type: worker.JobTypeCreate, EnvironmentID: env1.ID}); err != nil {
		fmt.Println("UNEXPECTED SUBMIT ERROR:", err)
	}
	if result, err := waitForStatus(sharedStore, env1.ID, 5*time.Second, model.StatusReady, model.StatusFailed); err != nil {
		fmt.Println("UNEXPECTED ERROR:", err)
	} else {
		fmt.Println("Final status:", result.Status)
	}

	// Scenario 3: forced failed CREATE job. This uses its own pool with
	// an always-failing provisioner (failure rate 1.0), sharing the
	// same store, since the main pool's provisioner is configured to
	// never fail and provisioner behavior can't be swapped per-call.
	heading("Scenario 3: Failed CREATE Job")
	env2 := model.Environment{
		ID:       "env-worker-002",
		Name:     "analytics-dev",
		Region:   "us-east-1",
		Services: []string{"storage"},
		Status:   model.StatusPending,
	}
	if err := sharedStore.Create(env2); err != nil {
		fmt.Println("UNEXPECTED ERROR creating environment:", err)
	}

	unreliableProvisioner := provisioner.NewMockProvisioner(serviceDelay, 1.0)
	unreliablePool := worker.NewWorkerPool(sharedStore, unreliableProvisioner, 3, 10)
	unreliableCtx, unreliableCancel := context.WithCancel(context.Background())
	unreliablePool.Start(unreliableCtx)

	if err := unreliablePool.Submit(worker.Job{Type: worker.JobTypeCreate, EnvironmentID: env2.ID}); err != nil {
		fmt.Println("UNEXPECTED SUBMIT ERROR:", err)
	}
	if result, err := waitForStatus(sharedStore, env2.ID, 5*time.Second, model.StatusReady, model.StatusFailed); err != nil {
		fmt.Println("UNEXPECTED ERROR:", err)
	} else {
		fmt.Println("Final status:", result.Status)
		fmt.Println("Error message:", result.ErrorMessage)
	}

	unreliableCancel()
	unreliablePool.Wait()

	// Scenario 4: multiple jobs processed concurrently by three workers.
	heading("Scenario 4: Concurrent Jobs Across Three Workers")
	concurrentIDs := []string{"env-worker-101", "env-worker-102", "env-worker-103"}
	for i, id := range concurrentIDs {
		env := model.Environment{
			ID:       id,
			Name:     fmt.Sprintf("service-%d-dev", i+1),
			Region:   "us-west-2",
			Services: []string{"database"},
			Status:   model.StatusPending,
		}
		if err := sharedStore.Create(env); err != nil {
			fmt.Println("UNEXPECTED ERROR creating environment:", err)
			continue
		}
		if err := mainPool.Submit(worker.Job{Type: worker.JobTypeCreate, EnvironmentID: id}); err != nil {
			fmt.Println("UNEXPECTED SUBMIT ERROR:", err)
		}
	}
	for _, id := range concurrentIDs {
		result, err := waitForStatus(sharedStore, id, 5*time.Second, model.StatusReady, model.StatusFailed)
		if err != nil {
			fmt.Println("UNEXPECTED ERROR:", err)
			continue
		}
		fmt.Printf("Final status for %s: %s\n", id, result.Status)
	}

	// Scenario 5: submitting a job for an environment that doesn't
	// exist. The worker should log an error, stay alive, and remain
	// able to process later jobs correctly (proven right after).
	heading("Scenario 5: Missing Environment")
	if err := mainPool.Submit(worker.Job{Type: worker.JobTypeCreate, EnvironmentID: "env-does-not-exist"}); err != nil {
		fmt.Println("UNEXPECTED SUBMIT ERROR:", err)
	}
	time.Sleep(500 * time.Millisecond) // let a worker pick it up and log the not-found error

	env3 := model.Environment{
		ID:       "env-worker-003",
		Name:     "after-missing-dev",
		Region:   "us-west-2",
		Services: []string{"database"},
		Status:   model.StatusPending,
	}
	if err := sharedStore.Create(env3); err != nil {
		fmt.Println("UNEXPECTED ERROR creating environment:", err)
	}
	if err := mainPool.Submit(worker.Job{Type: worker.JobTypeCreate, EnvironmentID: env3.ID}); err != nil {
		fmt.Println("UNEXPECTED SUBMIT ERROR:", err)
	}
	if result, err := waitForStatus(sharedStore, env3.ID, 5*time.Second, model.StatusReady, model.StatusFailed); err != nil {
		fmt.Println("UNEXPECTED ERROR:", err)
	} else {
		fmt.Println("Worker still alive — subsequent job final status:", result.Status)
	}

	// Scenario 6: unsupported job type. Submit rejects this before it
	// ever reaches the queue.
	heading("Scenario 6: Unsupported Job Type")
	if err := mainPool.Submit(worker.Job{Type: worker.JobType("UPDATE"), EnvironmentID: env1.ID}); err != nil {
		fmt.Println("ERROR:", err)
	} else {
		fmt.Println("UNEXPECTED SUCCESS")
	}

	// Scenario 7: empty environment ID.
	heading("Scenario 7: Empty Environment ID")
	if err := mainPool.Submit(worker.Job{Type: worker.JobTypeCreate, EnvironmentID: ""}); err != nil {
		fmt.Println("ERROR:", err)
	} else {
		fmt.Println("UNEXPECTED SUCCESS")
	}

	// Scenario 8: DELETE job. The environment stays in the store with
	// status DELETED afterward — Task 6's worker never removes records,
	// unlike Task 4's existing immediate-delete HTTP endpoint (that only
	// changes once Task 7 connects the API to this worker pool).
	heading("Scenario 8: DELETE Job")
	deleteEnv := model.Environment{
		ID:       "env-worker-delete",
		Name:     "temporary-dev",
		Region:   "us-west-2",
		Services: []string{"database"},
		Status:   model.StatusReady,
	}
	if err := sharedStore.Create(deleteEnv); err != nil {
		fmt.Println("UNEXPECTED ERROR creating environment:", err)
	}
	if err := mainPool.Submit(worker.Job{Type: worker.JobTypeDelete, EnvironmentID: deleteEnv.ID}); err != nil {
		fmt.Println("UNEXPECTED SUBMIT ERROR:", err)
	}
	if result, err := waitForStatus(sharedStore, deleteEnv.ID, 5*time.Second, model.StatusDeleted, model.StatusFailed); err != nil {
		fmt.Println("UNEXPECTED ERROR:", err)
	} else {
		fmt.Println("Final status:", result.Status)
	}
	if stillThere, err := sharedStore.Get(deleteEnv.ID); err != nil {
		fmt.Println("UNEXPECTED ERROR retrieving after delete:", err)
	} else {
		fmt.Println("Record still present in store with status:", stillThere.Status)
	}

	// Scenario 9: cancellation during active work. This is the last
	// scenario using mainPool — we cancel its context entirely here.
	heading("Scenario 9: Cancellation During Active Work")
	cancelEnv := model.Environment{
		ID:       "env-worker-cancel",
		Name:     "cancel-demo-dev",
		Region:   "us-west-2",
		Services: []string{"database", "queue", "cache"},
		Status:   model.StatusPending,
	}
	if err := sharedStore.Create(cancelEnv); err != nil {
		fmt.Println("UNEXPECTED ERROR creating environment:", err)
	}
	if err := mainPool.Submit(worker.Job{Type: worker.JobTypeCreate, EnvironmentID: cancelEnv.ID}); err != nil {
		fmt.Println("UNEXPECTED SUBMIT ERROR:", err)
	}

	time.Sleep(150 * time.Millisecond) // let provisioning begin, but not finish
	mainCancel()
	mainPool.Wait()
	fmt.Println("All main pool workers have exited.")

	if result, err := sharedStore.Get(cancelEnv.ID); err != nil {
		fmt.Println("UNEXPECTED ERROR:", err)
	} else {
		fmt.Println("Final status:", result.Status)
		fmt.Println("Error message:", result.ErrorMessage)
	}

	// Scenario 10: queue-full behavior. This pool is deliberately never
	// started, so nothing ever drains the queue — that makes filling it
	// completely deterministic.
	heading("Scenario 10: Queue Full")
	tinyStore := store.NewMemoryStore()
	tinyProvisioner := provisioner.NewMockProvisioner(serviceDelay, 0.0)
	tinyPool := worker.NewWorkerPool(tinyStore, tinyProvisioner, 3, 1)

	if err := tinyPool.Submit(worker.Job{Type: worker.JobTypeCreate, EnvironmentID: "env-tiny-001"}); err != nil {
		fmt.Println("UNEXPECTED ERROR filling queue:", err)
	}
	if err := tinyPool.Submit(worker.Job{Type: worker.JobTypeCreate, EnvironmentID: "env-tiny-002"}); err != nil {
		fmt.Println("ERROR:", err)
	} else {
		fmt.Println("UNEXPECTED SUCCESS")
	}
	tinyPool.Stop()
	fmt.Println("Demonstration pool stopped (never started, so nothing to wait for).")
}
