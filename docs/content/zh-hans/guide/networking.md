# 网络配置

本文档介绍如何在 Composia 中配置 DNS 和 Caddy 反向代理。

## DNS 配置

Composia 支持自动 DNS 记录管理，当前仅支持 Cloudflare。

### Controller 配置

```yaml
controller:
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"
```

创建 API Token 文件：

```bash
echo "your-cloudflare-api-token" > configs/cloudflare-token.txt
```

**Cloudflare Token 权限要求：**
- Zone:Read
- DNS:Edit

### 服务 DNS 配置

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

### 自动推导 IP

如果不指定 `value`，Composia 会尝试从节点配置自动推导：

```yaml
controller:
  nodes:
    - id: "main"
      public_ipv4: "203.0.113.10"    # 用于 A 记录
      public_ipv6: "2001:db8::1"      # 用于 AAAA 记录
```

**注意：** 自动推导仅适合单节点服务。多节点服务建议显式指定 `value`。

### 触发 DNS 更新

DNS 更新目前适用于以下场景：
- 迁移服务到新节点
- 手动执行 `dns_update`

手动触发时，请调用 ConnectRPC 方法 `composia.controller.v1.ServiceCommandService/RunServiceAction`，并传入 `SERVICE_ACTION_DNS_UPDATE`。

### DNS 配置示例

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

## Caddy 反向代理

Composia 支持自动生成和同步 Caddy 配置片段。

### 架构

```
Service (composia-meta.yaml)
    │ network.caddy.enabled: true
    ▼
Controller (生成配置片段)
    │
    ▼
Agent (分发到各节点)
    │ 写入 generated_dir
    ▼
Caddy (加载配置并 reload)
```

### 1. 部署 Caddy 基础设施服务

创建一个 Caddy 基础设施服务：

```yaml
# infra-caddy/composia-meta.yaml
name: infra-caddy
nodes:
  - main
enabled: true

infra:
  caddy:
    compose_service: caddy      # Compose 服务名
    config_dir: /etc/caddy      # Caddy 配置目录
```

```yaml
# infra-caddy/docker-compose.yaml
services:
  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config
      - /srv/caddy/generated:/etc/caddy/conf.d  # 生成的配置目录
    command: caddy run --config /etc/caddy/Caddyfile --adapter caddyfile

volumes:
  caddy_data:
  caddy_config:
```

```caddy
# infra-caddy/Caddyfile
# 导入生成的配置
import /etc/caddy/conf.d/*.conf

# 可选：默认响应
:80 {
    respond "Caddy is running"
}
```

### 2. 配置 Agent

```yaml
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-agent-token"
  caddy:
    generated_dir: "/srv/caddy/generated"  # 必须与 Caddy 容器挂载路径一致
```

### 3. 配置业务服务

在需要被代理的服务中添加配置：

```yaml
# my-app/composia-meta.yaml
name: my-app
nodes:
  - main

network:
  caddy:
    enabled: true
    source: ./Caddyfile.fragment
```

创建 Caddy 配置片段：

```caddy
# my-app/Caddyfile.fragment
app.example.com {
    reverse_proxy localhost:8080
    
    # 安全头
    header {
        X-Frame-Options "SAMEORIGIN"
        X-Content-Type-Options "nosniff"
        X-XSS-Protection "1; mode=block"
        Referrer-Policy "strict-origin-when-cross-origin"
    }
    
    # Gzip 压缩
    encode gzip
    
    # 日志
    log {
        output file /var/log/caddy/app.log
        format json
    }
    
    # TLS（自动申请 Let's Encrypt）
    tls {
        protocols tls1.2 tls1.3
    }
}
```

### 4. 自动化行为

Caddy 配置会在以下情况自动同步：

| 操作 | 自动行为 |
|------|----------|
| `deploy` | 成功后触发 `caddy_sync` + `caddy_reload` |
| `update` | 成功后触发 `caddy_sync` + `caddy_reload` |
| `stop` | 删除配置片段并触发 `caddy_reload` |
| `migrate` | 源节点删除配置，目标节点添加配置 |

### Caddy 配置片段模板

**基础反向代理：**

```caddy
app.example.com {
    reverse_proxy localhost:3000
}
```

**带负载均衡（多实例）：**

```caddy
app.example.com {
    reverse_proxy localhost:3000 localhost:3001 localhost:3002 {
        lb_policy round_robin
        health_uri /health
        health_interval 10s
    }
}
```

**带基本认证：**

```caddy
app.example.com {
    basicauth {
        admin $2a$14$Zkx19XLiW6VYouLHR5NmfOFU0z2GTNmpkT/5qqR7hx4IjWJPDhjvG
    }
    reverse_proxy localhost:3000
}
```

**WebSocket 支持：**

```caddy
app.example.com {
    reverse_proxy localhost:3000 {
        header_up Upgrade {>Upgrade}
        header_up Connection {>Connection}
    }
}
```

**速率限制：**

```caddy
app.example.com {
    rate_limit {
        zone static_example {
            key static
            events 100
            window 1m
        }
    }
    reverse_proxy localhost:3000
}
```

## 完整示例

### 部署一个完整的 Web 应用

**目录结构：**

```
my-webapp/
├── composia-meta.yaml
├── docker-compose.yaml
└── Caddyfile.fragment
```

**composia-meta.yaml：**

```yaml
name: my-webapp
nodes:
  - main

network:
  caddy:
    enabled: true
    source: ./Caddyfile.fragment
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    proxied: true

data_protect:
  data:
    - name: uploads
      backup:
        strategy: files.copy
        include:
          - ./data/uploads
      restore:
        strategy: files.copy
        include:
          - ./data/uploads

backup:
  data:
    - name: uploads
      provider: rustic
```

**docker-compose.yaml：**

```yaml
services:
  app:
    image: myapp:1.0.0
    ports:
      - "127.0.0.1:8080:8080"  # 仅本地监听，通过 Caddy 暴露
    volumes:
      - ./data/uploads:/app/uploads
    environment:
      - NODE_ENV=production
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

**Caddyfile.fragment：**

```caddy
app.example.com {
    reverse_proxy localhost:8080
    
    header {
        X-Frame-Options "SAMEORIGIN"
        X-Content-Type-Options "nosniff"
    }
    
    encode gzip
    
    log {
        output file /var/log/caddy/my-webapp.log
    }
}
```

**部署步骤：**

1. 确保 Caddy 基础设施服务已部署
2. 将 `my-webapp` 目录提交到 Git 仓库
3. 在 Web UI 中找到 `my-webapp` 服务
4. 点击「部署」
5. 如有需要，在部署后手动执行 `dns_update`；Caddy 文件同步会通过对应的节点维护步骤完成
6. 访问 `https://app.example.com`

## 故障排查

### DNS 未更新

检查：
1. Controller 是否配置了 `dns.cloudflare`
2. Cloudflare API Token 是否有效
3. 域名 Zone 是否正确

### Caddy 配置未生效

检查：
1. Caddy 基础设施服务是否运行
2. Agent 配置的 `caddy.generated_dir` 是否正确
3. Caddy 容器是否正确挂载了生成目录
4. 查看 Caddy 日志：`docker logs infra-caddy-caddy-1`

### HTTPS 证书问题

- 确保证书目录已持久化（`caddy_data` 卷）
- 检查域名 DNS 是否正确指向服务器
- 查看 Caddy 日志了解证书申请状态

## 相关文档

- [服务定义](./service-definition) —— 服务配置完整说明
- [部署管理](./deployment) —— 服务部署流程
- [Caddy 官方文档](https://caddyserver.com/docs/) —— Caddy 配置参考
