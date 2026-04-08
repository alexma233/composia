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

### Option 2: Use Docker Compose (Recommended)

```bash
docker compose -f docker-compose.dev.yaml up -d
```

## Development Configuration Examples

### Controller Configuration

```yaml
# configs/config.controller.dev.yaml
listen_addr: ":7001"
controller_addr: "http://localhost:7001"
repo_dir: "./repo-controller"
state_dir: "./state-controller"
log_dir: "./logs"
cli_tokens:
  - name: "dev-admin"
    token: "dev-token-change-in-production"
    enabled: true
nodes:
  - id: "local"
    display_name: "Local Development"
    enabled: true
    token: "local-agent-token"
```

### Agent Configuration

```yaml
# configs/config.agent.dev.yaml
controller_addr: "http://localhost:7001"
node_id: "local"
token: "local-agent-token"
repo_dir: "./repo-agent"
state_dir: "./state-agent"
```

## Test Multi-Node Scenarios

To test multi-node setups, start multiple agents:

**Agent 1:**

```bash
go run ./cmd/composia agent \
  -config ./configs/config.agent1.dev.yaml
```

**Agent 2:**

```bash
go run ./cmd/composia agent \
  -config ./configs/config.agent2.dev.yaml
```

## Code Generation

### Generate Protobuf Code

After modifying `.proto` files, regenerate the Go code:

```bash
buf generate
```

### Generate Frontend API Client

```bash
cd web
bun run generate:api
```

## Project Structure

```
composia/
├── cmd/
│   └── composia/           # Main application entry
│       ├── main.go
│       ├── controller.go   # Controller command
│       └── agent.go        # Agent command
├── configs/                # Development configuration examples
├── docs/                   # Documentation (VitePress)
│   ├── content/
│   └── .vitepress/
├── gen/
│   └── go/                 # Generated protobuf code
├── internal/               # Internal packages
│   ├── controller/         # Controller implementation
│   ├── agent/              # Agent implementation
│   ├── proto/              # Protobuf definitions
│   └── ...
├── proto/                  # Protobuf source files
├── web/                    # SvelteKit frontend
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/ # UI components
│   │   │   └── api/        # API client
│   │   └── routes/         # Page routes
│   └── package.json
├── docker-compose.yaml     # Production deployment config
├── docker-compose.dev.yaml # Development deployment config
└── README.md
```

## Key Directory Descriptions

| Directory | Description |
|-----------|-------------|
| `internal/controller/` | Controller business logic |
| `internal/agent/` | Agent business logic |
| `internal/proto/` | Protobuf message definitions |
| `internal/service/` | Shared service layer |
| `internal/store/` | Data storage layer |
| `web/src/lib/api/` | Frontend API calls |
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

### Run Frontend Tests

```bash
cd web
bun test
```

## Debugging Tips

### Controller Debugging

```bash
# Enable verbose logging
go run ./cmd/composia controller -config ... -v

# Or set environment variable
LOG_LEVEL=debug go run ./cmd/composia controller ...
```

### Agent Debugging

```bash
LOG_LEVEL=debug go run ./cmd/composia agent ...
```

### View gRPC Communication

Use [grpcui](https://github.com/fullstorydev/grpcui) or [grpcurl](https://github.com/fullstorydev/grpcurl):

```bash
# Reflection mode (enable in development)
grpcui -plaintext localhost:7001
```

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

**Q: Frontend API calls failing**

A: Ensure the Controller is running and check the `VITE_API_URL` configuration
