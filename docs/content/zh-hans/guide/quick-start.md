# 快速开始

本指南将帮助你在几分钟内使用预构建的容器镜像启动并运行 Composia。

## 前提条件

- Docker Engine 20.10+
- Docker Compose v2.0+

## 安装步骤

### 1. 创建工作目录

先准备一个空目录，并保留下面这套相对路径：

```text
composia/
├── docker-compose.yaml
└── configs/
    └── config.compose.yaml
```

### 2. 下载启动文件

从仓库下载以下文件，并按上面的目录结构保存：

- [`docker-compose.yaml`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/docker-compose.yaml)
- [`configs/config.compose.yaml`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/configs/config.compose.yaml)

如果你打算保留 `config.compose.yaml` 里的默认 `secrets` 配置，还需要同时下载：

- [`configs/age-identity.key`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/configs/age-identity.key)
- [`configs/age-recipients.txt`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/configs/age-recipients.txt)

### 3. 修改平台配置

启动前至少检查并修改这些值：

- `controller.access_tokens[].token`：Controller 访问 token，Web UI 会使用它访问 Controller
- `controller.nodes[].token` 与 `agent.token`：节点认证 token，二者必须一致
- `docker-compose.yaml` 里的 `COMPOSIA_ACCESS_TOKEN`：必须与 `controller.access_tokens[].token` 保持一致

如果你不准备使用仓库附带的 age 密钥文件，请在 `configs/config.compose.yaml` 中替换 `secrets` 配置，或先移除该段配置。

### 4. 启动 Composia

下面的命令会使用当前目录中的 `docker-compose.yaml` 和 `configs/config.compose.yaml` 启动 Composia：

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

Web UI 不会提示输入 token。它会使用注入到 Web 服务进程中的 `COMPOSIA_ACCESS_TOKEN` 环境变量。这个值必须与 `controller.access_tokens[].token` 中某个已启用的 token 一致。

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
- [阅读配置指南](./configuration) —— 学习如何配置 controller 和 agent
- [查看架构概览](./architecture) —— 理解系统工作原理

## 本地开发

如需从源代码进行本地开发，请参阅 [开发指南](./development)。
