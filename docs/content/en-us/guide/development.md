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

## Environment Setup

### 1. Clone the Repository

```bash
git clone https://forgejo.alexma.top/alexma233/composia.git
cd composia
```

### 2. Install Frontend Dependencies

```bash
cd web
bun install
cd ..
```

### 3. Initialize Development Configuration

```bash
# Create required directories
mkdir -p repo-controller repo-agent

# Initialize Git repository (required for Controller)
git init repo-controller
```

## Start Development Environment

### Option 1: Start Frontend and Backend Separately

**Start the frontend development server:**

```bash
cd web
bun run dev
```

The frontend will be available at `http://localhost:5173`.

**Start the Controller (Terminal 2):**

```bash
go run ./cmd/composia controller \
  -config ./configs/config.controller.dev.yaml
```

**Start the Agent (Terminal 3):**

```bash
go run ./cmd/composia agent \
  -config ./configs/config.agent.dev.yaml
```

### Option 2: Use Docker Compose

```bash
docker compose up -d
```

## Development Configuration Examples

### Controller Configuration

```yaml
# configs/config.controller.dev.yaml
controller:
  listen_addr: "127.0.0.1:7001"
  controller_addr: "http://127.0.0.1:7001"
  repo_dir: "./repo-controller"
  state_dir: "./state-controller"
  log_dir: "./logs"
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
  repo_dir: "./repo-agent-node-2"
  state_dir: "./state-agent-node-2"
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

A: You need to initialize the Git repository first: `git init repo-controller`

**Q: Agent connection failed**

A: Check if the Controller address and Token match

**Q: Frontend requests failing**

A: Ensure the Controller is running and that `COMPOSIA_CONTROLLER_ADDR` and `COMPOSIA_CLI_TOKEN` are set correctly for the Web process
