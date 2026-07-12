// Command validate manually exercises the CreateEnvironmentRequest
// validation logic from the terminal. It exists only because there is no
// HTTP endpoint yet that accepts these requests — this is a temporary,
// learning-friendly way to see validation behavior directly.
package main

import (
	"fmt"

	"github.com/pragatinarote/cloud-provisioner/internal/model"
)

// testCase pairs a human-readable label with a request to validate.
type testCase struct {
	label   string
	request model.CreateEnvironmentRequest
}

func main() {
	cases := []testCase{
		{
			label: "Valid request",
			request: model.CreateEnvironmentRequest{
				Name:     "payments-dev",
				Region:   "us-west-2",
				Services: []string{"database", "queue"},
			},
		},
		{
			label: "Missing name",
			request: model.CreateEnvironmentRequest{
				Name:     "",
				Region:   "us-west-2",
				Services: []string{"database"},
			},
		},
		{
			label: "Missing region",
			request: model.CreateEnvironmentRequest{
				Name:     "payments-dev",
				Region:   "",
				Services: []string{"database"},
			},
		},
		{
			label: "No services",
			request: model.CreateEnvironmentRequest{
				Name:     "payments-dev",
				Region:   "us-west-2",
				Services: nil,
			},
		},
		{
			label: "Unsupported service",
			request: model.CreateEnvironmentRequest{
				Name:     "payments-dev",
				Region:   "us-west-2",
				Services: []string{"kafka"},
			},
		},
		{
			label: "Duplicate service",
			request: model.CreateEnvironmentRequest{
				Name:     "payments-dev",
				Region:   "us-west-2",
				Services: []string{"database", "database"},
			},
		},
		{
			label: "Blank service",
			request: model.CreateEnvironmentRequest{
				Name:     "payments-dev",
				Region:   "us-west-2",
				Services: []string{"   "},
			},
		},
	}

	for _, c := range cases {
		fmt.Println("--------------------------------")
		fmt.Println("Case:", c.label)

		if err := c.request.Validate(); err != nil {
			fmt.Println("Result: INVALID:", err)
		} else {
			fmt.Println("Result: VALID")
		}
	}
	fmt.Println("--------------------------------")
}
