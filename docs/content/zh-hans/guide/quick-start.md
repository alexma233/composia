# 快速开始

几分钟内启动 Composia 并部署你的第一个服务。完整的配置选项说明请参见[安装指南](./installation)。

## 前提条件

- Docker Engine 20.10+ 和 Docker Compose v2.0+

## 准备工作

```bash
mkdir -p composia/config
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/docker-compose.yaml -o composia/docker-compose.yaml
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/.env.example -o composia/.env
cd composia
```

## 最小配置

编辑 `.env`，至少填写以下内容：

```env
# 必填
WEB_CONTROLLER_ADDR=http://controller:7001
WEB_BROWSER_CONTROLLER_ADDR=http://localhost:7001
WEB_CONTROLLER_ACCESS_TOKEN=替换为安全令牌
WEB_LOGIN_USERNAME=admin
WEB_LOGIN_PASSWORD_HASH=<用下方命令生成>
WEB_SESSION_SECRET=<用下方命令生成>
ORIGIN=http://localhost:3000
DOCKER_SOCK_GID=<从 ls -ln /var/run/docker.sock 获取>
```

生成密码哈希和 session 密钥：

```bash
docker run --rm authelia/authelia:latest authelia crypto hash generate argon2 --password '你的密码'
openssl rand -hex 32
```

创建最小 `config/config.yaml`：

```yaml
controller:
  listen_addr: ":7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  access_tokens:
    - name: "admin"
      token: "替换为安全令牌"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-agent-token"

agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-agent-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
```

创建必要的目录：

```bash
mkdir -p ./data/repo-controller ./data/state-controller ./data/logs ./data/state-agent
sudo mkdir -p /data/repo-agent && sudo chown 65532:65532 /data/repo-agent
```

## 启动

```bash
docker compose up -d
```

打开 `http://localhost:3000`，用你配置的账号密码登录。

## 部署第一个服务

1. 进入 **Services** > **Create service**
2. 输入服务名称
3. 在编辑器中粘贴 `docker-compose.yaml` 内容
4. 在 `composia-meta.yaml` 中将目标节点设为 `main`
5. 点击 **Deploy**

以下是最小示例：

```yaml
# composia-meta.yaml
name: hello
nodes:
  - main
```
```yaml
# docker-compose.yaml
services:
  hello:
    image: nginx:alpine
    ports:
      - "8080:80"
```

## 下一步

- [安装指南](./installation) —— 包含所有选项的完整安装流程
- [核心概念](./core-concepts) —— 理解 Service、Instance、Container、Node
- [架构概览](./architecture) —— 系统架构与数据流
- [配置指南](./configuration) —— 所有平台配置选项
- [服务定义](./service-definition) —— 创建和配置服务
