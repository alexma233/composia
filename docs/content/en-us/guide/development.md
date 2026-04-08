# Development

This guide covers how to set up a local development environment for Composia.

## Prerequisites

- Go 1.25+
- Bun 1.3+
- Docker Engine + Docker Compose v2
- SQLite3
- Git

## Clone the Repository

```bash
git clone https://forgejo.alexma.top/alexma233/composia.git
cd composia
```

## Install Dependencies

### Frontend

```bash
bun install
```

### Backend

Go dependencies are managed automatically with `go mod`.

## Start Development Servers

### Frontend Development Server

```bash
bun run dev
```

The web interface will be available at `http://localhost:5173`.

### Backend - Controller

First, initialize the controller repository:

```bash
mkdir -p ./repo-controller && git -C ./repo-controller init
```

Then start the controller:

```bash
go run ./cmd/composia controller -config ./configs/config.controller.dev.yaml
```

### Backend - Agent

In another terminal, start the agent:

```bash
go run ./cmd/composia agent -config ./configs/config.agent.dev.yaml
```

## Run a Second Agent

To test multi-node scenarios, run a second agent with a different node ID:

```bash
go run ./cmd/composia agent -config ./configs/config.agent.dev.yaml
```

## Generate Protobuf Stubs

After modifying `.proto` files, regenerate the Go code:

```bash
buf generate
```

## Project Structure

```text
cmd/composia/         # composia entrypoint
configs/              # local development config examples
gen/go/               # generated protobuf and Connect code
internal/             # backend packages
proto/                # protobuf definitions
web/                  # SvelteKit frontend
```

## Contributing

Please ensure your code follows the existing patterns and includes appropriate tests.
