# Architecture

Composia uses a control plane-agent architecture that supports distributed service management.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Control Plane                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │   Web UI    │  │   API       │  │  Task Queue     │  │
│  │  (SvelteKit)│  │  (Connect)  │  │  (SQLite)       │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │  Git Repo   │  │  Service    │  │  Node Manager   │  │
│  │  (Config)   │  │  Registry   │  │                 │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────┘
                           │
           ┌───────────────┼───────────────┐
           │               │               │
    ┌──────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
    │   Agent 1   │ │   Agent 2   │ │   Agent N   │
    │  (Node A)   │ │  (Node B)   │ │  (Node C)   │
    │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌─────────┐ │
    │ │ Docker  │ │ │ │ Docker  │ │ │ │ Docker  │ │
    │ │ Compose │ │ │ │ Compose │ │ │ │ Compose │ │
    │ └─────────┘ │ │ └─────────┘ │ │ └─────────┘ │
    └─────────────┘ └─────────────┘ └─────────────┘
```

## Component Details

### Control Plane

The control plane is the central hub of the system, responsible for:

- **Configuration Management**: Loading service definitions from Git repositories
- **State Aggregation**: Collecting status information from all agents
- **Task Scheduling**: Assigning deployment tasks to appropriate agents
- **API Services**: Providing Web UI and external integration interfaces

### Execution Agents

Agents run on target Docker hosts and are responsible for:

- **Heartbeat Communication**: Regularly reporting status to the control plane
- **Task Execution**: Executing deployment, stop, restart, and other operations
- **Log Collection**: Collecting and forwarding container logs
- **Resource Monitoring**: Monitoring host and container resource usage

### Communication Protocol

Composia uses ConnectRPC for inter-service communication, providing:

- Bidirectional streaming based on HTTP/2
- Protobuf serialization
- Compatibility with gRPC and REST clients

## Data Flow

### Deployment Process

1. User initiates a deployment request via Web UI or API
2. Control plane validates the service definition
3. Control plane selects the target agent node(s)
4. Task enters the queue for execution
5. Agent retrieves the task and executes the deployment
6. Agent reports the execution result

### Status Synchronization

- Agents send heartbeats every 5 seconds
- Heartbeats include node status and container list
- Control plane aggregates status from all agents
- Web UI displays real-time status updates

## Security

- Agents use token-based authentication
- All communication is encrypted with TLS (production environments)
- Principle of least privilege: Agents can only access assigned services
