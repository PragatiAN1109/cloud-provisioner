// Command store-demo manually exercises the in-memory environment store
// from the terminal. It exists because the store is not wired up to any
// HTTP endpoint yet — this lets us see its real behavior directly,
// scenario by scenario, without writing automated tests.
package main

import (
	"fmt"
	"time"

	"github.com/pragatinarote/cloud-provisioner/internal/model"
	environmentstore "github.com/pragatinarote/cloud-provisioner/internal/store"
)

// heading prints a labeled section divider so each scenario's output is
// easy to tell apart when scanning the terminal.
func heading(title string) {
	fmt.Println("================================")
	fmt.Println(title)
	fmt.Println("================================")
}

// printEnvironment prints the fields someone would actually want to see
// when checking an environment's current state.
func printEnvironment(env model.Environment) {
	fmt.Println("ID:", env.ID)
	fmt.Println("Name:", env.Name)
	fmt.Println("Region:", env.Region)
	fmt.Println("Services:", env.Services)
	fmt.Println("Status:", env.Status)
}

func main() {
	// Scenario 1: create the store.
	// We alias the store package to "environmentstore" so the local
	// variable can simply be called "store" without shadowing the
	// package name in a confusing way.
	heading("1. Creating the store")
	store := environmentstore.NewMemoryStore()
	fmt.Println("SUCCESS: store created")

	// Scenario 2: list an empty store.
	heading("2. Listing an empty store")
	all := store.List()
	fmt.Println("Environment count:", len(all))

	// Scenario 3: create the first environment.
	heading("3. Creating first environment")
	now := time.Now()
	env1 := model.Environment{
		ID:        "env-001",
		Name:      "payments-dev",
		Region:    "us-west-2",
		Services:  []string{"database", "queue"},
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.Create(env1); err != nil {
		fmt.Println("ERROR:", err)
	} else {
		fmt.Println("SUCCESS: created", env1.ID)
	}

	// Scenario 4: retrieve the environment we just created.
	heading("4. Retrieving env-001")
	fetched, err := store.Get("env-001")
	if err != nil {
		fmt.Println("ERROR:", err)
	} else {
		printEnvironment(fetched)
	}

	// Scenario 5: create a second environment, then list all of them.
	heading("5. Creating second environment and listing all")
	env2 := model.Environment{
		ID:        "env-002",
		Name:      "analytics-dev",
		Region:    "us-east-1",
		Services:  []string{"storage", "cache"},
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.Create(env2); err != nil {
		fmt.Println("ERROR:", err)
	} else {
		fmt.Println("SUCCESS: created", env2.ID)
	}

	all = store.List()
	fmt.Println("Environment count:", len(all))
	fmt.Println("(list order may vary — Go map iteration order is not guaranteed)")
	for _, env := range all {
		printEnvironment(env)
		fmt.Println("---")
	}

	// Scenario 6: attempt a duplicate create.
	heading("6. Attempting duplicate create (env-001)")
	if err := store.Create(env1); err != nil {
		fmt.Println("ERROR:", err)
	} else {
		fmt.Println("SUCCESS: created", env1.ID)
	}

	// Scenario 7: update an environment's status.
	heading("7. Updating env-001 status to READY")
	toUpdate, err := store.Get("env-001")
	if err != nil {
		fmt.Println("ERROR:", err)
	} else {
		toUpdate.Status = model.StatusReady
		toUpdate.UpdatedAt = time.Now()
		if err := store.Update(toUpdate); err != nil {
			fmt.Println("ERROR:", err)
		} else {
			updated, err := store.Get("env-001")
			if err != nil {
				fmt.Println("ERROR:", err)
			} else {
				fmt.Println("Updated status:", updated.Status)
			}
		}
	}

	// Scenario 8: retrieve a missing environment.
	heading("8. Retrieving missing environment (env-999)")
	if _, err := store.Get("env-999"); err != nil {
		fmt.Println("ERROR:", err)
	}

	// Scenario 9: update a missing environment.
	heading("9. Updating missing environment (env-999)")
	missing := model.Environment{ID: "env-999", Status: model.StatusReady}
	if err := store.Update(missing); err != nil {
		fmt.Println("ERROR:", err)
	}

	// Scenario 10: delete an environment.
	heading("10. Deleting env-002")
	if err := store.Delete("env-002"); err != nil {
		fmt.Println("ERROR:", err)
	} else {
		fmt.Println("SUCCESS: deleted env-002")
	}
	all = store.List()
	fmt.Println("Environment count:", len(all))

	// Scenario 11: delete a missing environment.
	heading("11. Deleting missing environment (env-999)")
	if err := store.Delete("env-999"); err != nil {
		fmt.Println("ERROR:", err)
	}

	// Scenario 12: prove that data returned by Get is safely copied.
	heading("12. Proving returned data is safely copied")
	first, err := store.Get("env-001")
	if err != nil {
		fmt.Println("ERROR:", err)
	} else {
		first.Services[0] = "MODIFIED"

		second, err := store.Get("env-001")
		if err != nil {
			fmt.Println("ERROR:", err)
		} else {
			fmt.Println("Stored services remain unchanged:", second.Services)
		}
	}
}
