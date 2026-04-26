# 服务定义

服务定义是 Composia 的核心，本文档介绍如何创建和配置服务。

## 服务目录结构

### 最小结构

一个最基础的服务至少包含两个文件：

```
my-service/
├── composia-meta.yaml    # 服务元数据
└── docker-compose.yaml   # Docker Compose 配置
```

### 完整结构

一个功能完整的服务目录可能包含：

```
my-service/
├── composia-meta.yaml      # 服务元数据（必需）
├── docker-compose.yaml     # Compose 配置（必需）
├── .env                    # 环境变量（可选）
├── Caddyfile.fragment      # Caddy 配置片段（可选）
├── secrets/                # 加密密钥（可选）
│   └── database.env.age
└── data/                   # 数据目录（可选）
    └── uploads/
```

## composia-meta.yaml

### 完整示例

```yaml
# 基础信息
name: my-app               # 服务唯一名称（必需）
project_name: my-app-prod # Compose 项目名称（可选）
enabled: true              # 是否启用（可选，默认 true）

# 部署目标
nodes:
  - main
  - edge

# 网络配置
network:
  # Caddy 反向代理
  caddy:
    enabled: true
    source: ./Caddyfile.fragment
  
  # DNS 配置
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    proxied: true
    ttl: 120

# 数据保护
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
    
    - name: database
      backup:
        strategy: database.pgdumpall
        service: postgres

# 备份配置
backup:
  data:
    - name: uploads
      provider: rustic
    - name: database
      provider: rustic

# 迁移配置
migrate:
  data:
    - name: uploads
    - name: database

# 基础设施声明（仅用于基础设施服务）
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
  rustic:
    compose_service: rustic
    profile: default
    data_protect_dir: /data-protect
    init_args:
      - --set-chunker
      - rabin
```

### 字段说明

#### 基础信息

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 服务唯一标识符，用于 URL 和内部引用 |
| `project_name` | string | 否 | 覆盖 Docker Compose 项目名 |
| `enabled` | boolean | 否 | 是否启用服务声明，默认 `true` |

Composia 会以严格模式校验 `composia-meta.yaml`。未知字段不会被忽略，而是会直接报错。

#### 部署目标

| 字段 | 类型 | 说明 |
|------|------|------|
| `nodes` | array | 目标节点列表，每个元素是节点 ID |

**示例：**

```yaml
# 单节点部署
nodes:
  - main

# 多节点部署
nodes:
  - main
  - edge-1
  - edge-2
```

#### 网络配置

**Caddy 配置 (`network.caddy`)：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `enabled` | boolean | 是否启用 Caddy 反向代理 |
| `source` | string | Caddyfile 片段路径，相对于服务目录 |

**DNS 配置 (`network.dns`)：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `provider` | string | 是 | DNS 提供商，当前仅支持 `cloudflare` |
| `hostname` | string | 是 | 域名，如 `app.example.com` |
| `record_type` | string | 否 | 记录类型：`A`, `AAAA`, `CNAME`，默认 `A` |
| `value` | string | 否 | 记录值，为空时自动从节点 IP 推导 |
| `proxied` | boolean | 否 | 是否启用 Cloudflare 代理，默认 `false` |
| `ttl` | number | 否 | TTL 秒数，默认 `120` |

#### 数据保护

`data_protect` 定义可备份和可恢复的数据项：

```yaml
data_protect:
  data:
    - name: uploads                    # 数据项名称
      backup:                          # 备份策略
        strategy: files.copy           # 备份策略类型
        include:                       # 包含路径
          - ./data/uploads
        exclude:                       # 排除路径（可选）
          - ./data/uploads/temp
      restore:                         # 恢复策略
        strategy: files.copy_after_stop
        include:
          - ./data/uploads
    
    - name: database
      backup:
        strategy: database.pgdumpall   # PostgreSQL 全量备份
        service: postgres              # Compose 服务名
```

**备份策略：**

| 策略 | 说明 | 适用场景 |
|------|------|----------|
| `files.copy` | 直接复制文件 | 静态文件、上传目录 |
| `files.copy_after_stop` | 停止服务后复制并恢复 | 需要一致性的数据 |
| `database.pgdumpall` | PostgreSQL 全量导出 | PostgreSQL 数据库 |

