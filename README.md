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
  and request validation, not yet wired up to an HTTP endpoint.
* **Task 3** — a thread-safe, in-memory store that can create, read,
  update, and delete `Environment` records, not yet wired up to an
  HTTP endpoint either.

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
│   │   └── main.go          # starts the HTTP server
│   ├── validate/
│   │   └── main.go          # temporary manual validation runner
│   └── store-demo/
│       └── main.go          # temporary manual store demo
├── internal/
│   ├── api/
│   │   └── handler.go       # /health handler logic
│   ├── model/
│   │   └── environment.go   # environment models and validation
│   └── store/
│       └── memory_store.go  # thread-safe in-memory environment store
├── go.mod                   # Go module definition
├── README.md
└── .gitignore
```

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
  so multiple goroutines (like concurrent HTTP requests, later) can
  read at the same time, but writes get exclusive access.
* **CRUD** — Create, Read (`Get`/`List`), Update, Delete: the standard
  four operations most backend storage exposes.
* **Temporary data** — because everything lives in memory, all stored
  environments disappear the moment the program stops. That's an
  accepted tradeoff for this learning stage; a real system would use a
  database for durability.

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

### Manual Task 3 test (in-memory store)

```bash
cd /Users/pragatinarote/Desktop/cloud-provisioner
go run ./cmd/store-demo
```

This runs the real `MemoryStore` implementation from
`internal/store/memory_store.go` through 12 scenarios and prints the
result of each one. Key expected results:

* An empty store reports `Environment count: 0`.
* The first environment (`env-001`, `payments-dev`) and second
  environment (`env-002`, `analytics-dev`) can both be created
  successfully.
* Creating `env-001` a second time is rejected with
  `ERROR: environment already exists: env-001`.
* `env-001` can be retrieved, showing its ID, name, region, services,
  and status.
* After creating both environments, `List()` reports
  `Environment count: 2` (note: the printed order between env-001 and
  env-002 may vary, since Go map iteration order is not guaranteed).
* `env-001`'s status can be updated from `PENDING` to `READY`.
* Retrieving or updating a missing ID (`env-999`) returns
  `ERROR: environment not found: env-999`.
* Deleting `env-002` succeeds, and the count drops to `1`; deleting
  `env-999` again returns a not-found error.
* Modifying the `Services` slice on a value returned by `Get` does
  **not** change the stored data — a second `Get` still shows
  `Stored services remain unchanged: [database queue]`.

Automated tests can be added back later once the project has grown
past this early learning stage.
