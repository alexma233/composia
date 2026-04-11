# 快速开始

本指南将帮助你在几分钟内使用预构建的容器镜像启动并运行 Composia。

## 前提条件

- Docker Engine 20.10+
- Docker Compose v2.0+

## 安装步骤

### 1. 创建工作目录

先创建本地工作目录，并直接下载生产用的 Compose 文件，不需要克隆整个仓库：

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
└── config/
    ├── config.yaml
    ├── age-identity.key
    └── age-recipients.txt
```

### 2. 下载启动文件

发布的 `docker-compose.yaml` 已可直接用于生产启动。根据 [配置指南](./configuration)、[Controller 配置](./configuration/controller) 和 [Agent 配置](./configuration/agent) 编写 `config/config.yaml`，并按需修改 `.env` 里的占位值。

如果你启用 `secrets`，请参考 [Secrets 配置](./configuration/secrets) 自行生成 age 密钥：

```bash
mkdir -p config
age-keygen -o config/age-identity.key
grep "public key:" config/age-identity.key | awk '{print $4}' > config/age-recipients.txt
```

### 3. 修改平台配置

启动前至少检查并修改这些值：

- `controller.access_tokens[].token`：Controller 访问 token，Web UI 会使用它访问 Controller
- `controller.nodes[].token` 与 `agent.token`：节点认证 token，二者必须一致
- `.env` 里的 `COMPOSIA_ACCESS_TOKEN`：必须与 `controller.access_tokens[].token` 保持一致
- `.env` 里的 `WEB_LOGIN_USERNAME`：Web 登录页使用的本地用户名
- `.env` 里的 `WEB_LOGIN_PASSWORD_HASH`：Web 登录页使用的 Argon2 密码哈希
- `.env` 里的 `WEB_SESSION_SECRET`：用于签名 Web session cookie 的随机密钥
- `.env` 里的 `ORIGIN`：必须改成你实际访问 Web UI 的地址，例如 `http://localhost:3000`、`http://127.0.0.1:3000` 或正式域名。不要混用不同 host，否则表单登录会触发 `Cross-site POST form submissions are forbidden`

启动前先生成 Argon2 哈希。你可以直接在这个页面里生成：

<ClientOnly>
  <Argon2Generator />
</ClientOnly>

如果你更想用 CLI，也可以执行：

```bash
docker run --rm authelia/authelia:latest authelia crypto hash generate argon2 --password '替换成你的密码'
```

再生成一个足够长的 session 密钥，例如：

```bash
openssl rand -hex 32
```

如果你暂时不使用 `secrets`，可以先移除 `config/config.yaml` 中的 `secrets` 段。

### 4. 启动 Composia

确认已经修改 `.env` 中的占位值后，下面的命令会使用当前工作目录里的 `docker-compose.yaml`、`.env` 和 `config/config.yaml` 启动 Composia：

```bash
docker compose up -d
```

启动后，以下长期运行的服务会启动：

| 服务 | 端口 | 说明 |
|------|------|------|
| controller | `:7001` | 控制平面 API |
| web | `:3000` | Web 管理界面 |
| agent | - | 执行代理（连接本地 Docker） |

此外，Compose 还会先运行一次性的 `init-repo-controller` 容器，用于初始化 controller 的 Git 工作树卷。

### 5. 访问界面

先确认 `.env` 里的 `ORIGIN` 与你准备打开的地址完全一致，然后再访问，例如 `http://localhost:3000`。

如果你通过 SSH tunnel、本地端口映射或反向代理访问 Web UI，必须同步修改 `ORIGIN`。例如：

- 访问 `http://localhost:3000` 时，设置 `ORIGIN=http://localhost:3000`
- 访问 `http://127.0.0.1:3000` 时，设置 `ORIGIN=http://127.0.0.1:3000`
- 访问 `https://composia.example.com` 时，设置 `ORIGIN=https://composia.example.com`

`localhost` 和 `127.0.0.1` 不是同一个 origin，不能混用。

Web UI 现在有两层鉴权：

- 浏览器先使用 `WEB_LOGIN_USERNAME` 和 `WEB_LOGIN_PASSWORD_HASH` 对应的密码登录
- Web 服务进程再使用 `COMPOSIA_ACCESS_TOKEN` 访问 Controller

浏览器不会拿到 `COMPOSIA_ACCESS_TOKEN`。登录成功后只会保存一个签名的 HttpOnly session cookie。

### 6. 部署第一个服务

1. 在 Web 界面中进入「服务」页面并点击「Create service」
2. 输入服务名称
3. 在编辑器中添加 `docker-compose.yaml` 内容
4. 在 `composia-meta.yaml` 中定义目标节点
5. 点击「部署」

### 7. 停止 Composia

这条命令会停止当前工作目录里 `docker-compose.yaml` 启动的 Composia 容器栈：

```bash
docker compose down
```

## 镜像源选择

默认使用自建 Forgejo Registry。如需使用 GitHub Container Registry：

编辑当前目录里的 `docker-compose.yaml`，将镜像地址替换为：

```yaml
services:
  controller:
    image: ghcr.io/alexma233/composia:latest
  
  web:
    image: ghcr.io/alexma233/composia-web:latest
  
  agent:
    image: ghcr.io/alexma233/composia:latest
```

## 下一步

- [了解核心概念](./core-concepts) —— 理解 Service、Instance、Container、Node 的关系
- [阅读配置指南](./configuration) —— 查看平台配置总览
- [阅读 Controller 配置](./configuration/controller) —— 学习基础字段、token 和节点配置
- [阅读 Agent 配置](./configuration/agent) —— 学习 agent 必填字段和 Caddy 输出目录
- [查看架构概览](./architecture) —— 理解系统工作原理

## 本地开发

如需从源代码进行本地开发，请参阅 [开发指南](./development)。
