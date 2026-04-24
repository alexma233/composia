# Caddy 配置

本文档介绍如何在 Composia 中配置 Caddy 反向代理。

## 架构

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

## 1. 部署 Caddy 基础设施服务

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
      - /data/state-agent/caddy/generated:/etc/caddy/conf.d  # 生成的配置目录
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

## 2. 配置 Agent

```yaml
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-agent-token"
  caddy:
    generated_dir: "/data/state-agent/caddy/generated"  # 必须与 Caddy 容器挂载源路径一致
```

Agent 侧字段说明见 [配置指南中的 Agent 配置](./configuration/agent)。

不要把 `generated_dir` 放在 Caddy 服务目录内，例如 `repo_dir/caddy/...`。服务部署会用最新 bundle 整体替换服务目录，放在其中的生成配置会在重新部署 Caddy 时被删除。推荐使用 Agent 的 `state_dir` 下的持久目录，并把同一个宿主机目录挂载到 Caddy 容器内。

## 3. 配置业务服务

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

## 4. 自动化行为

Caddy 配置会在以下情况自动同步：

| 操作 | 自动行为 |
|------|----------|
| `deploy` | 成功后触发 `caddy_sync` + `caddy_reload` |
| `update` | 成功后触发 `caddy_sync` + `caddy_reload` |
| `stop` | 删除配置片段并触发 `caddy_reload` |
| `migrate` | 源节点删除配置，目标节点添加配置 |

## Caddy 配置片段模板

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

### Caddy 配置未生效

检查：
1. Caddy 基础设施服务是否运行
2. Agent 配置的 `caddy.generated_dir` 是否正确
3. Caddy 容器是否正确挂载了生成目录
4. 通过你自己的容器运行环境查看 Caddy 容器日志

### HTTPS 证书问题

- 确保证书目录已持久化（`caddy_data` 卷）
- 检查域名 DNS 是否正确指向服务器
- 查看 Caddy 日志了解证书申请状态

## 相关文档

- [服务定义](./service-definition) —— 服务配置完整说明
- [部署管理](./deployment) —— 服务部署流程
- [DNS 配置](./dns) —— 服务侧 DNS 配置
- [Caddy 官方文档](https://caddyserver.com/docs/) —— Caddy 配置参考
