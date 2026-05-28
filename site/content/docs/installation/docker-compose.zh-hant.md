---
title: "Docker Compose"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Docker Compose 堆疊從官方 [`docker-compose.yaml`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/docker-compose.yaml) 執行控制器、一個本地代理和 Web UI。

## 下載檔案

使用 Docker Compose 安裝不需要複製整個存放庫。下載 compose 檔案和環境範本：

```bash
curl -LO https://forgejo.alexma.top/alexma233/composia/raw/branch/main/docker-compose.yaml
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/.env.example -o .env
```

在啟動堆疊前編輯 `.env`。範本按角色分組；對於整合式堆疊，保留所有組別。各變數的意義請參見[設定](../configuration/)。

在主機上找到 Docker socket 群組 ID：

```bash
stat -c '%g' /var/run/docker.sock
```

將 `DOCKER_SOCK_GID` 設定為該值。

## 代理存放庫路徑

`COMPOSIA_AGENT_REPO_DIR` 掛載為：

```yaml
- ${COMPOSIA_AGENT_REPO_DIR}:${COMPOSIA_AGENT_REPO_DIR}
```

主機路徑和容器路徑必須相同。代理呼叫主機 Docker 守護程序，而主機 Docker 守護程序從主機檔案系統解析繫結掛載。如果服務存放庫在代理容器內掛載到不同路徑，Docker Compose 可能會產生不存在的主機路徑。

在兩側使用相同的絕對路徑，例如：

```bash
COMPOSIA_AGENT_REPO_DIR=/srv/composia/repo-agent
```

## 基本的 `config.yaml`

在 `COMPOSIA_CONFIG_DIR` 內建立 `config.yaml`。Docker Compose 檔案將此目錄掛載到 `/app/configs`。

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

將 `.env` 中的 `WEB_CONTROLLER_ACCESS_TOKEN` 設定為與 `controller.access_tokens[0].token` 相同的值。

## Web 密碼

`WEB_LOGIN_PASSWORD_HASH` 必須是 Argon2 密碼雜湊。使用支援 Argon2 的密碼雜湊工具，並將完整的編碼雜湊貼入 `.env`。

使用任何密碼學安全的隨機產生器來產生 `WEB_SESSION_SECRET`，例如：

```bash
openssl rand -hex 32
```

## 啟動

```bash
docker compose up -d
docker compose ps
```

在 `http://localhost:3000` 開啟 Web UI。

## 角色拆分

Compose 檔案按角色分區：

- **控制器堆疊**：`init-repo-controller`、`init-perms-controller`、`controller`。
- **Web UI**：`web`。
- **共用初始化**：`init-config-perms`。
- **代理堆疊**：`init-perms-agent`、`agent`。

對於整合式部署以外的任何情況，根據您的拓撲明確拆分這些區段。控制器和 Web 可以一起執行或分開執行。每個代理節點保留代理堆疊及其自身的 Docker socket 存取。

## 映像檔

發行版本映像檔發布到 Forgejo、GHCR 和 Docker Hub：

| 元件 | Forgejo | GHCR | Docker Hub |
|-----------|---------|------|------------|
| CLI | `forgejo.alexma.top/alexma233/composia-cli` | `ghcr.io/alexma233/composia-cli` | `alexma233/composia-cli` |
| Controller | `forgejo.alexma.top/alexma233/composia-controller` | `ghcr.io/alexma233/composia-controller` | `alexma233/composia-controller` |
| Agent | `forgejo.alexma.top/alexma233/composia-agent` | `ghcr.io/alexma233/composia-agent` | `alexma233/composia-agent` |
| Web | `forgejo.alexma.top/alexma233/composia-web` | `ghcr.io/alexma233/composia-web` | `alexma233/composia-web` |

Canary 映像檔僅發布到 Forgejo 和 GHCR。

## 常見檢查

- 控制器無法啟動：驗證 `config.yaml` 是否存在於 `COMPOSIA_CONFIG_DIR` 下，且必要的控制器路徑存在或可建立。
- 代理無法使用 Docker：驗證 `DOCKER_SOCK_GID` 與主機上的 `/var/run/docker.sock` 匹配。
- Web 無法連接控制器：`WEB_CONTROLLER_ADDR` 用於 Web 伺服器容器，而 `WEB_BROWSER_CONTROLLER_ADDR` 用於瀏覽器。
