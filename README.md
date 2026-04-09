# Composia

<div align="center">
  <p><strong>Main Repository</strong></p>
  <p>
    <a href="https://forgejo.alexma.top/alexma233/composia">
      <img src="https://img.shields.io/gitea/stars/alexma233/composia?gitea_url=https://forgejo.alexma.top&style=for-the-badge&label=AlexMa's%20Forgejo%20Stars" alt="Forgejo Stars" />
    </a>
  </p>

  <p>Mirrors</p>
  <p>
    <a href="https://codeberg.org/alexma233/composia">
      <img src="https://img.shields.io/gitea/stars/alexma233/composia?gitea_url=https://codeberg.org&style=flat-square&label=Codeberg%20Stars" alt="Codeberg Stars" />
    </a>
    <a href="https://github.com/alexma233/composia">
      <img src="https://img.shields.io/github/stars/alexma233/composia?style=flat-square&label=GitHub%20Stars" alt="GitHub Stars" />
    </a>
    <a href="https://tangled.org/fur.im/composia">
      <img src="https://img.shields.io/badge/Tangled-View%20Repo-blue?style=flat-square" alt="Tangled" />
    </a>
  </p>

  <p>
    <a href="https://docs.composia.io">
      <strong>📚 Documentation</strong>
    </a>
  </p>
</div>

Composia is a self-hosted service manager built around service definitions, a single control plane, and one or more execution agents.

## Stack

- Backend: Go
- Frontend: SvelteKit with Bun
- Runtime: Docker Compose
- State database: SQLite
- RPC: ConnectRPC

## Prerequisites

- Docker Engine + Docker Compose v2

## Quick Start

Use the bundled `configs/config.compose.yaml` example.

The repository already includes that file together with the age key files it references. Edit it only if you need different tokens, paths, or optional features.

Run the container stack defined in the repository root `docker-compose.yaml`:

```bash
docker compose up -d
```

By default, `docker-compose.yaml` pulls images from the self-hosted Forgejo registry.
If you prefer GHCR, replace the image references in `docker-compose.yaml` with:

```yaml
ghcr.io/alexma233/composia:latest
ghcr.io/alexma233/composia-web:latest
```

The compose stack starts these long-running services:

- `controller` on `:7001`
- `web` on `:3000`
- `agent` connected to the local Docker socket

It also runs a one-shot `init-repo-controller` container first to initialize the controller Git working tree volume.

Access the web UI at `http://localhost:3000`.

Pre-built images are published to:

- Default registry: `forgejo.alexma.top/alexma233/composia`
- Default registry: `forgejo.alexma.top/alexma233/composia-web`
- Alternative registry: `ghcr.io/alexma233/composia`
- Alternative registry: `ghcr.io/alexma233/composia-web`

To stop the container stack started from the repository root `docker-compose.yaml`:

```bash
docker compose down
```

Note: the example stack injects the Web UI token through `COMPOSIA_CLI_TOKEN=dev-admin-token` in `docker-compose.yaml`. For production, generate your own token, update `configs/config.compose.yaml`, and change the Web service environment variable to match.

The release workflows publish to both Forgejo Registry and GHCR. Configure these repository secrets for automated pushes:

- `REGISTRY_USERNAME`
- `REGISTRY_PASSWORD`
- `GHCR_USERNAME`
- `GHCR_TOKEN`

## Development

Prerequisites for local development:

- Go 1.25+
- Bun 1.3+
- SQLite3
- Git

Install frontend dependencies:

```bash
bun install
```

Start the web app:

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

Generate protobuf and Connect stubs after changing files under `proto/`:

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

This repository now contains a working controller, agent runtime, and Web UI for the first full control-plane slice:

- Go controller and agent entrypoints
- Bun workspace and SvelteKit Web UI
- Strict controller and agent config loading
- SQLite initialization and persistent controller state
- ConnectRPC controller-agent link for heartbeat, long-poll task pull, bundle download, task state, step state, log upload, backup reporting, and Docker stats reporting
- Multi-node `composia-meta.yaml` parsing, repo validation, and service discovery
- Git-backed desired-state repo read/write APIs with sync state tracking
- Query/command split controller APIs for services, repo, nodes, and Docker inspection
- Task execution for deploy, update, stop, restart, backup, DNS update, Caddy sync/reload, Docker prune, and service migration orchestration
- Web UI pages for dashboard, services, nodes, tasks, backups, settings, and node-scoped Docker resource browsing
- Example controller and agent config files

## Attributions

- [Dockman](https://github.com/RA341/dockman) - Docker management UI reference for Docker resource list/inspect page patterns (AGPL-3.0)

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0). See [LICENSE](LICENSE) for details.

If you require a commercial license for use cases not permitted under AGPL-3.0, please contact the author.
