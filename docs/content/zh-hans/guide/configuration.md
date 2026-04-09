# 配置指南

本文档介绍 Composia 的两类配置：平台配置和服务配置。

配置加载是严格模式，启动时会拒绝未知字段。

## 配置分类

| 配置类型 | 文件 | 作用范围 | 说明 |
|----------|------|----------|------|
| 平台配置 | `configs/config.compose.yaml` | 整个平台 | 定义 Controller 和 Agent 如何启动 |
| 服务配置 | `composia-meta.yaml` | 单个服务 | 定义服务部署目标和功能特性 |

## 平台配置

### 完整配置示例

```yaml
controller:
  # 网络配置
  listen_addr: ":7001"
  controller_addr: "http://controller:7001"
  
  # 目录配置
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  
  # 认证配置
  cli_tokens:
    - name: "compose-admin"
      token: "replace-this-token"
      enabled: true
  
  # 节点配置
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-agent-token"
      public_ipv4: "203.0.113.10"
    - id: "edge"
      display_name: "Edge"
      enabled: true
      token: "edge-agent-token"
  
  # Git 同步配置（可选）
  git:
    remote_url: "https://git.example.com/infra/composia.git"
    branch: "main"
    pull_interval: "30s"
    author_name: "Composia"
    author_email: "composia@example.com"
    auth:
      token_file: "/app/configs/git-token.txt"
  
  # DNS 配置（可选）
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"
  
  # 备份配置（可选）
  rustic:
    main_nodes:
      - "main"
  
  # Secrets 配置（可选）
  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
    recipient_file: "/app/configs/age-recipients.txt"
    armor: true

agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-agent-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
  caddy:
    generated_dir: "/srv/caddy/generated"
```

### Controller 配置项

#### 基础配置

| 配置项 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `listen_addr` | string | 是 | Controller 监听地址，如 `:7001` |
| `controller_addr` | string | 是 | Agent 和 Web UI 访问 Controller 的地址 |
| `repo_dir` | string | 是 | Git 工作树目录，保存服务定义 |
| `state_dir` | string | 是 | SQLite 和运行时状态目录 |
| `log_dir` | string | 是 | 任务日志持久化目录 |
| `nodes` | array | 是 | 顶层字段必须出现，即使为空数组也要写出 |

#### 认证配置

```yaml
cli_tokens:
  - name: "admin"
    token: "your-secure-token-here"
    enabled: true
  - name: "readonly"
    token: "readonly-token"
    enabled: true
```

| 字段 | 说明 |
|------|------|
| `name` | 必填的 Token 名称，用于识别 |
| `token` | 必填的 Token 值，Web UI 和 CLI 使用 |
| `enabled` | 是否启用该 Token |
| `comment` | 可选的运维备注 |

**安全建议：**
- 使用强随机字符串作为 Token
- 生产环境使用不同的 Token
- 定期轮换 Token

#### 节点配置

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

#### Git 同步配置（可选）

```yaml
git:
  remote_url: "https://github.com/example/composia-services.git"
  branch: "main"
  pull_interval: "30s"
  author_name: "Composia"
  author_email: "composia@example.com"
  auth:
    token_file: "/app/configs/git-token.txt"
```

| 字段 | 说明 |
|------|------|
| `remote_url` | 远端 Git 仓库地址 |
| `branch` | 跟踪的分支；未填写时沿用当前本地分支 |
| `pull_interval` | 自动拉取间隔，如 `30s`, `5m`；设置 `remote_url` 后必填 |
| `author_name` | Git 提交者名称 |
| `author_email` | Git 提交者邮箱 |
| `auth.token_file` | 访问令牌文件路径 |

#### DNS 配置（可选）

```yaml
dns:
  cloudflare:
    api_token_file: "/app/configs/cloudflare-token.txt"
```

#### Secrets 配置（可选）

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
  recipient_file: "/app/configs/age-recipients.txt"
  armor: true
```

| 字段 | 说明 |
|------|------|
| `provider` | 加密提供方，当前仅支持 `age` |
| `identity_file` | age 私钥文件路径 |
| `recipient_file` | age 公钥文件路径 |
| `armor` | 是否使用 ASCII Armor 格式 |

如果配置了 `secrets` 段，则 `provider`、`identity_file` 和 `recipient_file` 都是必填项，且 `provider` 必须是 `age`。

### Agent 配置项

| 配置项 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `controller_addr` | string | 是 | Controller API 地址 |
| `node_id` | string | 是 | 节点 ID，必须匹配 Controller 配置 |
| `token` | string | 是 | 节点认证 Token |
| `repo_dir` | string | 是 | 本地服务 bundle 目录 |
| `state_dir` | string | 是 | 本地运行状态目录 |
| `caddy.generated_dir` | string | 否 | Caddy 配置片段输出目录 |

如果同一个文件同时包含 `controller` 和 `agent`，还需要满足以下约束：

- `agent.node_id` 必须是 `main`
- `controller.nodes` 必须包含 `main`
- `controller.repo_dir` 和 `agent.repo_dir` 不能相同

## 配置建议

### 最小配置（单机部署）

```yaml
controller:
  listen_addr: ":7001"
  controller_addr: "http://controller:7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  cli_tokens:
    - name: "admin"
      token: "your-token"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-token"

agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
```

### 启用 Caddy

在 Agent 配置中添加：

```yaml
agent:
  # ... 其他配置
  caddy:
    generated_dir: "/srv/caddy/generated"
```

同时需要部署 Caddy 基础设施服务，参考 [网络配置](./networking)。

### 启用备份

Controller 配置：

```yaml
controller:
  # ... 其他配置
  rustic:
    main_nodes:
      - "main"
```

同时需要部署 rustic 基础设施服务，参考 [备份与迁移](./backup-migrate)。

`rustic.main_nodes` 中的每个节点 ID 都必须引用已存在的 `controller.nodes[].id`。

### 启用 DNS

Controller 配置：

```yaml
controller:
  # ... 其他配置
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"
```

参考 [网络配置](./networking) 了解服务侧 DNS 配置。

### 启用 Git 远端同步

Controller 配置：

```yaml
controller:
  # ... 其他配置
  git:
    remote_url: "https://github.com/example/composia-services.git"
    branch: "main"
    pull_interval: "30s"
    auth:
      token_file: "/app/configs/git-token.txt"
```

## 配置文件安全

### Token 管理

1. **对配置文件使用只读挂载**

```yaml
# docker-compose.yaml
volumes:
  - ./configs:/app/configs:ro
```

### age 密钥管理

```bash
# 生成 age 密钥对
age-keygen -o key.txt

# 提取公钥
cat key.txt | grep "public key" > recipients.txt

# 挂载到容器
# key.txt 作为 identity_file（私钥）
# recipients.txt 作为 recipient_file（公钥）
```

## 验证配置

如果是在本地直接跑源码，建议使用开发配置进行验证：

```bash
# 使用 dev 配置启动 Controller
go run ./cmd/composia controller -config ./configs/config.controller.dev.yaml

# 使用共享的 dev 配置启动 Agent
go run ./cmd/composia agent -config ./configs/config.controller.dev.yaml
```

`configs/config.compose.yaml` 主要用于仓库自带的 `docker-compose.yaml` 容器栈，不适合作为宿主机本地开发配置直接运行。
