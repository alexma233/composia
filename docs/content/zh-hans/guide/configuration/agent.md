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

同时需要部署 Caddy 基础设施服务，参考 [网络配置](../networking)。
