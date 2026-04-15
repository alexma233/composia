# Agent 配置

本文档说明 `config/config.yaml` 中的 `agent` 段。

## 配置项

| 配置项 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `controller_addr` | string | 是 | Controller API 地址 |
| `node_id` | string | 是 | 节点 ID，必须匹配 Controller 配置 |
| `token` | string | 是 | 节点认证 Token |
| `repo_dir` | string | 是 | 本地服务 bundle 目录 |
| `state_dir` | string | 是 | 本地运行状态目录 |
| `caddy.generated_dir` | string | 否 | Caddy 配置片段输出目录 |

如果 `agent.controller_addr` 以 `http://` 开头，只应在受信任的反向代理负责 TLS 终止，或 Controller 仅暴露在受信任的本地网络内时使用。不要在不受信任的明文网络上传输 agent token。

## 与 Controller 同文件时的约束

如果同一个文件同时包含 `controller` 和 `agent`，还需要满足以下约束：

- `agent.node_id` 必须是 `main`
- `controller.nodes` 必须包含 `main`
- `controller.repo_dir` 和 `agent.repo_dir` 不能相同

## 最小示例

```yaml
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
```

## 重要路径约束

如果 `agent` 通过 `/var/run/docker.sock` 调用宿主机 Docker，那么 `agent.repo_dir` 和 `agent.state_dir` 不能只存在于 agent 容器自己的 volume 里。

以下三个位置必须使用同一个绝对路径：

- `config.yaml` 里的 `agent.repo_dir` / `agent.state_dir`
- `docker-compose.yaml` 中 agent 服务的宿主机挂载源路径
- `docker-compose.yaml` 中 agent 服务的容器内挂载目标路径

例如默认部署应保持为：

```yaml
agent:
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
```

```yaml
services:
  agent:
    volumes:
      - /data/repo-agent:/data/repo-agent
      - /data/state-agent:/data/state-agent
```

不要把宿主机路径改成别的值再映射到容器内的 `/data/...`，例如：

```yaml
services:
  agent:
    volumes:
      - /srv/composia/repo-agent:/data/repo-agent
```

这种写法会让 agent 容器内看到 `/data/repo-agent`，但宿主机 Docker daemon 实际只能看到 `/srv/composia/repo-agent`。当服务 Compose 使用 `/data/repo-agent/...` 这样的 bind mount 时，宿主机 Docker 会去访问宿主机的 `/data/repo-agent/...`，最终导致文件挂载失败，或者把不存在的文件路径错误创建成目录。

## 启用 Caddy

在 Agent 配置中添加：

```yaml
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
  caddy:
    generated_dir: "/srv/caddy/generated"
```

同时需要部署 Caddy 基础设施服务，参考 [Caddy 配置](../caddy)。
