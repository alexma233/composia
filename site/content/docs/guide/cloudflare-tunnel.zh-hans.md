---
title: "Cloudflare Tunnel"
date: '2026-05-31T00:00:00+08:00'
weight: 25
---

Composia 可以为声明了 `network.cloudflare_tunnel` 的服务管理远程配置的 Cloudflare Tunnel 入口规则。隧道同步作为控制器端任务运行，因为 Cloudflare 的远程隧道配置是全局状态。

## 工作原理

当服务被部署、更新、停止或手动同步时，控制器会创建一个 `cloudflare_tunnel_sync` 任务。控制器工作器执行以下步骤：

1. 读取任务仓库版本中的所有服务元数据。
2. 从声明了 `network.cloudflare_tunnel` 的服务构建隧道入口规则。
3. 通过 `PUT /accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations` 将完整的入口列表发送到 Cloudflare。
4. 确保每个主机名都有一个指向 `{tunnel_id}.cfargotunnel.com` 的代理 CNAME 记录。

Cloudflare 要求一个通配入口规则。Composia 默认追加 `http_status:404`。

## 控制器配置

隧道 ID 和 Cloudflare 凭证应放在控制器配置中，而不是服务元数据中：

```yaml
controller:
  cloudflare_tunnel:
    account_id: "REPLACE"
    api_token_file: /run/secrets/cloudflare-api-token
    tunnels:
      edge:
        tunnel_id: "c1744f8b-faa1-48a4-9e5c-02ac921467fa"
        fallback_service: http_status:404
    nodes:
      main:
        tunnel: edge
```

| 键 | 类型 | 必需 | 描述 |
|-----|------|----------|-------------|
| `account_id` | `string` | 是 | Cloudflare 账户 ID。 |
| `api_token` / `api_token_file` | `string` | 是 | 具有 Cloudflare Tunnel 配置和 DNS 写入权限的 API 令牌。 |
| `tunnels` | `map` | 是 | 映射到 Cloudflare 隧道 ID 的隧道别名。 |
| `nodes` | `map` | 否 | 当服务未指定 `tunnel` 时使用的默认节点到隧道映射。 |

`tunnel_id` 不是连接器密钥，但它仍然是控制器级的基础设施元数据。Cloudflared 连接器令牌或凭证应保留在 `cloudflared` 服务使用的节点/代理密钥中。

## 服务声明

在服务的 `composia-meta.yaml` 中声明隧道入口：

```yaml
network:
  cloudflare_tunnel:
    hostname: app.example.com
    service: http://app:8080
    origin_request:
      no_tls_verify: false
      http_host_header: app.internal
```

| 键 | 类型 | 必需 | 描述 |
|-----|------|----------|-------------|
| `hostname` | `string` | 是 | 由 Cloudflare Tunnel 路由的公共主机名。 |
| `service` | `string` | 是 | `cloudflared` 使用的源 URL，例如 `http://app:8080`。 |
| `tunnel` | `string` | 否 | 隧道别名。省略时，Composia 从目标节点映射中推导。 |
| `path` | `string` | 否 | 入口规则的可选路径匹配器。 |
| `origin_request` | `object` | 否 | Cloudflare 源参数。初始支持包括 `no_tls_verify`、`http_host_header`、`origin_server_name`、`connect_timeout` 和 `tls_timeout`。 |

## 隧道选择

Composia 按以下规则为每个服务解析隧道：

1. 如果设置了 `network.cloudflare_tunnel.tunnel`，则使用该别名。
2. 如果服务只针对一个节点，Composia 使用 `controller.cloudflare_tunnel.nodes.<node>.tunnel`。
3. 如果服务针对多个节点且所有节点映射到同一个隧道，则使用该隧道。
4. 如果目标节点映射到不同的隧道，服务必须显式设置 `network.cloudflare_tunnel.tunnel`。

## 停止行为

当已停止的服务声明了 `network.cloudflare_tunnel` 时，后续的隧道同步会排除该服务并删除其 CNAME 记录。之后的同步只包含有运行实例的服务，因此重新部署服务会将其重新加入。

## 手动同步

使用 CLI 同步服务的隧道配置：

```bash
composia service my-app tunnel-sync
```

这会将完整的已配置隧道状态同步到 Cloudflare，而不仅仅是选定的服务，因为 Cloudflare 的远程隧道配置是作为单个文档更新的。
