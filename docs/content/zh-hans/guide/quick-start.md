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

### 2. 创建配置文件

创建平台配置文件：

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

agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-agent-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
EOF
```

### 3. 启动服务栈

```bash
docker compose up -d
```

启动后，以下服务将运行：

| 服务 | 端口 | 说明 |
|------|------|------|
| controller | `:7001` | 控制平面 API |
| web | `:3000` | Web 管理界面 |
| agent | - | 执行代理（连接本地 Docker） |

### 4. 访问界面

打开浏览器访问 `http://localhost:3000`，使用以下默认令牌登录：

- **CLI Token**: `dev-admin-token`

### 5. 部署第一个服务

1. 在 Web 界面中点击「服务」→「新建服务」
2. 输入服务名称和选择目标节点
3. 在编辑器中添加 `docker-compose.yaml` 内容
4. 点击「部署」

### 6. 停止服务栈

```bash
docker compose down
```

如需删除数据卷，添加 `-v` 参数：

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
