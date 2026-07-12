# cloud-provisioner

A small Go project that will eventually grow into a cloud provisioning
tool - something that can manage jobs, environments, and infrastructure
tasks. Right now, at the very start, it does something much simpler.

## What Task 1 does

This first version is just a foundation. It starts a basic web server
that responds to one endpoint:

```
GET /health
```

which returns:

```json
{"status":"ok"}
```

This lets us confirm the server can start, run, and respond correctly
before we build anything more complex on top of it.

## Requirements

- Go 1.22 or newer

## Getting started

Navigate to the project folder:

```bash
cd /cloud-provisioner
```

Run the server:

```bash
go run ./cmd/server
```

You should see:

```
starting server on port 8081
```

## Testing the health endpoint

In a second terminal window, run:

```bash
curl -i http://localhost:8081/health
```

Expected output:

```
HTTP/1.1 200 OK
Content-Type: application/json

{"status":"ok"}
```

## Stopping the server

Go back to the terminal window running the server and press:

```
Control + C
```

## Project structure

```
cloud-provisioner/
├── cmd/
│   └── server/
│       └── main.go       # starts the HTTP server
├── internal/
│   └── api/
│       └── handler.go    # /health handler logic
├── go.mod                # Go module definition
├── README.md
└── .gitignore
```
