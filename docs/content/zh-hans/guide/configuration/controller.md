# Controller 配置

本文档说明 `config/config.yaml` 中的 `controller` 段。

## 基础配置

| 配置项 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `listen_addr` | string | 是 | Controller 监听地址，如 `:7001` |
| `repo_dir` | string | 是 | Git 工作树目录，保存服务定义 |
| `state_dir` | string | 是 | SQLite 和运行时状态目录 |
| `log_dir` | string | 是 | 任务日志持久化目录 |
| `nodes` | array | 是 | 顶层字段必须出现，即使为空数组也要写出 |

## Controller 访问 token（`access_tokens`）

```yaml
access_tokens:
  - name: "admin"
    token_file: "/app/configs/controller-access-token.txt"
    enabled: true
  - name: "automation"
    token: "automation-token"
    enabled: true
```

| 字段 | 说明 |
|------|------|
| `name` | 必填的 Token 名称，用于识别 |
| `token` | 与 `token_file` 二选一；供 Web UI、CLI 或自定义客户端访问 Controller |
| `token_file` | 与 `token` 二选一；读取 Token 的文件路径 |
| `enabled` | 是否启用该 Token |
| `comment` | 可选的运维备注 |

Composia 当前没有 RBAC。所有已启用的访问 Token 都拥有完整的 Controller 访问权限，Token 名称不会影响权限。

解析后的 Token 值必须在 `controller.access_tokens[].token` 和 `controller.nodes[].token` 两处全局唯一。配置加载器会拒绝重复值，以及这两类 Token 之间的冲突。

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
    token_file: "/app/configs/main-agent-token.txt"
    public_ipv4: "203.0.113.10"
    public_ipv6: "2001:db8::1"
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `id` | 是 | 节点唯一标识，Agent 的 `node_id` 必须匹配 |
| `display_name` | 否 | 显示名称，用于 Web UI |
| `enabled` | 否 | 是否允许该节点接入，默认 `true` |
| `token` | 条件必填 | 与 `token_file` 二选一；节点认证 Token |
| `token_file` | 否 | 与 `token` 二选一；读取节点认证 Token 的文件路径 |
| `public_ipv4` | 否 | 节点公网 IPv4，用于自动 DNS 记录 |
| `public_ipv6` | 否 | 节点公网 IPv6，用于自动 DNS 记录 |

`controller.nodes[].id` 不能重复。

`controller.nodes[].token` 的解析值也不能重复，并且不能复用 `controller.access_tokens[].token` 的解析值。

## 更新配置

```yaml
controller:
  updates:
    default_check_schedule: "0 4 * * *"
    auto_apply: false
    backup_before_update: false
    digest_pin: true
    forge_auth:
      github:
        - url: https://github.com
          token_file: /run/secrets/github-token
        - url: https://github.example.com
          api_url: https://github.example.com/api/v3
          token_file: /run/secrets/github-enterprise-token
      gitlab:
        - url: https://gitlab.com
          token_file: /run/secrets/gitlab-token
      forgejo:
        - url: https://git.example.com
          api_url: https://git.example.com/api/v1
          token_file: /run/secrets/forgejo-token
    semver:
      default_allow:
        - patch
        - minor
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `default_check_schedule` | 否 | 镜像更新检查的默认 cron 调度，当服务级和镜像级均未指定时使用 |
| `auto_apply` | 否 | 控制器级默认值，是否自动应用检测到的镜像更新，默认 `false` |
| `backup_before_update` | 否 | 控制器级默认值，是否在镜像更新前执行备份，默认 `false` |
| `digest_pin` | 否 | 控制器级默认值，是否将标签固定到 `tag@sha256:digest`，默认 `true` |
| `forge_auth.<platform>` | 否 | 可选的单个 auth 对象或 auth 对象列表；公开 release 可以匿名查询 |
| `forge_auth.<platform>.url` | 否 | 用于匹配 `discovery.repo_url` 的网页 URL，例如 `https://github.com` |
| `forge_auth.<platform>.token` / `token_file` | 否 | controller 侧 release discovery 使用的 token |
| `forge_auth.<platform>.api_url` | 否 | 自托管实例的 API base URL；GitHub/GitLab 默认使用公开 API，Codeberg 可从 `repo_url` 自动推导 |
| `semver.default_allow` | 否 | 默认允许的 semver 升级类型（`patch`、`minor`、`major`）；未设置时默认允许 `patch, minor` |

各镜像和各服务的覆盖值优先于控制器级默认值。请参见[服务定义](../service-definition)中的 `update` 段落。

## 最小配置（单机部署）

```yaml
controller:
  listen_addr: ":7001"
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
