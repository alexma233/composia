# 配置指南

本文档现在只保留总览。平台配置已经按功能拆分成独立文档，便于按需查阅。

配置加载是严格模式，启动时会拒绝未知字段。

## 配置分类

| 配置类型 | 文件 | 作用范围 | 说明 |
|----------|------|----------|------|
| 平台配置 | `config/config.yaml` | 整个平台 | 定义 Controller 和 Agent 如何启动 |
| 服务配置 | `composia-meta.yaml` | 单个服务 | 定义服务部署目标和功能特性 |

服务配置请参考 [服务定义](./service-definition)。

## 平台配置总览

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
  access_tokens:
    - name: "compose-admin"
      token: "replace-this-token"
      enabled: true
      comment: "主管理员 token"

  # 节点配置
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-agent-token"
      public_ipv4: "203.0.113.10"
      public_ipv6: "2001:db8::10"
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
      username: "git"
      token_file: "/app/configs/git-token.txt"

  # DNS 配置（可选）
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"

  # 备份配置（可选）
  backup:
    default_schedule: "0 2 * * *"

  rustic:
    main_nodes:
      - "main"
    maintenance:
      forget_schedule: "15 3 * * *"
      prune_schedule: "45 3 * * *"

  # Secrets 配置（可选）
  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
    recipient_file: "/app/configs/age-recipients.txt"
    armor: true

agent:
  controller_addr: "http://controller:7001"
  controller_grpc: false
  node_id: "main"
  token: "main-agent-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
  caddy:
    generated_dir: "/srv/caddy/generated"
```

## 按功能阅读

- [Controller 配置](./configuration/controller) —— 基础字段、访问 token、节点配置和最小单机示例
- [Agent 配置](./configuration/agent) —— Agent 必填字段、和 Controller 同文件时的约束、Caddy 输出目录
- [Git 远端同步](./configuration/git-sync) —— `controller.git` 的字段说明与示例
- [DNS 配置](./configuration/dns) —— `controller.dns` 的字段说明与和服务侧 DNS 的关系
- [备份配置](./configuration/backup) —— `controller.backup`、`controller.rustic` 与服务侧备份定时覆盖规则
- [Secrets 配置](./configuration/secrets) —— `controller.secrets`、age 密钥文件和启用方式
- [配置安全](./configuration/security) —— token 与密钥文件的存放和挂载建议
- [配置验证](./configuration/verification) —— 本地源码启动时的验证方式

## 相关文档

- [服务定义](./service-definition) —— 服务侧 `composia-meta.yaml` 配置
- [DNS 配置](./dns) —— 服务侧 DNS 规则、自动推导和记录更新
- [Caddy 配置](./caddy) —— Caddy 基础设施、配置片段和自动同步
- [备份与迁移](./backup-migrate) —— rustic 基础设施与数据保护流程
- [快速开始](./quick-start) —— 容器方式快速启动
