# cloud-provisioner

A small Go project that will eventually grow into a cloud provisioning
tool - something that can manage jobs, environments, and infrastructure
tasks. Right now, at the very start, it does something much simpler.

This is a short learning project (a few days), so automated tests have
intentionally **not** been added yet. Instead, this project is verified
with manual testing steps — see the "Manual Testing" section below.
Automated tests can be added later once the project is more complete.

## What exists so far

* A basic HTTP server with a `/health` endpoint (Task 1).
* Domain models and validation for environment creation requests, not
  yet wired up to an HTTP endpoint (Task 2).

## Requirements

- Go 1.22 or newer

## Getting started

Navigate to the project folder:

```bash
cd /Users/pragatinarote/Desktop/cloud-provisioner
```

Run the server:

```bash
go run ./cmd/server
```

You should see:

```
starting server on port 8081
```

## Project structure

```
cloud-provisioner/
├── cmd/
│   ├── server/
│   │   └── main.go        # starts the HTTP server
│   └── validate/
│       └── main.go        # temporary manual validation runner
├── internal/
│   ├── api/
│   │   └── handler.go     # /health handler logic
│   └── model/
│       └── environment.go # environment models and validation
├── go.mod                 # Go module definition
├── README.md
└── .gitignore
```

## Manual Testing

This project currently uses manual testing instead of automated tests.
Below are the exact steps to verify each part of the project by hand.

### Manual Task 1 test (health endpoint)

Terminal 1:

```bash
cd /Users/pragatinarote/Desktop/cloud-provisioner
go run ./cmd/server
```

Terminal 2:

```bash
curl -i http://localhost:8081/health
```

Expected result:

```
HTTP/1.1 200 OK
Content-Type: application/json
```

and a body similar to:

```json
{"status":"ok"}
```

To stop the server, go back to Terminal 1 and press `Control + C`.

### Manual Task 2 test (validation logic)

```bash
cd /Users/pragatinarote/Desktop/cloud-provisioner
go run ./cmd/validate
```

This runs several valid and invalid `CreateEnvironmentRequest` values
through the real validation logic in `internal/model/environment.go` and
prints the result of each one. Expected output for each case:

* **Valid request** → `VALID`
* **Missing name** → `INVALID: name is required`
* **Missing region** → `INVALID: region is required`
* **No services** → `INVALID: at least one service is required`
* **Unsupported service** (`kafka`) → `INVALID: unsupported service: kafka`
* **Duplicate service** (`database`, `database`) → `INVALID: duplicate service: database`
* **Blank service** (spaces only) → `INVALID: service name cannot be empty`

Automated tests can be added back later once the project has grown
past this early learning stage.
