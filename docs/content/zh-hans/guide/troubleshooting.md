# 故障排查

常见问题及其解决方法。

## Agent 无法连接 Controller

**症状：** Agent 容器不断重启，节点在 Web UI 中显示为离线。

**检查：**
- `config/config.yaml` 中的 `controller_addr` 是否正确？Agent 必须能通过网络访问 Controller。
- `controller.nodes[].token` 和 `agent.token` 的值是否一致？
- Agent 主机与 Controller 主机之间的网络连通性——检查防火墙和 DNS。
- Controller 是否在运行？用 `docker compose ps` 检查。

## 部署失败

**症状：** 部署任务以 `failed` 状态结束。

**检查：**
1. 进入 **Tasks** 页面，找到失败的任务，查看详细日志。
2. 验证 `docker-compose.yaml` 语法是否正确。
3. 镜像是否可拉取？检查镜像名称和网络访问。
4. 端口冲突——检查所需端口是否已被占用。
5. 环境变量缺失——检查服务目录中的 `.env` 文件。

## 容器无法启动

**症状：** 容器显示为已创建但未运行。

**检查：**
1. 进入 **Containers** 页面，找到目标容器，查看其日志。
2. 检查服务的 `docker-compose.yaml` 中的环境变量和卷挂载。
3. 检查系统资源限制（CPU、内存、磁盘）。

## 服务状态不一致

**症状：** Web UI 显示的状态与实际容器状态不匹配。

**检查：**
- Agent 是否在线？Agent 每 15 秒发送一次心跳。
- 容器是否实际在运行？在节点上直接用 `docker ps` 检查。
- 容器上的 `composia.service` 和 `composia.instance` 标签是否正确设置？

## Caddy 配置未生效

**症状：** 对 Caddy 片段的修改没有生效。

**检查：**
1. 服务的 `composia-meta.yaml` 中的 `network.caddy.enabled` 是否设为 `true`？
2. `Caddyfile.fragment` 路径是否正确（相对于服务目录）？
3. Caddy 基础设施服务是否在运行？
4. 必要时从 Web UI 手动触发 `caddy_sync` 和 `caddy_reload`。

## Docker Socket 权限拒绝

**症状：** Agent 日志显示 `permission denied while trying to connect to the docker API`。

**修复：** 在 `.env` 中将 `DOCKER_SOCK_GID` 设为主机 `/var/run/docker.sock` 的 GID：

```bash
ls -ln /var/run/docker.sock
# srw-rw---- 1 0 131 0 ... — 使用 "131"
```

## 调试模式

使用显式配置文件在本地复现运维问题：

```bash
# Controller
go run ./cmd/composia controller -config ./dev/config.controller.yaml

# Agent
go run ./cmd/composia agent -config ./dev/config.controller.yaml
```

## 日志位置

| 日志来源 | 位置 |
|------------|----------|
| 任务执行日志 | `log_dir/tasks/<task-id>.log` |
| 容器日志 | 通过 Docker API 实时获取 |
| Controller 日志 | Docker 容器日志 (`docker compose logs controller`) |
| Agent 日志 | Docker 容器日志 (`docker compose logs agent`) |

启用 Docker 日志轮转以防止磁盘耗尽：

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

## 获取帮助

- [GitHub Issues](https://github.com/alexma233/composia/issues)
- [开发指南](./development)
