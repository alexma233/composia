# Operations

This document explains Composia's task system, resource management, and common operational tasks.

## Task System

### Overview

Composia uses a task queue to manage all asynchronous operations:

- Controller is responsible for task creation and status management
- Agents actively pull tasks belonging to them via long-polling
- Tasks report status, logs, and results step by step

### Task Types

| Task Type | Description | Trigger |
|-----------|-------------|---------|
| `deploy` | Deploy service | Manual/API |
| `update` | Update service | Manual/API |
| `stop` | Stop service | Manual/API |
| `restart` | Restart service | Manual/API |
| `backup` | Execute backup | Manual/API/Scheduled |
| `restore` | Restore backup data | Manual/API |
| `dns_update` | Update DNS records | Migration/Manual |
| `migrate` | Migrate service | Manual/API |
| `caddy_sync` | Sync Caddy configuration | Automatic |
| `caddy_reload` | Reload Caddy | Automatic |
| `prune` | Clean up resources | Manual/API |
| `rustic_forget` | Clean rustic snapshot references | Manual/API/Scheduled |
| `rustic_prune` | Run rustic prune | Manual/API/Scheduled |
| `docker_list` | Fetch node-scoped Docker resource lists | Web UI/API |
| `docker_inspect` | Inspect one Docker resource | Web UI/API |
| `docker_start` | Start a container | Web UI/API |
| `docker_stop` | Stop a container | Web UI/API |
| `docker_restart` | Restart a container | Web UI/API |
| `docker_logs` | Fetch container logs | Web UI/API |

### Task Lifecycle

```
Pending → Running → Succeeded
         │         │
         │         ├─► Failed
         │         │
         │         └─► Cancelled
         │
         └─► Awaiting confirmation
```

### Viewing Tasks

**Web UI:**
- **Tasks** page shows all tasks
- Filter by service, node, type, status
- Click task to view detailed logs

**Task Status:**

| Status | Description |
|--------|-------------|
| `pending` | Waiting to start |
| `running` | Currently executing |
| `awaiting_confirmation` | Waiting for an external confirmation step |
| `succeeded` | Execution successful |
| `failed` | Execution failed |
| `cancelled` | Cancelled |

### Task Logs

Real-time logs are emitted during task execution. Built-in task logs currently come mostly from the agent and Docker commands, so they are primarily in English:

```
starting remote deploy task for service=my-app node=main repo_revision=4f3c2a1b
render step completed after bundle download
Container my-app-1  Started
finalize step completed after compose up
deploy task finished successfully
```

Task sources include:

- `web`: triggered from the Web UI, including follow-up tasks derived from a Web-triggered action
- `cli`: triggered manually by CLI or API, including follow-up tasks derived from a CLI-triggered action
- `others`: triggered by third-party extensions or integrations, including follow-up tasks derived from those actions
- `schedule`: triggered by the controller's built-in scheduler
- `system`: triggered by internal controller workflows

## Docker Resource Management

### Container Management

Agents regularly report Docker container information from nodes, and Controller provides a unified browsing interface.

**Viewing Containers:**
1. Navigate to a node detail page
2. Open the node's Docker container view
3. View container status, image, ports, and labels

**Container Operations:**

| Operation | Description |
|-----------|-------------|
| View Logs | Fetch the latest container logs from the node |
| Inspect | View container metadata and runtime details |
| Terminal | Open an exec session into the container (still basic) |

**Viewing Container Logs:**

```
# In Web UI
1. Find target container
2. Click **Logs** button
3. Load the latest log output from that container
```

**Container Terminal:**

The Web UI exposes a container exec entrypoint, but the terminal experience is still basic:

1. Click the **Terminal** button on a container
2. Select a shell such as `/bin/sh` or `bash`
3. Establish the WebSocket session and run commands

Availability depends on the node exec tunnel being online and the selected shell existing in the container.

### Image Management

**Viewing Images:**
- Each node has its own Docker images page
- Displays image tags, size, and creation time

**Cleaning Images:**
Use the Web UI or call the ConnectRPC method `composia.controller.v1.NodeMaintenanceService/PruneNodeDocker`.

### Network Management

**Viewing Networks:**
- Each node has its own Docker networks page
- View network driver, subnet, and connected containers

### Volume Management

**Viewing Volumes:**
- Each node has its own Docker volumes page
- View volume labels and mount metadata exposed by Docker

## Node Management

### Node Status

Agents send heartbeats every 15 seconds containing:

