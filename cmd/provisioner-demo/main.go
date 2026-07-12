// Command provisioner-demo manually exercises the MockProvisioner from
// the terminal: successful create/delete, a forced temporary failure,
// manual cancellation, a deadline timeout, and an empty-services error.
// Nothing here touches real infrastructure — everything is simulated.
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pragatinarote/cloud-provisioner/internal/model"
	"github.com/pragatinarote/cloud-provisioner/internal/provisioner"
)

func heading(title string) {
	fmt.Println("========================================")
	fmt.Println(title)
	fmt.Println("========================================")
}

func main() {
	env := model.Environment{
		ID:       "env-demo-001",
		Name:     "payments-dev",
		Region:   "us-west-2",
		Services: []string{"database", "queue"},
		Status:   model.StatusPending,
	}

	shortDelay := 500 * time.Millisecond

	// Scenario 1: successful Create. A failure rate of 0.0 makes this
	// deterministic — it will never simulate a failure.
	heading("Scenario 1: Successful Create")
	reliable := provisioner.NewMockProvisioner(shortDelay, 0.0)
	if err := reliable.Create(context.Background(), env); err != nil {
		fmt.Println("UNEXPECTED ERROR:", err)
	} else {
		fmt.Println("SUCCESS: environment provisioning simulation completed")
	}

	// Scenario 2: successful Delete, using the same reliable provisioner.
	heading("Scenario 2: Successful Delete")
	if err := reliable.Delete(context.Background(), env); err != nil {
		fmt.Println("UNEXPECTED ERROR:", err)
	} else {
		fmt.Println("SUCCESS: environment deletion simulation completed")
	}

	// Scenario 3: forced temporary failure. A failure rate of 1.0 makes
	// this deterministic — it will always simulate a failure, after the
	// services finish "provisioning".
	heading("Scenario 3: Forced Temporary Failure")
	unreliable := provisioner.NewMockProvisioner(shortDelay, 1.0)
	if err := unreliable.Create(context.Background(), env); err != nil {
		fmt.Println("EXPECTED TEMPORARY ERROR:", err)
		fmt.Println("Is temporary:", provisioner.IsTemporary(err))
		fmt.Println("(Task 8 will later decide whether to retry errors like this one.)")
	} else {
		fmt.Println("UNEXPECTED SUCCESS")
	}

	// Scenario 4: manual cancellation. We start Create in a goroutine so
	// the main function can keep running and call cancel() partway
	// through, before all services would normally finish. A buffered
	// result channel lets the goroutine send its result even if nobody
	// were listening yet, and lets main wait for that exact result.
	heading("Scenario 4: Manual Cancellation")
	cancelDemo := provisioner.NewMockProvisioner(1*time.Second, 0.0)
	cancelCtx, cancel := context.WithCancel(context.Background())
	resultChannel := make(chan error, 1)

	go func() {
		resultChannel <- cancelDemo.Create(cancelCtx, env)
	}()

	time.Sleep(300 * time.Millisecond) // let it start, but not finish
	cancel()
	cancelResult := <-resultChannel

	if errors.Is(cancelResult, context.Canceled) {
		fmt.Println("EXPECTED CANCELLATION:", cancelResult)
	} else {
		fmt.Println("UNEXPECTED RESULT:", cancelResult)
	}

	// Scenario 5: deadline timeout. Unlike scenario 4, nobody calls
	// cancel() manually here — the context cancels itself automatically
	// once the timeout elapses, which happens well before the
	// provisioner's 1-second-per-service delay would finish.
	heading("Scenario 5: Deadline Timeout")
	slowProvisioner := provisioner.NewMockProvisioner(1*time.Second, 0.0)
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer timeoutCancel()

	timeoutResult := slowProvisioner.Create(timeoutCtx, env)
	if errors.Is(timeoutResult, context.DeadlineExceeded) {
		fmt.Println("EXPECTED DEADLINE:", timeoutResult)
	} else {
		fmt.Println("UNEXPECTED RESULT:", timeoutResult)
	}

	// Scenario 6: empty services. The API's validation would normally
	// prevent this, but the provisioner still defends against it, since
	// it could be called incorrectly by future code.
	heading("Scenario 6: Empty Services")
	emptyEnv := model.Environment{
		ID:       "env-demo-002",
		Name:     "empty-env",
		Region:   "us-west-2",
		Services: []string{},
		Status:   model.StatusPending,
	}
	if err := reliable.Create(context.Background(), emptyEnv); err != nil {
		fmt.Println("EXPECTED ERROR:", err)
	} else {
		fmt.Println("UNEXPECTED SUCCESS")
	}
}
