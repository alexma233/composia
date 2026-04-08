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

- `caddy`

## Quick Start

Install frontend dependencies:

```bash
bun install
```

Start the placeholder web app:

```bash
bun run dev
```

Run the backend in controller mode:

```bash
mkdir -p ./repo-controller && git -C ./repo-controller init
go run ./cmd/composia controller -config ./configs/config.controller.dev.yaml
```

Run the backend in agent mode:

```bash
go run ./cmd/composia agent -config ./configs/config.controller.dev.yaml
```

Run a second agent with a different node ID:

```bash
go run ./cmd/composia agent -config ./configs/config.agent.dev.yaml
```

Run the containerized stack with Docker Compose:

```bash
docker compose up -d
```

The compose stack starts these services:

- `controller` on `:7001`
- `web` on `:3000`
- `agent` connected to the local Docker socket

The included `configs/config.compose.yaml` is wired for container networking and uses:

- `COMPOSIA_CONTROLLER_ADDR=http://controller:7001`
- `COMPOSIA_CLI_TOKEN=dev-admin-token`

Generate protobuf and Connect stubs:

```bash
buf generate
```

The example controller config also includes a development CLI token:

```text
dev-admin-token
```

## Repository Layout

```text
cmd/composia/         # composia entrypoint
configs/              # local development config examples
gen/go/               # generated protobuf and Connect code
internal/             # backend packages
proto/                # protobuf definitions
web/                  # SvelteKit frontend
plan.md               # product and architecture notes
```

## Current Scope

This repository currently contains a minimal development scaffold only:

- Go module and binary entrypoint
- Bun workspace and SvelteKit app shell
- Strict controller and agent config loading
- SQLite schema initialization
- Minimal ConnectRPC heartbeat and system status APIs
- Strict `composia-meta.yaml` parsing and service discovery
- Service snapshot refresh into SQLite
- Example controller and agent config files
- Git ignore and editor config

The next backend steps are:

1. Add the durable task queue.
2. Expose read-only service and node APIs.
3. Implement the first `deploy` flow.
4. Add task logs and task detail views.

## Attributions

- [Dockman](https://github.com/RA341/dockman) - Docker management UI reference for Docker resource list/inspect page patterns (AGPL-3.0)

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0). See [LICENSE](LICENSE) for details.

If you require a commercial license for use cases not permitted under AGPL-3.0, please contact the author.