| Information | Description |
|-------------|-------------|
| Online Status | Whether connected to Controller |
| Docker Version | Node Docker version |
| Container Count | Number of running containers |
| Resource Usage | Disk capacity plus Docker inventory counts |
| Service Instances | List of service instances on this node |

### Node Views

**Web UI provides the following views:**

- **Node List**: Overview of all nodes
- **Node Detail**: Detailed information for a single node
- **Node Docker Views**: Node-scoped containers, images, networks, and volumes
- **Dashboard**: Service, node, and recent task summaries

### Node Operations

**Reconnect Agent:**

If Agent disconnects:
1. Check Agent container logs
2. Check network connectivity
3. Restart Agent container

```bash
docker compose restart agent
```

## Resource Cleanup

### Cleanup Tasks

Execute `prune` tasks to clean up unused resources:

**Web UI:**
1. Navigate to **Nodes** page
2. Select target node
3. Click **Clean** button
4. Select resource types to clean

**API:**

The current controller does not expose REST endpoints under `/api/v1/...`.
Use the ConnectRPC method `composia.controller.v1.NodeMaintenanceService/PruneNodeDocker` instead.

### Auto-Cleanup Recommendations

Docker `prune` is still best handled by external automation when you need recurring cleanup.

For rustic, `forget` and `prune` can be scheduled directly by the controller's built-in scheduler. See [Backup & Migration](./backup-migrate) and [Backup Configuration](./configuration/backup).

## Log Management

### Task Logs

Task logs are stored in Controller's `log_dir`:

```
log_dir/
├── tasks/
│   ├── <task-id-1>.log
│   ├── <task-id-2>.log
│   └── <task-id-3>.log
```

### Container Logs

Container logs are retrieved in real-time via Docker API; historical logs are managed by Docker.

### Log Retention Policy

Recommended log rotation configuration:

```yaml
# docker-compose.yaml
services:
  controller:
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "5"
```

## Monitoring and Alerting

### Current Monitoring Capabilities

- **Real-time Status**: Web UI displays service, container, and node status in real-time
- **Resource Usage**: Node disk capacity and Docker inventory counts
- **Log Viewing**: Streaming task logs and on-demand container log fetches

### Recommended Monitoring Solutions

**Integrate Prometheus + Grafana:**

Deploy node-exporter and cadvisor on nodes to be monitored:

```yaml
# monitoring/docker-compose.yaml
services:
  node-exporter:
    image: prom/node-exporter
    volumes:
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /:/rootfs:ro

  cadvisor:
    image: gcr.io/cadvisor/cadvisor
    volumes:
      - /:/rootfs:ro
      - /var/run:/var/run:ro
      - /sys:/sys:ro
      - /var/lib/docker:/var/lib/docker:ro
```

**Custom Alerts:**

Use ConnectRPC query methods such as `composia.controller.v1.ServiceQueryService/GetService` together with external alerting systems.

## Troubleshooting

### Common Issues

**1. Agent Cannot Connect to Controller**

Check:
- Is Controller address correct?
- Do Tokens match?
- Network connectivity
- Firewall settings

**2. Deployment Failed**

Check:
- Error messages in task logs
- Docker Compose file syntax
- Is image pullable?
- Port conflicts

**3. Service Status Inconsistent**

Check:
- Is Agent online?
- Are containers actually running?
- Are labels correctly set?

**4. Caddy Configuration Not Applied**

Check:
- Caddy infrastructure service status
- Configuration fragment syntax
- Agent directory mounts

### Debug Mode

Use the explicit config files below when reproducing operational issues locally:

```bash
# Controller
go run ./cmd/composia controller -config ./dev/config.controller.yaml

# Agent
go run ./cmd/composia agent -config ./dev/config.controller.yaml
```

### Getting Support

- View [GitHub Issues](https://github.com/alexma233/composia/issues)
- Refer to [Development Guide](./development)
- Check log files

## Performance Optimization

### Controller Optimization

- Use SSD storage for `state_dir`
- Regularly clean old task logs
- Set appropriate `pull_interval`

### Agent Optimization

- Ensure smooth Docker socket access
- Monitor Agent resource usage
- Regularly clean unused resources

## Related Documentation

- [Deployment](./deployment) — Service deployment operations
- [Backup & Migration](./backup-migrate) — Data protection operations
- [DNS Configuration](./dns) — DNS configuration and updates
- [Caddy Configuration](./caddy) — Proxy configuration and automated sync
