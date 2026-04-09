# Introduction

Composia is a self-hosted Docker Compose management platform built around service definitions, a single control plane, and multiple execution agents.

## What is Composia?

Composia enables you to:

- **Manage Docker Compose Services** — Define services with `docker-compose.yaml` plus a small `composia-meta.yaml`
- **Multi-Node Deployment** — Deploy services to multiple nodes (agents) across your infrastructure
- **Centralized Control** — Manage all services and nodes through a single control plane
- **Operational Visibility** — View service status, task logs, node summaries, and node-scoped Docker details

## Use Cases

- Individuals or teams managing multiple Docker services
- Deploying and coordinating services across multiple servers
- Managing containerized applications through an intuitive web interface
- Automating backups, DNS management, and other operational tasks

## Core Concepts

### Service Definitions

Composia uses `composia-meta.yaml` for service metadata together with standard `docker-compose.yaml` files:

```yaml
# composia-meta.yaml
name: my-service
nodes:
  - main
```

### Control Plane

The control plane is the core of Composia, responsible for:

- **Service Management**: Loading and maintaining service definitions from Git repositories
- **State Aggregation**: Collecting status information from all agents
- **Task Scheduling**: Assigning deployment tasks to appropriate agents
- **API Services**: Providing Web UI and external integration interfaces

### Execution Agents

Agents run on actual Docker hosts and are responsible for:

- **Heartbeat Communication**: Regularly reporting status to the control plane
- **Task Execution**: Executing deployment, stop, restart, and other operations
- **Task and Runtime Reporting**: Reporting task results, logs, and Docker inventory summaries back to the Controller

## Technology Stack

| Component | Technology |
|-----------|------------|
| Backend | Go 1.25+ |
| Frontend | SvelteKit + Bun |
| Runtime | Docker Compose |
| Database | SQLite |
| Communication | ConnectRPC |

## Documentation

- [Quick Start](./quick-start) — Get up and running in minutes
- [Core Concepts](./core-concepts) — Understand how Composia works
- [Architecture](./architecture) — System architecture overview
- [Configuration Guide](./configuration) — Platform and service configuration
- [Service Definition](./service-definition) — How to define and manage services
- [Deployment](./deployment) — Deploy, update, stop, and restart services
- [Networking](./networking) — DNS and reverse proxy configuration
- [Backup & Migration](./backup-migrate) — Data protection and migration strategies
- [Operations](./operations) — Task system and resource management
- [Development](./development) — Set up local development environment
- [API Reference](./api/) — Generated RPC reference from protobuf comments

## License

Composia is released under the [AGPL-3.0](https://www.gnu.org/licenses/agpl-3.0.html) open source license.
