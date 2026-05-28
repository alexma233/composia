---
title: "Docker Compose"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Docker Compose 堆栈使用官方的 [`docker-compose.yaml`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/docker-compose.yaml) 运行控制器、一个本地 agent 和 Web UI。

## 下载文件

您无需克隆整个仓库来进行 Docker Compose 安装。下载 compose 文件和环境模板：

```bash
curl -LO https://forgejo.alexma.top/alexma233/composia/raw/branch/main/docker-compose.yaml
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/.env.example -o .env
```

在启动堆栈之前编辑 `.env`。模板按角色分组；对于一体化堆栈，保留所有组。各变量的含义请参阅[配置](../configuration/)。

查找主机上 Docker 套接字的组 ID：

```bash
stat -c '%g' /var/run/docker.sock
```

将 `DOCKER_SOCK_GID` 设置为该值。

## Agent 仓库路径

`COMPOSIA_AGENT_REPO_DIR` 的挂载方式为：

```yaml
- ${COMPOSIA_AGENT_REPO_DIR}:${COMPOSIA_AGENT_REPO_DIR}
```

主机路径和容器路径必须相同。Agent 调用主机 Docker 守护进程，主机 Docker 守护进程从主机文件系统解析绑定挂载。如果服务仓库在 agent 容器内挂载到不同路径，Docker Compose 可能会生成不存在的主机路径。

在两侧使用相同的绝对路径，例如：

```bash
COMPOSIA_AGENT_REPO_DIR=/srv/composia/repo-agent
```

## 基础 `config.yaml`

在 `COMPOSIA_CONFIG_DIR` 内创建 `config.yaml`。Docker Compose 文件将此目录挂载到 `/app/configs`。

```yaml {filename="config.yaml"}
controller:
  listen_addr: ":7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  access_tokens:
    - name: "web"
      token: "REPLACE_WITH_WEB_ACCESS_TOKEN"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
```

在 `.env` 中将 `WEB_CONTROLLER_ACCESS_TOKEN` 设置为与 `controller.access_tokens[0].token` 相同的值。

## Web 密码

`WEB_LOGIN_PASSWORD_HASH` 必须是 Argon2 密码哈希。使用支持 Argon2 的密码哈希工具，并将完整的编码哈希粘贴到 `.env` 中。

使用任何密码学安全的随机生成器生成 `WEB_SESSION_SECRET`，例如：

```bash
openssl rand -hex 32
```

## 启动

```bash
docker compose up -d
docker compose ps
```

在 `http://localhost:3000` 打开 Web UI。

## 角色拆分

Compose 文件按角色分段：

- **控制器堆栈**: `init-repo-controller`、`init-perms-controller`、`controller`。
- **Web UI**: `web`。
- **共享初始化**: `init-config-perms`。
- **Agent 堆栈**: `init-perms-agent`、`agent`。

对于一体化部署以外的场景，请根据您的拓扑显式拆分这些部分。控制器和 Web 可以一起运行，也可以分开运行。每个 agent 节点保留 agent 堆栈及其自己的 Docker 套接字访问。

## 镜像

发布镜像发布到 Forgejo、GHCR 和 Docker Hub：

| 组件 | Forgejo | GHCR | Docker Hub |
|-----------|---------|------|------------|
| CLI | `forgejo.alexma.top/alexma233/composia-cli` | `ghcr.io/alexma233/composia-cli` | `alexma233/composia-cli` |
| Controller | `forgejo.alexma.top/alexma233/composia-controller` | `ghcr.io/alexma233/composia-controller` | `alexma233/composia-controller` |
| Agent | `forgejo.alexma.top/alexma233/composia-agent` | `ghcr.io/alexma233/composia-agent` | `alexma233/composia-agent` |
| Web | `forgejo.alexma.top/alexma233/composia-web` | `ghcr.io/alexma233/composia-web` | `alexma233/composia-web` |

Canary 镜像仅发布到 Forgejo 和 GHCR。

## 常见检查

- 控制器无法启动：验证 `config.yaml` 是否存在于 `COMPOSIA_CONFIG_DIR` 下，以及所需的控制器路径是否存在或可以创建。
- Agent 无法使用 Docker：验证 `DOCKER_SOCK_GID` 与主机上 `/var/run/docker.sock` 的组 ID 匹配。
- Web 无法连接控制器：`WEB_CONTROLLER_ADDR` 用于 Web 服务器容器，而 `WEB_BROWSER_CONTROLLER_ADDR` 用于浏览器。
