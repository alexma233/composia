# Controller 配置

本文档说明 `config/config.yaml` 中的 `controller` 段。

## 基础配置

| 配置项 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `listen_addr` | string | 是 | Controller 监听地址，如 `:7001` |
| `controller_addr` | string | 是 | Agent 和 Web UI 访问 Controller 的地址 |
| `repo_dir` | string | 是 | Git 工作树目录，保存服务定义 |
| `state_dir` | string | 是 | SQLite 和运行时状态目录 |
| `log_dir` | string | 是 | 任务日志持久化目录 |
| `nodes` | array | 是 | 顶层字段必须出现，即使为空数组也要写出 |

## Controller 访问 token（`access_tokens`）

```yaml
access_tokens:
  - name: "admin"
    token: "your-secure-token-here"
    enabled: true
  - name: "automation"
    token: "automation-token"
    enabled: true
```

| 字段 | 说明 |
|------|------|
| `name` | 必填的 Token 名称，用于识别 |
| `token` | 必填的 Token 值，供 Web UI、CLI 或自定义客户端访问 Controller |
| `enabled` | 是否启用该 Token |
| `comment` | 可选的运维备注 |

Composia 当前没有 RBAC。所有已启用的访问 Token 都拥有完整的 Controller 访问权限，Token 名称不会影响权限。

Token 值必须在 `controller.access_tokens[].token` 和 `controller.nodes[].token` 两处全局唯一。配置加载器会拒绝重复值，以及这两类 Token 之间的冲突。

安全建议：

- 使用强随机字符串作为 Token
- 生产环境使用不同的 Token
- 定期轮换 Token

## 节点配置

```yaml
nodes:
  - id: "main"
    display_name: "Main Server"
    enabled: true
    token: "main-agent-token"
    public_ipv4: "203.0.113.10"
    public_ipv6: "2001:db8::1"
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `id` | 是 | 节点唯一标识，Agent 的 `node_id` 必须匹配 |
| `display_name` | 否 | 显示名称，用于 Web UI |
| `enabled` | 否 | 是否允许该节点接入，默认 `true` |
| `token` | 是 | 节点认证 Token |
| `public_ipv4` | 否 | 节点公网 IPv4，用于自动 DNS 记录 |
| `public_ipv6` | 否 | 节点公网 IPv6，用于自动 DNS 记录 |

`controller.nodes[].id` 不能重复。

`controller.nodes[].token` 也不能重复，并且不能复用 `controller.access_tokens[].token` 中的值。

## 最小配置（单机部署）

```yaml
controller:
  listen_addr: ":7001"
  controller_addr: "http://controller:7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  access_tokens:
    - name: "admin"
      token: "your-token"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-token"
```

如需配置 Agent，请继续参考 [Agent 配置](./agent)。
