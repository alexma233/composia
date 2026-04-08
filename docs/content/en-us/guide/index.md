# Introduction

Composia is a self-hosted service manager built around service definitions, a single control plane, and one or more execution agents.

## What is Composia?

Composia enables you to:

- **Manage Docker Compose services** - Use familiar Docker Compose YAML files to define services
- **Multi-node deployment** - Deploy services to multiple nodes (agents)
- **Centralized control** - Manage all services and nodes through a single control plane
- **Real-time monitoring** - View service status, logs, and resource usage

## Core Concepts

### Service Definitions

Composia uses `composia-meta.yaml` files to define service metadata, combined with standard `docker-compose.yaml` files:

```yaml
# composia-meta.yaml
name: my-service
description: My awesome service
version: "1.0"
```

### Control Plane

The control plane is the brain of Composia, responsible for:

- **Configuration management**: Loading service definitions from Git repositories
- **State aggregation**: Collecting status information from all agents
- **Task scheduling**: Assigning deployment tasks to appropriate agents
- **API services**: Providing Web UI and external integration interfaces

### Execution Agents

Agents run on actual Docker hosts and are responsible for:

- **Heartbeat communication**: Regularly reporting status to the control plane
- **Task execution**: Executing deployment, stop, restart, and other operations
- **Log collection**: Collecting and forwarding container logs
- **Resource monitoring**: Monitoring host and container resource usage

## Technology Stack

- **Backend**: Go 1.25+
- **Frontend**: SvelteKit + Bun
- **Runtime**: Docker Compose
- **Database**: SQLite
- **Communication**: ConnectRPC

## License

Composia is released under the AGPL-3.0 open source license.