恢复策略中，`files.copy` 会直接恢复；`files.copy_after_stop` 会先停止 Compose project，恢复文件或 Docker volume 后再启动。

#### 备份配置

`backup` 定义哪些数据项参与备份任务：

```yaml
backup:
  data:
    - name: uploads
      provider: rustic     # 备份提供方
    - name: database
      provider: rustic
```

#### 迁移配置

`migrate` 定义迁移时会带走哪些数据：

```yaml
migrate:
  data:
    - name: uploads
    - name: database
```

**注意：** 迁移的数据项必须在 `data_protect` 中同时定义 `backup` 和 `restore` 策略。

#### 基础设施声明

用于声明该服务是基础设施服务（如 Caddy、rustic）：

```yaml
infra:
  caddy:
    compose_service: caddy      # Compose 服务名
    config_dir: /etc/caddy      # Caddy 配置目录
  
  rustic:
    compose_service: rustic     # Compose 服务名
    profile: default            # rustic profile
    data_protect_dir: /data-protect  # rustic 容器内可读取的数据保护目录
    init_args:                  # Settings 中执行 rustic init 时追加的参数
      - --set-chunker
      - rabin
```

## docker-compose.yaml

服务目录中的 `docker-compose.yaml` 是标准的 Docker Compose 文件，Composia 完全兼容。

### 最小示例

```yaml
services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
    volumes:
      - ./html:/usr/share/nginx/html
```

### 带标签的示例（推荐）

```yaml
services:
  web:
    image: myapp:latest
    labels:
      - "composia.service=my-app"
      - "traefik.enable=true"
    environment:
      - NODE_ENV=production
    volumes:
      - data:/app/data
    networks:
      - backend

  db:
    image: postgres:15
    environment:
      POSTGRES_DB: myapp
      POSTGRES_USER: app
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    secrets:
      - db_password

volumes:
  data:
  postgres_data:

networks:
  backend:

secrets:
  db_password:
    file: ./secrets/db_password.txt
```

### 使用环境变量

```yaml
services:
  app:
    image: myapp:${APP_VERSION:-latest}
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - LOG_LEVEL=${LOG_LEVEL:-info}
```

`.env` 文件：

```
APP_VERSION=1.2.3
DATABASE_URL=postgresql://user:pass@db:5432/myapp
LOG_LEVEL=debug
```

## Caddyfile.fragment

当启用 Caddy 反向代理时，需要提供 Caddy 配置片段：

```caddy
app.example.com {
    reverse_proxy localhost:8080
    
    header {
        X-Frame-Options "SAMEORIGIN"
        X-Content-Type-Options "nosniff"
    }
    
    encode gzip
    
    log {
        output file /var/log/caddy/app.log
    }
}
```

**注意：** Caddy 片段不需要完整的 Caddyfile 结构，只需域名块即可。

## 服务模板

### Web 应用模板

```yaml
# composia-meta.yaml
name: web-app
nodes:
  - main
network:
  caddy:
    enabled: true
    source: ./Caddyfile.fragment
  dns:
    provider: cloudflare
    hostname: app.example.com
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
migrate:
  data:
    - name: uploads
```

```yaml
# docker-compose.yaml
services:
  app:
    image: myapp:latest
    volumes:
      - ./data/uploads:/app/uploads
    environment:
      - NODE_ENV=production
```

### 数据库服务模板

```yaml
# composia-meta.yaml
name: postgres-main
nodes:
  - main
data_protect:
  data:
    - name: database
      backup:
        strategy: database.pgdumpall
        service: postgres
      restore:
        strategy: files.copy
        include:
          - ./restore/
backup:
  data:
    - name: database
      provider: rustic
```

```yaml
# docker-compose.yaml
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: mydb
      POSTGRES_USER: dbuser
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    secrets:
      - db_password

volumes:
  postgres_data:

secrets:
  db_password:
    file: ./secrets/db_password.txt
```

## 相关文档

- [配置指南](./configuration) —— 平台配置说明
- [部署管理](./deployment) —— 如何部署服务
- [DNS 配置](./dns) —— DNS 详细配置
- [Caddy 配置](./caddy) —— Caddy 详细配置
- [备份与迁移](./backup-migrate) —— 数据保护配置
