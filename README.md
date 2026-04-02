# composia

Composia is a self-hosted service manager built around service definitions, a single control plane, and one or more execution agents.

## Stack

- Backend: Go
- Frontend: SvelteKit with Bun
- Runtime: Docker Compose
- State database: SQLite
- Planned RPC: ConnectRPC

## Prerequisites

- Go 1.25+
- Bun 1.3+
- Docker Engine + Docker Compose v2
- SQLite3
- Git

These tools are planned but not yet wired into the scaffold:

- `buf`
- `protoc`
- `caddy`

## Quick Start

Install frontend dependencies:

```bash
bun install
```

Start the placeholder web app:

```bash
bun run dev:web
```

Run the backend in main mode:

```bash
go run ./cmd/composia -role main -config ./configs/config.main.dev.yaml
```

Run the backend in agent mode:

```bash
go run ./cmd/composia -role agent -config ./configs/config.agent.dev.yaml
```

## Repository Layout

```text
cmd/composia/         # composia entrypoint
configs/              # local development config examples
internal/             # backend packages
web/                  # SvelteKit frontend
plan.md               # product and architecture notes
```

## Current Scope

This repository currently contains a minimal development scaffold only:

- Go module and binary entrypoint
- Bun workspace and SvelteKit app shell
- Example main and agent config files
- Git ignore and editor config

The next backend steps are:

1. Define the config model and loader.
2. Add initial ConnectRPC protobufs.
3. Implement main-agent heartbeat.
4. Add SQLite state storage.
