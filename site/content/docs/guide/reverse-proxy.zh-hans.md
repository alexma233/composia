---
title: "反向代理"
date: '2026-05-26T00:00:00+08:00'
weight: 30
---

Composia 与 Caddy 集成以实现反向代理管理。Caddy 基础设施服务作为普通的 Docker Compose 服务运行，Composia 在部署和停止时同步 Caddy 配置文件。

## 架构

```
Controller repo
  ├── caddy/
  │   ├── docker-compose.yaml   （Caddy Compose 服务）
  │   ├── Caddyfile             （主 Caddy 配置，导入生成的文件）
  │   └── composia-meta.yaml    （声明 infra.caddy）
  ├── my-app/
  │   ├── docker-compose.yaml
  │   ├── Caddyfile             （服务专属的 Caddy 配置）
  │   └── composia-meta.yaml    （声明 network.caddy）
  └── ...
```

在部署时，Composia 将每个服务的 Caddyfile 复制到生成的目录中，然后触发 Caddy 重载。

## 基础设施设置

在仓库中声明恰好一个 Caddy 基础设施服务：

```yaml {filename="caddy/composia-meta.yaml"}
name: caddy
nodes:
  - main
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
```

Caddy 服务目录中的主 Caddyfile 应导入生成的文件：

```caddy {filename="caddy/Caddyfile"}
import /etc/caddy/generated/*.caddy
```

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `compose_service` | `string` | Compose 服务名称。默认为 `caddy`。 |
| `config_dir` | `string` | 容器内的 Caddy 配置目录。默认为 `/etc/caddy`。 |

仓库中只能有一个服务被声明为 Caddy 基础设施。

## 服务配置

对于需要反向代理条目的每个服务，在 `composia-meta.yaml` 中启用 Caddy 并提供 Caddyfile：

```yaml {filename="my-app/composia-meta.yaml"}
name: my-app
nodes:
  - main
network:
  caddy:
    enabled: true
    source: Caddyfile
```

`source` 路径相对于服务目录，且必须保持在目录内。文件可以任意命名，但惯例是 `Caddyfile`。

```caddy {filename="my-app/Caddyfile"}
app.example.com {
    reverse_proxy app:8080
}
```

## 同步工作原理

在部署或更新任务期间，agent 在 `compose up` 后运行 Caddy 同步步骤：

1. 从服务的 `composia-meta.yaml` 读取 `network.caddy.source`。
2. 将源文件复制到 `<agent_state_dir>/caddy/generated/<service_dir>.caddy`。
3. 运行 `docker compose exec <caddy_service> caddy reload --config <Caddyfile> --adapter caddyfile`。

生成的文件名从服务目录名派生。对于 `my-app`，文件名为 `my-app.caddy`。

在停止任务期间，生成的 Caddy 文件会被删除。

## Caddy 同步任务

独立的 `caddy_sync` 任务在不部署服务的情况下重建 Caddy 配置。它支持两种模式：

**完全重建** (`full_rebuild: true`): 删除生成目录中所有 `.caddy` 文件，然后重新同步所有受 Caddy 管理的服务。

**定向同步**: 仅同步指定的服务目录。

通过 Web UI 或 CLI 触发：

```bash
composia service caddy-sync my-app
```

## Caddy 重载任务

`caddy_reload` 任务在 Caddy 容器内运行 `caddy reload`，而不更改任何文件。在手动编辑主 Caddyfile 后使用：

```bash
composia node reload-caddy main
```

## Agent 配置

Agent 配置有一个可选的 Caddy 部分：

```yaml
agent:
  caddy:
    generated_dir: "/data/state-agent/caddy/generated"
```

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `generated_dir` | `string` | 生成的 Caddy 配置目录。默认为 `<state_dir>/caddy/generated`。 |

生成的目录必须位于 Caddy 容器可读取的路径内。Caddy compose 服务必须有一个卷将此目录挂载到主 Caddyfile 中导入的路径。
