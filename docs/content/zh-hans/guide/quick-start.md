# 快速开始

本指南将帮助你在几分钟内使用预构建的容器镜像启动并运行 Composia。

## 前提条件

- Docker Engine + Docker Compose v2

## 安装

### 1. 创建配置文件

为容器栈创建配置文件：

```bash
mkdir -p configs
cat > configs/config.compose.yaml << 'EOF'
controller:
  listen_addr: ":7001"
  controller_addr: "http://controller:7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  cli_tokens:
    - name: "compose-admin"
      token: "dev-admin-token"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-agent-token"
  rustic:
    main_nodes:
      - "main"
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
EOF
```

### 2. 启动服务栈

```bash
docker compose up -d
```

默认情况下，`docker-compose.yaml` 使用自建的 Forgejo registry。
如果你更希望使用 GHCR，请将 `docker-compose.yaml` 中的镜像地址替换为：

```yaml
ghcr.io/alexma233/composia:latest
ghcr.io/alexma233/composia-web:latest
```

这将拉取预构建的镜像并启动：
- `controller` 在 `:7001`
- `web` 在 `:3000`
- `agent` 连接到本地 Docker 套接字

### 3. 访问界面

打开浏览器访问 `http://localhost:3000` 查看 Web 界面。

默认的开发 CLI 令牌是 `dev-admin-token`。

已发布的镜像地址：
- 默认：`forgejo.alexma.top/alexma233/composia` 和 `forgejo.alexma.top/alexma233/composia-web`
- 可选：`ghcr.io/alexma233/composia` 和 `ghcr.io/alexma233/composia-web`

### 4. 停止服务栈

```bash
docker compose down
```

## 下一步

- 了解 [架构](./architecture) 详情
- 查看 API 文档
- 部署你的第一个服务

## 开发

如需使用源代码进行本地开发，请参阅[开发指南](./development)。
