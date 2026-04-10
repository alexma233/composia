# 快速开始

本指南将帮助你在几分钟内使用预构建的容器镜像启动并运行 Composia。

## 前提条件

- Docker Engine 20.10+
- Docker Compose v2.0+

## 安装步骤

### 1. 创建工作目录

先准备一个空目录，并自行创建启动文件：

```text
composia/
├── docker-compose.yaml
└── config/
    ├── config.yaml
    ├── age-identity.key
    └── age-recipients.txt
```

### 2. 下载启动文件

根据 [配置指南](./configuration)、[Controller 配置](./configuration/controller) 和 [Agent 配置](./configuration/agent) 自行编写 `docker-compose.yaml` 和 `config/config.yaml`。

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
- `docker-compose.yaml` 里的 `COMPOSIA_ACCESS_TOKEN`：必须与 `controller.access_tokens[].token` 保持一致
- `docker-compose.yaml` 里的 `WEB_LOGIN_USERNAME`：Web 登录页使用的本地用户名
- `docker-compose.yaml` 里的 `WEB_LOGIN_PASSWORD_HASH`：Web 登录页使用的 Argon2 密码哈希
- `docker-compose.yaml` 里的 `WEB_SESSION_SECRET`：用于签名 Web session cookie 的随机密钥

启动前先生成 Argon2 哈希：

```bash
cd web
bun -e "import { hash } from 'argon2'; console.log(await hash(Bun.argv[2]));" -- "替换成你的密码"
```

再生成一个足够长的 session 密钥，例如：

```bash
openssl rand -hex 32
```

如果你暂时不使用 `secrets`，可以先移除 `config/config.yaml` 中的 `secrets` 段。

### 4. 启动 Composia

下面的命令会使用当前目录中的 `docker-compose.yaml` 和 `config/config.yaml` 启动 Composia：

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

打开浏览器访问 `http://localhost:3000`。

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

这条命令会停止当前目录 `docker-compose.yaml` 启动的 Composia 容器栈：

```bash
docker compose down
```

## 镜像源选择

默认使用自建 Forgejo Registry。如需使用 GitHub Container Registry：

编辑 `docker-compose.yaml`，将镜像地址替换为：

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
