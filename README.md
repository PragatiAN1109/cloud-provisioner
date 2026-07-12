# cloud-provisioner

A small Go project that will eventually grow into a cloud provisioning
tool - something that can manage jobs, environments, and infrastructure
tasks. Right now, at the very start, it does something much simpler.

This is a short learning project (a few days), so automated tests have
intentionally **not** been added yet. Instead, this project is verified
with manual testing steps — see the "Manual Testing" section below.
Automated tests can be added later once the project is more complete.

## Current progress

* **Task 1** — a basic HTTP server with a `/health` endpoint.
* **Task 2** — domain models (`Environment`, `CreateEnvironmentRequest`)
  and request validation.
* **Task 3** — a thread-safe, in-memory store that can create, read,
  update, and delete `Environment` records.
* **Task 4** — a REST API, connected to the store, that lets a client
  create, list, retrieve, and delete environments over HTTP.
* **Task 5** — a `Provisioner` interface and a `MockProvisioner` that
  simulates infrastructure provisioning (waiting, logging, occasional
  temporary failures, context cancellation). Not yet connected to the
  REST API or any worker — see below.

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
│   │   └── main.go                # starts the HTTP server, wires store + handler
│   ├── validate/
│   │   └── main.go                # temporary manual validation runner
│   ├── store-demo/
│   │   └── main.go                # temporary manual store demo
│   └── provisioner-demo/
│       └── main.go                # temporary manual provisioner demo
├── internal/
│   ├── api/
│   │   ├── handler.go             # /health handler
│   │   ├── environment_handler.go # environment REST endpoints
│   │   └── response.go            # writeJSON / writeError helpers
│   ├── model/
│   │   └── environment.go         # environment models and validation
│   ├── store/
│   │   └── memory_store.go        # thread-safe in-memory environment store
│   └── provisioner/
│       ├── provisioner.go         # Provisioner interface + TemporaryError
│       └── mock_provisioner.go    # MockProvisioner simulation
├── go.mod                         # Go module definition
├── README.md
└── .gitignore
```

## Available endpoints

### `GET /health`

Health check. No body required.

```bash
curl -i http://localhost:8081/health
```

Success: `200 OK`, `{"status":"ok"}`.

### `POST /environments`

Creates a new environment. The server generates the ID and sets the
initial status to `PENDING` — the client cannot set either.

Request body:

```json
{"name": "payments-dev", "region": "us-west-2", "services": ["database", "queue"]}
```

```bash
curl -i -X POST http://localhost:8081/environments \
  -H "Content-Type: application/json" \
  -d '{"name": "payments-dev", "region": "us-west-2", "services": ["database", "queue"]}'
