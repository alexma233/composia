---
title: "DNS"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Composia 为声明了 `network.dns` 的服务管理 DNS 记录。DNS 更新作为控制器端的任务运行。

## 工作原理

当服务部署或手动触发 DNS 更新时，控制器会创建一个 `dns_update` 任务。控制器工作线程执行它：

1. 读取任务中记录的仓库版本下的服务元数据。
2. 从 `network.dns` 构建期望的 DNS 记录。
3. 将记录同步到 DNS 提供商。

## 提供商配置

在控制器配置中配置至少一个 DNS 提供商。提供商凭据和区域列表是全局的：

```yaml
controller:
  dns:
    cloudflare:
      api_token: "REPLACE"
      zones:
        - "example.com"
        - "other.com"
```

支持五种提供商。每种都有自己的凭据键，并且都共享一个列出托管域区域的 `zones` 字段：

| 提供商 | 键前缀 | 凭据键 |
|----------|-----------|-----------------|
| `cloudflare` | `dns.cloudflare` | `api_token`、`api_token_file` |
| `alidns` | `dns.alidns` | `access_key_id`、`access_key_secret`、`region_id`，可选 `security_token` |
| `dnspod` | `dns.dnspod` | `secret_id`、`secret_key`、`region`，可选 `session_token` |
| `route53` | `dns.route53` | `access_key_id`、`secret_access_key`、`region`，可选 `session_token`、`profile`、`hosted_zone_id` |
| `huaweicloud` | `dns.huaweicloud` | `access_key_id`、`secret_access_key`、`region_id` |

每个凭据字段都有对应的 `_file` 变体，用于从文件中读取（例如 `api_token_file`）。

## 服务 DNS 声明

在服务的 `composia-meta.yaml` 中声明 DNS 设置：

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    value: 203.0.113.10
    proxied: true
    ttl: 120
    comment: "由 Composia 管理"
```

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `provider` | `string` | 是 | `cloudflare`、`alidns`、`dnspod`、`route53` 或 `huaweicloud`。 |
| `hostname` | `string` | 是 | DNS 主机名。区域从已配置的区域列表中匹配。 |
| `record_type` | `string` | 否 | `A`、`AAAA` 或 `CNAME`。为空时，根据值或节点地址推断。 |
| `value` | `string` | 否 | 显式的 DNS 记录值。为空时，Composia 从目标节点派生值。 |
| `proxied` | `bool` | 否 | 启用 Cloudflare 代理。仅 Cloudflare 支持。 |
| `ttl` | `uint32` | 否 | DNS TTL，单位为秒。 |
| `comment` | `string` | 否 | DNS 记录备注。仅 Cloudflare 支持。 |

## 记录解析

### 使用显式值

当设置了 `value` 时，Composia 直接使用它。如果是 IP 地址，记录类型会被推断：IPv4 为 `A`，IPv6 为 `AAAA`。如果是主机名，记录类型必须为 `CNAME`（或留空，同样解析为 `CNAME`）。

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    value: 203.0.113.10
```

### 从节点地址解析

当 `value` 为空时，Composia 使用控制器配置中目标节点的 `public_ipv4` 和 `public_ipv6`：

```yaml
controller:
  nodes:
    - id: "main"
      public_ipv4: "203.0.113.10"
      public_ipv6: "2001:db8::10"
```

当 `record_type` 为空时，如果节点同时拥有两种地址，则会创建 A 和 AAAA 记录。如果 `record_type` 为 `A`，则仅使用 IPv4 地址。如果 `record_type` 为 `AAAA`，则仅使用 IPv6 地址。

目标多个节点的服务必须显式设置 `value`。在多目标节点且 `value` 为空的情况下会产生错误。

## 触发 DNS 更新

DNS 记录在部署任务流程中创建或更新。您也可以通过 Web UI 或 CLI 触发独立的 DNS 更新：

```bash
composia service dns-update my-app
```

这会创建一个 `dns_update` 任务。任务日志显示区域解析、记录操作和最终结果。

## Cloudflare 选项

当提供商为 `cloudflare` 时，`proxied` 和 `comment` 在记录创建后生效。Composia 调用 Cloudflare API 为每个 DNS 记录设置代理状态和备注。

非 Cloudflare 提供商不支持这些选项。在其他提供商上设置 `proxied` 或 `comment` 会导致 DNS 更新失败。

## 区域匹配

Composia 将服务主机名与已配置的区域进行匹配。区域按从最长到最短的顺序尝试匹配。例如，对于 `zones: ["example.com.", "sub.example.com."]`，主机名 `app.sub.example.com` 会首先匹配 `sub.example.com.`。

如果没有区域匹配到主机名，DNS 更新将失败。

## 过期记录清理

DNS 同步管理每个主机名的恰好三种记录类型：A、AAAA 和 CNAME。在设置新记录之前，任何不在期望状态中的已配置记录类型都会被删除。例如，如果某个服务之前使用 `record_type: A`，后来改为 `record_type: CNAME`，则旧的 A 记录会被删除，新的 CNAME 记录会被创建。

更改服务的主机名不会清理旧主机名的记录。如果您将 `app.example.com` 重命名为 `api.example.com`，`app.example.com` 的记录会保留在 DNS 提供商中，直到您手动删除。
