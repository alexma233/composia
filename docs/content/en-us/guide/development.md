# Development Guide

This guide covers how to set up a local development environment for Composia.

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25+ | Backend development language |
| Bun | 1.3+ | Frontend package manager and runtime |
| Docker | 20.10+ | Container runtime |
| Docker Compose | v2.0+ | Container orchestration |
| SQLite3 | 3.35+ | Database |
| Git | 2.30+ | Version control |
| buf | 1.30+ | Protobuf code generation |

The repository now ships a `mise.toml` for managing `go`, `bun`, and `buf`.

If you use `mise`, install and activate it first, then run:

```bash
mise install
```

`docker`, `docker compose`, `git`, and `sqlite3` are still best installed from your system package manager.

## Environment Setup

### 1. Clone the Repository

```bash
git clone https://forgejo.alexma.top/alexma233/composia.git
cd composia
```

### 2. Initialize Development Configuration

```bash
# Create required directories
mkdir -p dev/repo-controller dev/repo-agent

# Initialize Git repository (required for Controller)
git init dev/repo-controller
```

## Start Development Environment

### Option 1: Fully Containerized Development with Hot Reload (Recommended)

```bash
mise run dev
```

This development stack starts:

- `controller-dev` on `http://localhost:7001`
- `web-dev` on `http://localhost:5173`
- `docs-dev` on `http://localhost:5174`
- `agent-dev` connected to the local Docker socket

You do not need to run `bun install` on the host for this path. `web-dev` and `docs-dev` install workspace dependencies inside the containers when they start.

It reuses the existing development state directories under `dev/` by default:

- `./dev/repo-controller`
- `./dev/state-controller`
- `./dev/repo-agent`
- `./dev/state-agent`
- `./dev/logs`

That means service definitions, the SQLite database, and task logs created by your earlier manually started Controller or Agent processes are carried directly into the containerized dev stack.

Hot reload behavior:

- `web-dev` runs `vite dev`
- `docs-dev` runs `vitepress dev`
- `controller-dev` and `agent-dev` use `air` to rebuild and restart on Go source changes

Source code is bind-mounted into the containers, so edits are reflected automatically.

Stop the development stack with:

```bash
mise run dev:down
```

Follow the development stack logs with:

```bash
mise run dev:logs
```

If your host runs SELinux, use the override-enabled entrypoint instead:

```bash
mise run dev:selinux
```

This additionally loads `dev/docker-compose.dev.selinux.override.yaml` and sets `label=disable` for the development containers so bind-mounted source directories remain accessible on SELinux hosts.

Stop that stack with:

```bash
mise run dev:down:selinux
```

Follow that stack's logs with:

```bash
mise run dev:logs:selinux
```

### Option 2: Local Toolchain Development

Install workspace dependencies on the host first:

```bash
bun install
```

**Start the frontend development server:**

```bash
mise run web
```

The frontend will be available at `http://localhost:5173`.

To start the docs dev server:

```bash
mise run docs
```

The docs site runs on `http://localhost:5174`.

**Start the Controller (Terminal 2):**

```bash
mise run controller
```

**Start the Agent (Terminal 3):**

```bash
mise run agent
```

`agent` uses the shared `configs/config.controller.dev.yaml` file and connects as the local `main` node.

To start a second local Agent with `node-2`, run:

```bash
mise run agent2
```

### Option 3: Prebuilt Image Stack

```bash
docker compose up -d
```

This Compose stack uses pre-built images. It is useful for integration checks or prod-like local runs, but it does not provide source hot reload and will not rebuild automatically when you edit code.

## Development Configuration Examples

### Controller Configuration

```yaml
# configs/config.controller.dev.yaml
controller:
  listen_addr: "127.0.0.1:7001"
  controller_addr: "http://127.0.0.1:7001"
  repo_dir: "./dev/repo-controller"
  state_dir: "./dev/state-controller"
  log_dir: "./dev/logs"
  cli_tokens:
    - name: "dev-admin"
      token: "dev-admin-token"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-agent-token"
```

### Agent Configuration

```yaml
# configs/config.agent.dev.yaml
agent:
  controller_addr: "http://127.0.0.1:7001"
  node_id: "node-2"
  token: "node-2-token"
  repo_dir: "./dev/repo-agent-node-2"
  state_dir: "./dev/state-agent-node-2"
```

