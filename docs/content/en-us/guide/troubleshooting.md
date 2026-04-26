# Troubleshooting

Common issues and how to fix them.

## Agent Cannot Connect to Controller

**Symptoms:** Agent container keeps restarting, node shows as offline in the Web UI.

**Check:**
- Is `controller_addr` correct in `config/config.yaml`? The agent must be able to reach the controller over the network.
- Do the `controller.nodes[].token` and `agent.token` values match?
- Network connectivity between the agent host and controller host — check firewalls and DNS.
- Is the controller running? Check with `docker compose ps`.

## Deployment Failed

**Symptoms:** Deploy task ends in `failed` status.

**Check:**
1. Navigate to the **Tasks** page and find the failed task to view detailed logs.
2. Verify the `docker-compose.yaml` syntax is valid.
3. Is the image pullable? Check image name and network access.
4. Port conflicts — check if the required ports are already in use.
5. Missing environment variables — check the `.env` file in the service directory.

## Container Won't Start

**Symptoms:** Container shows as created but not running.

**Check:**
1. Navigate to the **Containers** page, find the target container, and view its logs.
2. Check environment variables and volume mounts in the service's `docker-compose.yaml`.
3. Check system resource limits (CPU, memory, disk).

## Service Status Inconsistent

**Symptoms:** Web UI shows a status that doesn't match actual container state.

**Check:**
- Is the Agent online? Agents send heartbeats every 15 seconds.
- Are containers actually running? Check directly with `docker ps` on the node.
- Are the `composia.service` and `composia.instance` labels correctly set on containers?

## Caddy Configuration Not Applied

**Symptoms:** Changes to Caddy fragments don't take effect.

**Check:**
1. Is `network.caddy.enabled` set to `true` in the service's `composia-meta.yaml`?
2. Is the `Caddyfile.fragment` path correct (relative to the service directory)?
3. Is the Caddy infrastructure service running?
4. Trigger a manual `caddy_sync` and `caddy_reload` from the Web UI if needed.

## Docker Socket Permission Denied

**Symptoms:** Agent logs show `permission denied while trying to connect to the docker API`.

**Fix:** Set `DOCKER_SOCK_GID` in `.env` to the GID of the host's `/var/run/docker.sock`:

```bash
ls -ln /var/run/docker.sock
# srw-rw---- 1 0 131 0 ... — use "131"
```

## Debug Mode

Reproduce operational issues locally with explicit config files:

```bash
# Controller
go run ./cmd/composia controller -config ./dev/config.controller.yaml

# Agent
go run ./cmd/composia agent -config ./dev/config.controller.yaml
```

## Log Locations

| Log Source | Location |
|------------|----------|
| Task execution logs | `log_dir/tasks/<task-id>.log` |
| Container logs | Retrieved in real-time via Docker API |
| Controller logs | Docker container logs (`docker compose logs controller`) |
| Agent logs | Docker container logs (`docker compose logs agent`) |

Enable Docker log rotation to prevent disk exhaustion:

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

## Getting Help

- [GitHub Issues](https://github.com/alexma233/composia/issues)
- [Development Guide](./development)
