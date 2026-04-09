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
| `backup` | Execute backup | Manual/API |
| `dns_update` | Update DNS records | Automatic/Manual |
| `caddy_sync` | Sync Caddy configuration | Automatic |
| `caddy_reload` | Reload Caddy | Automatic |
| `prune` | Clean up resources | Manual/API |
| `migrate` | Migrate service | Manual/API |

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

Real-time logs are output during task execution:

```
[2024-01-15 10:30:00] Starting deployment of service my-app to node main
[2024-01-15 10:30:01] Downloading service bundle...
[2024-01-15 10:30:05] Rendering runtime directory...
[2024-01-15 10:30:06] Executing docker compose up -d
[2024-01-15 10:30:15] Container started successfully
[2024-01-15 10:30:16] Syncing Caddy configuration...
[2024-01-15 10:30:18] Deployment completed
```

## Docker Resource Management

### Container Management

Agents regularly report Docker container information from nodes, and Controller provides a unified browsing interface.

**Viewing Containers:**
1. Navigate to **Containers** page
2. Filter containers by node
3. View container status, image, ports, etc.

**Container Operations:**

| Operation | Description |
|-----------|-------------|
| View Logs | View container logs in real-time |
| Start | Start a stopped container |
| Stop | Stop a running container |
| Restart | Restart container |
| Terminal | Enter container to execute commands |

**Viewing Container Logs:**

```
# In Web UI
1. Find target container
2. Click **Logs** button
3. View real-time or search historical logs
```

**Container Terminal:**

```bash
# Web UI provides basic terminal functionality
1. Click **Terminal** button on container
2. Select shell (bash/sh)
3. Execute commands
```

### Image Management

**Viewing Images:**
- **Images** page shows all images on all nodes
- Displays image tags, size, creation time

**Cleaning Images:**
Use the Web UI or call the ConnectRPC method `composia.controller.v1.NodeMaintenanceService/PruneNodeDocker`.

### Network Management

**Viewing Networks:**
- **Networks** page shows Docker networks
- View network driver, subnet, connected containers

### Volume Management

**Viewing Volumes:**
- **Volumes** page shows Docker volumes
- View volume size, mount points

## Node Management

### Node Status

Agents send heartbeats every 15 seconds containing:

| Information | Description |
|-------------|-------------|
| Online Status | Whether connected to Controller |
| Docker Version | Node Docker version |
| Container Count | Number of running containers |
| Resource Usage | CPU, memory, disk usage |
| Service Instances | List of service instances on this node |

### Node Views

**Web UI provides the following views:**

- **Node List**: Overview of all nodes
- **Node Detail**: Detailed information and resource usage of a single node
- **Service Instances**: Service deployment status on nodes
- **Dashboard**: Overall resource usage trends

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

Set up external automation to call the ConnectRPC maintenance method regularly if you want recurring cleanup.

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
- **Resource Usage**: Node CPU, memory, disk usage
- **Log Viewing**: Real-time container and task logs

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
go run ./cmd/composia controller -config ./configs/config.controller.dev.yaml

# Agent
go run ./cmd/composia agent -config ./configs/config.controller.dev.yaml
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

### Database Optimization

SQLite performance optimization:

```sql
-- Regularly execute VACUUM
VACUUM;

-- Check database integrity
PRAGMA integrity_check;
```

## Related Documentation

- [Deployment](./deployment) — Service deployment operations
- [Backup & Migration](./backup-migrate) — Data protection operations
- [Networking](./networking) — DNS and proxy configuration