## Test Multi-Node Scenarios

To test multi-node setups, start multiple agents:

**Agent 1:**

```bash
go run ./cmd/composia agent \
  -config ./configs/config.controller.dev.yaml
```

**Agent 2:**

```bash
go run ./cmd/composia agent \
  -config ./configs/config.agent.dev.yaml
```

## Web Editor Validation

The Web UI CodeMirror editor validates Compose files whose names match `compose*.yml`, `compose*.yaml`, `docker-compose*.yml`, or `docker-compose*.yaml`.

The editor also validates `.env` files and, for open Compose files, warns when `${VAR}` or `${VAR?message}` references are not defined by any open `.env` file from the same directory.

- Compose schema source: `https://github.com/compose-spec/compose-spec/blob/main/schema/compose-spec.json`
- Vendored schema path: `web/src/lib/schemas/compose-spec.json`
- Current implementation: `web/src/lib/codemirror/compose-lint.ts` and `web/src/lib/codemirror/env-lint.ts`

When you adjust Compose validation behavior, keep the upstream Compose specification schema URL above as the source of truth. If you refresh the vendored schema, replace `web/src/lib/schemas/compose-spec.json` from that upstream source and update this section if the source changes.

## Code Generation

### Generate Protobuf Code

After modifying `.proto` files, regenerate the Go code:

```bash
buf generate
```

## Project Structure

```
composia/
├── cmd/
│   └── composia/           # Main application entry
│       └── main.go
├── configs/                # Development configuration examples
├── docs/                   # Documentation (VitePress)
│   ├── content/
│   └── .vitepress/
├── gen/
│   └── go/                 # Generated protobuf code
├── internal/               # Internal packages
│   ├── controller/         # Controller implementation
│   ├── agent/              # Agent implementation
│   ├── repo/               # Service repo parsing and validation
│   ├── store/              # SQLite-backed state storage
│   └── ...
├── proto/                  # Protobuf source files
├── web/                    # SvelteKit frontend
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/ # UI components
│   │   │   └── server/     # Server-side controller access
│   │   └── routes/         # Page routes
│   └── package.json
├── docker-compose.yaml     # Compose stack for local/prod-like runs
└── README.md
```

## Key Directory Descriptions

| Directory | Description |
|-----------|-------------|
| `internal/controller/` | Controller business logic |
| `internal/agent/` | Agent business logic |
| `proto/` | Protobuf source definitions |
| `internal/store/` | Data storage layer |
| `web/src/lib/server/` | Server-side controller access |
| `web/src/lib/components/` | Reusable UI components |

## Code Standards

### Go Code

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for code formatting
- Use `golint` for style checking
- Add comments for important functions

### Frontend Code

- Use TypeScript strict mode
- Follow Svelte 5 syntax (using Runes)
- Use `$props()` to declare component properties
- Use `shadcn-svelte` UI component library

## Testing

### Run Backend Tests

```bash
go test ./...
```

### Run Frontend Checks

```bash
bun run web:check
```

## Debugging Tips

### Controller Debugging

```bash
# Run the controller with an explicit config file
go run ./cmd/composia controller -config ./configs/config.controller.dev.yaml
```

### Agent Debugging

```bash
go run ./cmd/composia agent -config ./configs/config.controller.dev.yaml
```

### View RPC Communication

The controller currently does not register gRPC reflection.

For RPC inspection, use the generated Connect clients in the Web app or call the registered ConnectRPC methods directly.

## Submitting Code

1. Ensure code passes tests
2. Follow [Conventional Commits](https://www.conventionalcommits.org/) specification
3. Run code formatting before committing

```bash
# Format Go code
gofmt -w .

# Format frontend code
cd web && bun run format
```

## Common Issues

**Q: Controller startup error "repo not initialized"**

A: You need to initialize the Git repository first: `git init dev/repo-controller`

**Q: Agent connection failed**

A: Check if the Controller address and Token match

**Q: Frontend requests failing**

A: Ensure the Controller is running and that `COMPOSIA_CONTROLLER_ADDR` and `COMPOSIA_CLI_TOKEN` are set correctly for the Web process
