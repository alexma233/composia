# 快速开始

本指南将帮助你在几分钟内使用预构建的容器镜像启动并运行 Composia。

## 前提条件

- Docker Engine 20.10+
- Docker Compose v2.0+

## 安装步骤

### 1. 克隆仓库

```bash
git clone https://forgejo.alexma.top/alexma233/composia.git
cd composia
```

### 2. 检查仓库自带配置

仓库已经提供 `configs/config.compose.yaml`。

- 直接用于本地快速启动即可。
- 只有在你需要不同 token 或路径时才修改它。
- 如果保留默认的 secrets 配置，也要保留仓库附带的 `configs/age-identity.key` 和 `configs/age-recipients.txt`。

### 3. 启动服务栈

下面的命令使用仓库根目录中的 `docker-compose.yaml`。你前面创建的 `configs/config.compose.yaml` 会被这套 Compose 栈作为平台配置文件使用。

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

### 4. 访问界面

打开浏览器访问 `http://localhost:3000`。

Web UI 不会提示输入 token。它会使用注入到 Web 服务进程中的 `COMPOSIA_CLI_TOKEN` 环境变量。在仓库提供的 `docker-compose.yaml` 中，这个值被设置为 `dev-admin-token`。

### 5. 部署第一个服务

1. 在 Web 界面中进入「服务」页面并点击「Create service」
2. 输入服务名称
3. 在编辑器中添加 `docker-compose.yaml` 内容
4. 在 `composia-meta.yaml` 中定义目标节点
5. 点击「部署」

### 6. 停止服务栈

这条命令会停止由仓库根目录 `docker-compose.yaml` 启动的那套容器栈：

```bash
docker compose down
```

如需同时删除这套 Compose 使用的卷，添加 `-v` 参数：

```bash
docker compose down -v
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
