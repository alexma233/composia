# DNS 配置

本文档介绍如何在 Composia 中配置服务侧 DNS。

## Controller 配置

```yaml
controller:
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"
```

创建 API Token 文件：

```bash
echo "your-cloudflare-api-token" > ./cloudflare-token.txt
```

**Cloudflare Token 权限要求：**
- Zone:Read
- DNS:Edit

平台侧字段说明见 [配置指南中的 DNS 配置](./configuration/dns)。

## 服务 DNS 配置

在服务的 `composia-meta.yaml` 中配置：

```yaml
name: my-app
nodes:
  - main

network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A        # A, AAAA, or CNAME
    proxied: true         # 启用 Cloudflare 代理
    ttl: 120              # TTL 秒数
    # value: "1.2.3.4"    # 可选，手动指定记录值
```

## 自动推导 IP

如果不指定 `value`，Composia 会尝试从节点配置自动推导：

```yaml
controller:
  nodes:
    - id: "main"
      public_ipv4: "203.0.113.10"    # 用于 A 记录
      public_ipv6: "2001:db8::1"      # 用于 AAAA 记录
```

**注意：** 自动推导仅适合单节点服务。多节点服务建议显式指定 `value`。

## 触发 DNS 更新

DNS 更新目前适用于以下场景：
- 迁移服务到新节点
- 手动执行 `dns_update`

手动触发时，请调用 ConnectRPC 方法 `composia.controller.v1.ServiceCommandService/RunServiceAction`，并传入 `SERVICE_ACTION_DNS_UPDATE`。

如果直接走 HTTP，请使用 `/api/controller/composia.controller.v1.ServiceCommandService/RunServiceAction`。

## DNS 配置示例

**基础 A 记录：**

```yaml
network:
  dns:
    provider: cloudflare
    hostname: api.example.com
    record_type: A
```

**启用 Cloudflare 代理：**

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    proxied: true
    ttl: 1    # 自动模式下 TTL 自动管理
```

**IPv6 支持：**

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: AAAA
```

**多域名：**

需要为每个域名配置独立的服务或使用通配符。

## 故障排查

### DNS 未更新

检查：
1. Controller 是否配置了 `dns.cloudflare`
2. Cloudflare API Token 是否有效
3. 域名 Zone 是否正确

## 相关文档

- [服务定义](./service-definition) —— 服务配置完整说明
- [部署管理](./deployment) —— 服务部署流程
- [Caddy 配置](./caddy) —— Caddy 反向代理配置