```

Success: `202 Accepted`, returning the created environment (including
its generated `id` and `PENDING` status). `202` rather than `201`
because this project models asynchronous provisioning — the record is
saved, but no actual provisioning work happens yet (see Task 5 below —
even the provisioner itself isn't wired into this endpoint yet).

Errors:
* `400 Bad Request` — invalid/malformed JSON, or a failed validation
  rule (`name is required`, `region is required`, `at least one
  service is required`, `unsupported service: <name>`, `duplicate
  service: <name>`, `service name cannot be empty`).
* `409 Conflict` — the generated ID already existed (extremely rare).
* `500 Internal Server Error` — an unexpected internal failure.

### `GET /environments`

Lists every stored environment.

```bash
curl -i http://localhost:8081/environments
```

Success: `200 OK`. Returns `[]` (not `null`) when nothing is stored yet.
List order is not guaranteed, since the store is backed by a Go map.

### `GET /environments/{id}`

Retrieves one environment by ID.

```bash
curl -i http://localhost:8081/environments/env-a3f91c20
```

Success: `200 OK`, the matching environment.
Error: `404 Not Found` — `{"error": "environment not found: env-a3f91c20"}`.

### `DELETE /environments/{id}`

Removes one environment immediately from the in-memory store. This is a
real, immediate delete for this learning stage — a production platform
would more likely mark the record `DELETING`, tear down real resources
asynchronously, and only then mark it `DELETED`.

```bash
curl -i -X DELETE http://localhost:8081/environments/env-a3f91c20
```

Success: `204 No Content` (no response body).
Error: `404 Not Found` if the ID doesn't exist.

## Task 3 concepts

* **In-memory storage** — data lives only in the program's own memory
  (RAM) while it runs, in a plain Go `map[string]model.Environment`.
* **Interfaces** — a `Store` interface describes *what* operations are
  supported (Create, Get, List, Update, Delete) without saying *how*
  they're implemented, so a future database-backed store could satisfy
  the same interface.
* **Constructors** — `NewMemoryStore()` builds a store with its map
  already initialized, so it's safe to use immediately.
* **Mutexes / read & write locks** — a `sync.RWMutex` protects the map
  so multiple goroutines (like concurrent HTTP requests) can read at
  the same time, but writes get exclusive access.
* **CRUD** — Create, Read (`Get`/`List`), Update, Delete: the standard
  four operations most backend storage exposes.
* **Temporary data** — because everything lives in memory, all stored
  environments disappear the moment the program stops. That's an
  accepted tradeoff for this learning stage; a real system would use a
  database for durability.

## Task 4 concepts

* **REST API** — HTTP endpoints where the URL identifies a *resource*
  (an environment) and the HTTP method identifies the *action*
  (create/read/delete).
* **Method-specific routing** — `mux.HandleFunc("POST /environments", ...)`
  uses Go 1.22+'s built-in routing patterns; no third-party router.
* **Dependency injection** — the HTTP handler is built with
  `api.NewHandler(environmentStore)`, receiving the store instance
  rather than constructing its own, so every request shares one store.
* **Server-generated IDs** — the client never supplies an ID; the
  server generates a random `env-xxxxxxxx` ID using `crypto/rand`.
* **Server-managed fields** — `status`, `created_at`, and `updated_at`
  are always set by the server, never by the client.
* **Consistent JSON error format** — every error response looks like
  `{"error": "message"}`.
* **Status code mapping** — validation failures become `400`, missing
  resources become `404`, ID collisions become `409`, unexpected
  failures become `500` (without leaking internal details).

## Task 5: Mock infrastructure provisioner

* **`Provisioner` interface** (`internal/provisioner/provisioner.go`) —
  describes what any real infrastructure provisioner must do:
  `Create(ctx, env) error` and `Delete(ctx, env) error`. Nothing about
  *how* provisioning happens is specified here — that's left to whatever
  concrete type implements it.
* **`MockProvisioner`** (`internal/provisioner/mock_provisioner.go`) — a
  fake implementation that simulates provisioning without touching any
  real infrastructure:
  * logs the start, each service being "provisioned" or "deleted", and
    the outcome;
  * waits briefly per service (configurable delay);
  * can be configured to sometimes return a `TemporaryError` (a
    retryable, transient-style failure) based on a configured failure
    rate between `0.0` (never fail) and `1.0` (always fail);
  * respects context cancellation and deadlines, stopping immediately
    instead of finishing all services;
  * deletes services in reverse order, mirroring how real
    infrastructure teardown commonly works.
* This does **not** create any real AWS, Terraform, Kubernetes,
  database, queue, cache, or storage resources — it is purely a
  simulation, used to build and test the shape of provisioning logic
  before wiring it into anything real.
* The provisioner is **not yet connected** to the REST API. `POST
  /environments` still only validates and stores an environment with
  `PENDING` status — no worker exists yet to call `Provisioner.Create`.

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

Expected: `HTTP/1.1 200 OK`, `Content-Type: application/json`, body
`{"status":"ok"}`. Stop the server with `Control + C`.

### Manual Task 2 test (validation logic)

```bash
cd /Users/pragatinarote/Desktop/cloud-provisioner
go run ./cmd/validate
```

Runs several valid and invalid `CreateEnvironmentRequest` values through
the real validation logic and prints the result of each one.

### Manual Task 3 test (in-memory store)

```bash
cd /Users/pragatinarote/Desktop/cloud-provisioner
go run ./cmd/store-demo
```

Runs the real `MemoryStore` through 12 scenarios, including proof that
returned data is safely copied.

### Manual Task 4 test (REST API)

Terminal 1:

```bash
cd /Users/pragatinarote/Desktop/cloud-provisioner
go run ./cmd/server
```

Terminal 2 — create, list, get, delete, and check all validation error
cases (`name`/`region`/`services` missing, unsupported/duplicate
service, malformed/empty JSON). See earlier project history or the
`internal/api/environment_handler.go` comments for the exact `curl`
commands and expected responses. Restarting the server (`Control + C`
then `go run ./cmd/server` again) resets all data back to `[]`, since
everything lives only in that process's memory.

### Manual Task 5 test (mock provisioner)

```bash
cd /Users/pragatinarote/Desktop/cloud-provisioner
go run ./cmd/provisioner-demo
```

Runs six scenarios against the real `MockProvisioner`:

1. **Successful Create** — failure rate `0.0`; logs each service, waits
   briefly, then prints `SUCCESS: environment provisioning simulation completed`.
2. **Successful Delete** — same provisioner; deletes services in
   reverse order, then prints `SUCCESS: environment deletion simulation completed`.
3. **Forced temporary failure** — failure rate `1.0`; after simulating
   both services, returns a `TemporaryError` and prints
   `EXPECTED TEMPORARY ERROR: ...` plus `Is temporary: true`.
4. **Manual cancellation** — starts `Create` in a goroutine, cancels the
   context partway through, and prints
   `EXPECTED CANCELLATION: context canceled`.
5. **Deadline timeout** — uses a context that times out before
   provisioning would finish, printing
   `EXPECTED DEADLINE: context deadline exceeded`.
6. **Empty services** — calling `Create` with no services prints
   `EXPECTED ERROR: environment has no services to provision`.

No automated tests are included — this is still a short learning
project, and testing here is entirely manual. Also note: **no worker
pool exists yet**, and **the provisioner is not connected to the REST
API** — `POST /environments` still only validates and stores an
environment with `PENDING` status; nothing currently calls
`Provisioner.Create` or `Provisioner.Delete` automatically.
