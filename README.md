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
  create, list, retrieve, and delete environments over HTTP. No
  automated tests are included; no real cloud infrastructure is
  created — everything lives in the server's own memory.

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
│   │   └── main.go               # starts the HTTP server, wires store + handler
│   ├── validate/
│   │   └── main.go               # temporary manual validation runner
│   └── store-demo/
│       └── main.go               # temporary manual store demo
├── internal/
│   ├── api/
│   │   ├── handler.go            # /health handler
│   │   ├── environment_handler.go # environment REST endpoints
│   │   └── response.go           # writeJSON / writeError helpers
│   ├── model/
│   │   └── environment.go        # environment models and validation
│   └── store/
│       └── memory_store.go       # thread-safe in-memory environment store
├── go.mod                        # Go module definition
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
saved, but no actual provisioning work happens yet (no worker exists so
far).

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
through the real validation logic and prints the result of each one.

### Manual Task 3 test (in-memory store)

```bash
cd /Users/pragatinarote/Desktop/cloud-provisioner
go run ./cmd/store-demo
```

This runs the real `MemoryStore` implementation through 12 scenarios,
including proof that returned data is safely copied.

### Manual Task 4 test (REST API)

Terminal 1:

```bash
cd /Users/pragatinarote/Desktop/cloud-provisioner
go run ./cmd/server
```

Terminal 2 — run these in order:

```bash
# 1. Health check
curl -i http://localhost:8081/health

# 2. List before creating anything -> expect []
curl -i http://localhost:8081/environments

# 3. Create a valid environment -> expect 202 Accepted
curl -i -X POST http://localhost:8081/environments \
  -H "Content-Type: application/json" \
  -d '{"name": "payments-dev", "region": "us-west-2", "services": ["database", "queue"]}'
```

Copy the `"id"` value from the response (e.g. `env-a3f91c20`) — you'll
need it for the next commands. To make this easier, save it in a shell
variable:

```bash
ENV_ID=env-REPLACE_WITH_YOUR_GENERATED_ID
```

```bash
# 4. List again -> should now include the environment above
curl -i http://localhost:8081/environments

# 5. Get it directly by ID
curl -i http://localhost:8081/environments/$ENV_ID

# 6. Get a missing ID -> expect 404
curl -i http://localhost:8081/environments/env-does-not-exist
```

```bash
# 7-11. Validation failures -> each expects 400 Bad Request
curl -i -X POST http://localhost:8081/environments -H "Content-Type: application/json" \
  -d '{"name": "", "region": "us-west-2", "services": ["database"]}'          # name is required

curl -i -X POST http://localhost:8081/environments -H "Content-Type: application/json" \
  -d '{"name": "payments-dev", "region": "", "services": ["database"]}'      # region is required

curl -i -X POST http://localhost:8081/environments -H "Content-Type: application/json" \
  -d '{"name": "payments-dev", "region": "us-west-2", "services": []}'       # at least one service is required

curl -i -X POST http://localhost:8081/environments -H "Content-Type: application/json" \
  -d '{"name": "payments-dev", "region": "us-west-2", "services": ["database", "kafka"]}'    # unsupported service: kafka

curl -i -X POST http://localhost:8081/environments -H "Content-Type: application/json" \
  -d '{"name": "payments-dev", "region": "us-west-2", "services": ["database", "database"]}' # duplicate service: database

# 12-13. Malformed / empty JSON -> each expects 400 Bad Request
curl -i -X POST http://localhost:8081/environments -H "Content-Type: application/json" \
  -d '{"name": "payments-dev",}'

curl -i -X POST http://localhost:8081/environments -H "Content-Type: application/json"
```

```bash
# 14. Delete the environment created earlier -> expect 204 No Content
curl -i -X DELETE http://localhost:8081/environments/$ENV_ID

# 15. Get it again -> expect 404
curl -i http://localhost:8081/environments/$ENV_ID

# 16. Delete a missing ID -> expect 404
curl -i -X DELETE http://localhost:8081/environments/env-does-not-exist

# 17. List after deletion -> expect []
curl -i http://localhost:8081/environments
```

**Manual Test 18 — confirm in-memory behavior**: create one more
environment, confirm it shows up in `GET /environments`, then stop the
server with `Control + C` in Terminal 1, and start it again with
`go run ./cmd/server`. Run `GET /environments` once more — it will
return `[]` again, because all data lived only in that server
process's memory and disappeared the moment the process stopped.

No automated tests are included because this is currently a short
learning project — testing here is entirely manual, using the commands
above, and can be revisited later once the project has grown further.
