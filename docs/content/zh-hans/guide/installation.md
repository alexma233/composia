# 安装指南

本文涵盖使用预构建容器镜像完整安装 Composia 的流程。如果想先快速体验，请查看[快速开始](./quick-start)。

## 前提条件

- Docker Engine 20.10+
- Docker Compose v2.0+

## 创建工作目录

直接下载生产用的 Compose 文件，不需要克隆整个仓库：

```bash
mkdir -p composia/config
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/docker-compose.yaml -o composia/docker-compose.yaml
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/.env.example -o composia/.env
cd composia
```

目录结构如下：

```text
composia/
├── docker-compose.yaml
├── .env
└── config/
    ├── config.yaml
    ├── age-identity.key
    └── age-recipients.txt
```

## 平台配置

结合[配置指南](./configuration)和以下专项文档来编写 `config/config.yaml`：

- [Controller 配置](./configuration/controller) —— 基础字段、访问令牌和节点定义
- [Agent 配置](./configuration/agent) —— 必填字段和 Caddy 输出目录
- [Git 远端同步](./configuration/git-sync) —— `controller.git` 字段（可选）
- [DNS 配置](./configuration/dns) —— `controller.dns` 和 Cloudflare 设置（可选）
- [备份配置](./configuration/backup) —— `controller.backup` 和 `controller.rustic`（可选）
- [Secrets 配置](./configuration/secrets) —— 基于 age 的加密设置（可选）

如果启用 `secrets`，请生成自己的 age 密钥对：

```bash
age-keygen -o config/age-identity.key
grep "public key:" config/age-identity.key | awk '{print $4}' > config/age-recipients.txt
```

## 环境变量

启动前至少检查并修改 `.env` 中的以下值。

### 必填

| 变量 | 说明 |
|----------|-------------|
| `WEB_CONTROLLER_ACCESS_TOKEN` | 必须与 `controller.access_tokens` 中的某个 token 匹配；Web 服务进程用于调用 Controller |
| `WEB_CONTROLLER_ADDR` | Compose 网络内访问 Controller 的基础地址 |
| `WEB_BROWSER_CONTROLLER_ADDR` | 浏览器访问 Controller 的基础地址（用于 WebSocket 终端） |
| `WEB_LOGIN_USERNAME` | Web 登录页用户名 |
| `WEB_LOGIN_PASSWORD_HASH` | 登录密码的 Argon2 哈希 |
| `WEB_SESSION_SECRET` | 用于签名 session cookie 的随机密钥 |
| `ORIGIN` | 你访问 Web UI 的实际地址（如 `http://localhost:3000`）。不要混用不同 host，否则表单登录会触发 `Cross-site POST form submissions are forbidden` |

### 目录挂载

| 变量 | 说明 |
|----------|-------------|
| `COMPOSIA_CONFIG_DIR` | 宿主机上 `config/` 目录的路径 |
| `COMPOSIA_CONTROLLER_REPO_DIR` | Controller Git 工作树的宿主机路径 |
| `COMPOSIA_CONTROLLER_STATE_DIR` | Controller 状态数据（SQLite、缓存）的宿主机路径 |
| `COMPOSIA_CONTROLLER_LOG_DIR` | 任务日志的宿主机路径 |
| `COMPOSIA_AGENT_REPO_DIR` | Agent repo 目录的宿主机路径 |
| `COMPOSIA_AGENT_STATE_DIR` | Agent 状态目录的宿主机路径 |

### Docker Socket

| 变量 | 说明 |
|----------|-------------|
| `DOCKER_SOCK_GID` | 宿主机 `/var/run/docker.sock` 的 GID。Agent 需要加入这个组才能访问本机 Docker。 |

查询方式：

```bash
ls -ln /var/run/docker.sock
```

如果输出显示 `srw-rw---- 1 0 131 ...`，则设置 `DOCKER_SOCK_GID=131`。如果这个值不正确，Agent 会报错 `permission denied while trying to connect to the docker API at unix:///var/run/docker.sock`。

## 创建目录

按默认 `.env` 值，启动前创建以下目录：

```bash
mkdir -p ./data/repo-controller ./data/state-controller ./data/logs ./data/state-agent
sudo mkdir -p /data/repo-agent
sudo chown 65532:65532 /data/repo-agent
```

如果你要修改路径，请同时更新 `.env` 和 `config/config.yaml`。`agent.repo_dir`、宿主机挂载路径和容器内挂载路径必须完全一致。

## 生成密码哈希

可以直接在这个页面生成：

<ClientOnly>
  <Argon2Generator />
</ClientOnly>

或使用 CLI：

```bash
docker run --rm authelia/authelia:latest authelia crypto hash generate argon2 --password '替换成你的密码'
```

生成 session 密钥：

```bash
openssl rand -hex 32
```

## ORIGIN 配置

将 `ORIGIN` 设为浏览器中实际访问的地址：

| 访问方式 | `ORIGIN` 值 |
|---------------|----------------|
| 直接本地访问 | `http://localhost:3000` |
| 指定环回地址 | `http://127.0.0.1:3000` |
| 生产域名 | `https://composia.example.com` |

`localhost` 和 `127.0.0.1` 是不同的 origin，不能混用。如果通过 SSH 隧道或反向代理访问，请将 `ORIGIN` 改为实际访问地址。

## 启动 Composia

```bash
docker compose up -d
```

启动以下长期运行的服务：

| 服务 | 端口 | 说明 |
|---------|------|-------------|
| controller | `:7001` | 管理 API |
| web | `:3000` | Web 管理界面 |
| agent | — | 执行代理（连接本地 Docker） |

Compose 还会先运行多个初始化容器（`init-repo-controller`、`init-perms-controller`、`init-config-perms`、`init-perms-agent`），用于初始化 Git 工作树卷并设置正确的文件权限。

## 访问界面

打开 `http://localhost:3000`（或与你的 `ORIGIN` 值匹配的地址）。

Web UI 有两层鉴权：
- 浏览器使用 `WEB_LOGIN_USERNAME` 和 `WEB_LOGIN_PASSWORD_HASH` 对应的密码登录。
- Web 服务进程使用 `WEB_CONTROLLER_ACCESS_TOKEN` 调用 Controller。浏览器不会拿到这个 token——登录成功后仅保存一个签名的 HttpOnly session cookie。

## 镜像源选择

默认使用自建 Forgejo Registry。镜像同时发布到 GitHub Container Registry 和 Docker Hub。

### Forgejo（默认）

```yaml
services:
  controller:
    image: forgejo.alexma.top/alexma233/composia-controller:latest
  web:
    image: forgejo.alexma.top/alexma233/composia-web:latest
  agent:
    image: forgejo.alexma.top/alexma233/composia-agent:latest
```

### GitHub Container Registry

```yaml
services:
  controller:
    image: ghcr.io/alexma233/composia-controller:latest
  web:
    image: ghcr.io/alexma233/composia-web:latest
  agent:
    image: ghcr.io/alexma233/composia-agent:latest
```

### Docker Hub

```yaml
services:
  controller:
    image: alexma233/composia-controller:latest
  web:
    image: alexma233/composia-web:latest
  agent:
    image: alexma233/composia-agent:latest
```

## 停止 Composia

```bash
docker compose down
```

## 下一步

- [快速开始](./quick-start) —— 5 分钟内部署第一个服务
- [核心概念](./core-concepts) —— 理解 Service、Instance、Container、Node
- [架构概览](./architecture) —— 理解系统工作原理
