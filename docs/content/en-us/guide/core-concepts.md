# Core Concepts

Understanding Composia's four core objects helps you better use the platform for service management.

## The Four Core Objects

Composia is designed around four primary objects:

| Object | Description | Example |
|--------|-------------|---------|
| **Service** | Logical service definition | `my-app` service configuration |
| **ServiceInstance** | Deployment instance of a service on a specific node | `my-app` on `node-1` |
| **Container** | Actual Docker container | `my-app-web-1` |
| **Node** | Docker host where an Agent runs | `node-1`, `node-2` |

## Object Relationships

```
┌─────────────────────────────────────┐
│           Service                   │
│  (Service definition in Git repo)   │
└──────────────┬──────────────────────┘
               │
       ┌───────┴───────┐
       ▼               ▼
┌──────────────┐ ┌──────────────┐
│ServiceInstance│ │ServiceInstance│
│  on Node A   │ │  on Node B   │
└──────┬───────┘ └──────┬───────┘
       │                │
       ▼                ▼
┌──────────────┐ ┌──────────────┐
│  Container   │ │  Container   │
│  (Docker)    │ │  (Docker)    │
└──────────────┘ └──────────────┘
```

### Service

A Service is a logical-level definition that originates from a directory in the Git repository:

```
repo/
└── my-service/
    ├── composia-meta.yaml    # Service metadata
    └── docker-compose.yaml   # Compose configuration
```

**Characteristics:**
- Uses `composia-meta.yaml` to define service properties
- Contains Docker Compose configuration
- Can be deployed to multiple nodes
- The control plane stores the desired state

### ServiceInstance

A ServiceInstance represents the deployment of a service on a specific node:

**Characteristics:**
- Uniquely identified by Service + Node
- Represents the desired deployment on that node
- The Agent is responsible for converging actual state to desired state
- Each instance can have independent deployment status

**Example:**

```yaml
# my-service/composia-meta.yaml
name: my-service
nodes:
  - main
  - edge
```

This creates two ServiceInstances:
- `my-service` on `main`
- `my-service` on `edge`

### Container

Containers are the actual processes running in Docker:

**Characteristics:**
- Associated with a ServiceInstance through Compose labels
- Can be managed independently (view logs, restart, etc.)
- May exist outside the ServiceInstance lifecycle
- Agents regularly report container status

**Label Association:**

```yaml
# docker-compose.yaml
services:
  web:
    labels:
      - "composia.service=my-service"
      - "composia.instance=my-service-main"
```

### Node

A Node is a Docker host where an Agent runs:

**Characteristics:**
- Pre-declared in Controller configuration
- Each node has a unique ID and authentication Token
- Agent connects to Controller using the Token
- Can have custom properties (such as public IP)

**Controller Configuration Example:**

```yaml
controller:
  nodes:
    - id: "main"
      display_name: "Main Server"
      enabled: true
      token: "main-agent-token"
      public_ipv4: "203.0.113.10"
    
    - id: "edge"
      display_name: "Edge Node"
      enabled: true
      token: "edge-agent-token"
```

## Layered Views

Different pages in the Web UI correspond to different object levels:

| Page | Object Level | Function |
|------|--------------|----------|
| Service List | Service | Manage all service definitions |
| Service Detail | Service + ServiceInstance | View node distribution of services |
| Instance Detail | ServiceInstance + Container | Manage containers of a specific instance |
| Container List | Container | Browse all Docker containers |
| Node List | Node | View all agent nodes |

## State Flow

```
Git Repo (Desired State)
       │
       ▼
Controller (Coordination)
       │
       ▼
ServiceInstance (Expansion)
       │
       ▼
Agent (Pull)
       │
       ▼
Docker (Actual State)
```

1. **Definition Phase**: Users define Services in the Git repository
2. **Coordination Phase**: Controller scans and parses Services, creates ServiceInstances
3. **Deployment Phase**: Agent retrieves tasks and creates actual Containers
4. **Synchronization Phase**: Agent regularly reports Container status

## Typical Use Cases

### Scenario 1: Single-Node Deployment

```yaml
# composia-meta.yaml
name: my-app
nodes:
  - main
```

- 1 Service
- 1 ServiceInstance (on main)
- N Containers

### Scenario 2: Multi-Node Deployment

```yaml
# composia-meta.yaml
name: my-app
nodes:
  - main
  - edge-1
  - edge-2
```

- 1 Service
- 3 ServiceInstances (one per node)
- N Containers per node

### Scenario 3: Multi-Service Multi-Node

```
Services:
  - web (nodes: [main, edge-1])
  - api (nodes: [main])
  - db (nodes: [main])

ServiceInstances:
  - web-main
  - web-edge-1
  - api-main
  - db-main
```

## Related Documentation

- [Service Definition](./service-definition) — How to define Services
- [Controller Configuration](./configuration/controller) — How to configure Nodes
- [Deployment](./deployment) — Managing ServiceInstance lifecycle
