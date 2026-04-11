# Architecture Overview

Composia uses a control plane-agent (Controller-Agent) architecture that supports distributed multi-node service management.

## System Architecture

```mermaid
flowchart TB
    subgraph CP[Control Plane]
        WEB[Web UI\nSvelteKit]
        API[API Server\nConnectRPC]
        DB[(Task Queue / State\nSQLite)]
        REPO[(Git Repo\nService Definitions)]
    end

    WEB --> API
    API --> DB
    API --> REPO

    API --> A1[Agent 1\nNode A]
    API --> A2[Agent 2\nNode B]
    API --> AN[Agent N\nNode N]

    A1 --> D1[Docker Compose]
    A2 --> D2[Docker Compose]
    AN --> DN[Docker Compose]
```

## Core Components

### Control Plane (Controller)

The control plane is the central hub of the system, running in its own container:

| Function | Description |
|----------|-------------|
| Configuration Management | Loading and maintaining service definitions from Git repositories |
| State Aggregation | Collecting status information from all agents |
| Task Scheduling | Assigning deployment tasks to appropriate agents |
| API Services | Providing Web UI and external integration interfaces |
| Data Persistence | Using SQLite for tasks and state storage |

### Execution Agents

Agents run on target Docker hosts:

| Function | Description |
|----------|-------------|
| Heartbeat Communication | Regularly reporting status to the control plane (default: 15 seconds) |
| Task Execution | Executing deployment, stop, restart, and other operations |
| Log Collection | Collecting and forwarding container logs |
| Runtime Summary | Reports disk capacity and Docker inventory statistics |
| Docker Operations | Directly managing local Docker containers |

### Web Interface

A modern management interface built with SvelteKit:

- **Service Management**: Create, edit, and deploy services
- **Node Monitoring**: View status of all agent nodes
- **Container Operations**: View logs, execute commands
- **Task Tracking**: Monitor task execution progress in real-time

## Communication

### ConnectRPC

Composia uses ConnectRPC for inter-service communication:

- Bidirectional streaming based on HTTP/2
- Protobuf serialization
- Compatible with gRPC-style tooling and Connect clients over HTTP
- Supports browser direct calls

### Authentication

| Component | Authentication Method |
|-----------|----------------------|
| Web UI → Controller | Controller access token (Bearer, from `controller.access_tokens`) |
| Agent → Controller | Node Token |
| Controller → Agent | Bearer token when calling controller-exposed RPCs |

## Data Flow

### Deployment Flow

```mermaid
flowchart LR
    A[User Request] --> B[Controller Validation]
    B --> C[Create Task]
    C --> D[Agent Pull]
    D --> E[Execute Deploy]
    E --> F[Report Result]
```

1. User initiates a deployment request via Web UI or API
2. Controller validates service definition and permissions
3. Creates deployment tasks for each target node
4. Agent retrieves tasks via long-polling
5. Agent downloads service bundle and executes Docker Compose deployment
6. Agent reports execution result and container status

### Status Synchronization

```mermaid
flowchart LR
    A[Agent heartbeat / Docker stats reports] --> B[Controller aggregation]
    B --> C[Web UI]
```

- Agents send heartbeats every 15 seconds
- Heartbeats include node liveness and disk summary
- Agents also report Docker inventory statistics periodically
- Controller aggregates status from all agents into SQLite
- Web UI displays real-time status updates

## Core Object Model

```mermaid
flowchart TB
    S[Service<br/>Service Definition]
    SI1[ServiceInstance<br/>Node Instance A]
    SI2[ServiceInstance<br/>Node Instance B]
    C1[Container<br/>Docker Container]
    C2[Container<br/>Docker Container]

    S --> SI1
    S --> SI2
    SI1 --> C1
    SI2 --> C2
```

| Object | Description | Storage |
|--------|-------------|---------|
| Service | Logical service definition from Git repository | Git Repo |
| ServiceInstance | Deployment instance of a service on a specific node | SQLite |
| Container | Actual Docker container | Docker Daemon |
| Node | Docker host where an Agent runs | Controller Config |

## Security

| Layer | Measures |
|-------|----------|
| Authentication | Token-based authentication |
| Transport | TLS encryption supported (recommended for production) |
| Authorization | Principle of least privilege; agents only access assigned services |
| Secrets | Encrypted storage using age |

## Scalability

- **Horizontal Scaling**: Add more Agent nodes to manage more Docker hosts
- **Service Scaling**: Deploy the same service to multiple nodes
- **Load Balancing**: Multi-instance load balancing through Caddy configuration

## Deployment Patterns

### Single-Node Mode

```mermaid
flowchart TB
    subgraph SERVER[Single Server]
        C[Controller]
        A[Agent]
        D[Docker]
    end

    C <--> A
    A --> D
```

### Multi-Node Mode

```mermaid
flowchart TB
    C[Controller]
    A1[Agent 1]
    A2[Agent 2]
    A3[Agent 3]
    AN[Agent N]
    D1[Docker Node 1]
    D2[Docker Node 2]
    D3[Docker Node 3]
    DN[Docker Node N]

    C --> A1
    C --> A2
    C --> A3
    C --> AN
    A1 --> D1
    A2 --> D2
    A3 --> D3
    AN --> DN
```
