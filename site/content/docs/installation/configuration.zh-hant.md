---
title: "設定"
date: '2026-05-26T00:00:00+08:00'
weight: 40
---

此頁面涵蓋安裝層級的設定：控制器設定、代理設定、Web 環境變數與 age 金鑰設定。

服務定義位於 `composia-meta.yaml` 中。請參見[服務指南](/docs/guide/service/)了解該檔案。

## 設定檔結構

控制器和代理使用相同的 YAML 檔案格式。一個檔案可以包含任一區段或兩者：

```yaml
controller:
  # 控制器設定

agent:
  # 代理設定
```

必須至少存在 `controller` 或 `agent` 其中之一。

當同一個設定檔同時包含兩個區段時，本地代理被視為內建節點：

- `agent.node_id` 必須為 `main`。
- `controller.nodes` 必須包含一個帶有 `id: main` 的項目。
- `controller.repo_dir` 和 `agent.repo_dir` 不得為相同路徑。

## 完整設定範本

此範本顯示每個支援的安裝層級鍵。它是結構參考，而非可直接複製貼上的預設值。移除您不使用的區段、移除空清單項目，並對每個類似密鑰的欄位使用內聯值或 `_file` 值。

```yaml {filename="config.yaml"}
controller:
  listen_addr: ":7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"

  access_tokens:
    - name: "web"
      token: "REPLACE_WITH_WEB_ACCESS_TOKEN"
      token_file: ""
      enabled: true
      comment: "Web UI 存取權杖"

  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      public_ipv4: ""
      public_ipv6: ""
      token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
      token_file: ""

  git:
    remote_url: ""
    branch: "main"
    pull_interval: ""
    author_name: "Composia"
    author_email: "composia@example.com"
    auth:
      username: ""
      token: ""
      token_file: ""

  backup:
    default_schedule: ""

  updates:
    default_check_schedule: ""
    auto_apply: false
    backup_before_update: true
    digest_pin: false
    semver:
      default_allow:
        - patch
        - minor
    forge_auth:
      github:
        url: "https://github.com"
        token: ""
        token_file: ""
        api_url: "https://api.github.com"
      gitlab:
        url: "https://gitlab.com"
        token: ""
        token_file: ""
        api_url: "https://gitlab.com/api/v4"
      forgejo:
        url: "https://forgejo.example.com"
        token: ""
        token_file: ""
        api_url: ""

  auto_deploy:
    infra: false
    services: false

  dns:
    cloudflare:
      api_token: ""
      api_token_file: ""
      zones: []
    alidns:
      access_key_id: ""
      access_key_id_file: ""
      access_key_secret: ""
      access_key_secret_file: ""
      security_token: ""
      security_token_file: ""
      region_id: ""
      zones: []
    dnspod:
      secret_id: ""
      secret_id_file: ""
      secret_key: ""
      secret_key_file: ""
      session_token: ""
      session_token_file: ""
      region: ""
      zones: []
    route53:
      access_key_id: ""
      access_key_id_file: ""
      secret_access_key: ""
      secret_access_key_file: ""
      session_token: ""
      session_token_file: ""
      region: ""
      profile: ""
      hosted_zone_id: ""
      zones: []
    huaweicloud:
      access_key_id: ""
      access_key_id_file: ""
      secret_access_key: ""
      secret_access_key_file: ""
      region_id: ""
      zones: []

  rustic:
    main_nodes:
      - "main"
    maintenance:
      forget_schedule: ""
      prune_schedule: ""

  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
    recipient_file: ""
    armor: true

  notifications:
    alertmanager:
      enabled: true
      listen_path: "/api/v1/alerts"
    smtp:
      enabled: false
      host: ""
      port: 587
      encryption: starttls
      username: ""
      password: ""
      password_file: ""
      from: ""
      to: []
      on: []
      task_sources: []
    telegram:
      enabled: false
      bot_token: ""
      bot_token_file: ""
      chat_id: ""
      on: []
      task_sources: []

agent:
  controller_addr: "http://controller:7001"
  controller_grpc: false
  controller_headers:
    - name: ""
      value: ""
      value_file: ""
  node_id: "main"
  token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
  token_file: ""
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
  caddy:
    generated_dir: ""
```

請勿保留如含有空白 `name` 的 `controller_headers` 等空清單項目。它們僅為記錄支援的物件結構而顯示。

Web 存取權杖和主要代理權杖必須不同。

## Age 金鑰設定

`controller.secrets` 是可選的。僅在您使用 Composia 管理的加密密鑰時才設定它。

當設定了 `controller.secrets` 時，`identity_file` 為必要項。`recipient_file` 為可選。若省略，Composia 從私鑰衍生接收者。

產生私鑰：

```bash
age-keygen -o age-identity.key
```

可選的接收者檔案：

```bash
age-keygen -y age-identity.key > age-recipients.txt
```

在設定中使用私鑰：

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
```

或同時使用兩個檔案：

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
  recipient_file: "/app/configs/age-recipients.txt"
```

`armor` 是可選的，預設為 `true`。

## 控制器設定參考

### 必要鍵

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `listen_addr` | `string` | 控制器監聽位址，例如 `":7001"` 或 `"127.0.0.1:7001"`。 |
| `repo_dir` | `string` | 期望狀態 Git 存放庫路徑。 |
| `state_dir` | `string` | 控制器狀態路徑。 |
| `log_dir` | `string` | 任務日誌目錄。 |
| `nodes` | `[]object` | 已設定的代理節點。此鍵必須存在，即使是空的。 |

### 可選的頂層鍵

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `access_tokens` | `[]object` | 用於 Web UI、CLI 和外部客戶端的 API 權杖。 |
| `backup` | `object` | 全域備份預設值。 |
| `git` | `object` | 期望狀態存放庫遠端同步。 |
| `notifications` | `object` | Alertmanager、SMTP 和 Telegram 通知。 |
| `dns` | `object` | DNS 提供者憑證。 |
| `rustic` | `object` | Rustic 維護設定。 |
| `secrets` | `object` | Age 加密設定。 |
| `updates` | `object` | 映像檔更新預設值和 forge API 驗證。 |
| `auto_deploy` | `object` | 全域自動部署開關。 |

### `nodes[]`

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `id` | `string` | 是 | 唯一的節點 ID。 |
| `display_name` | `string` | 否 | UI 中顯示的名稱。 |
| `enabled` | `bool` | 否 | 不移除節點而停用它。 |
| `public_ipv4` | `string` | 否 | DNS 工作流程使用的公開 IPv4。 |
| `public_ipv6` | `string` | 否 | DNS 工作流程使用的公開 IPv6。 |
| `token` | `string` | 是* | 代理驗證權杖。 |
| `token_file` | `string` | 否 | 從檔案讀取權杖。 |

*使用 `token` 或 `token_file`，不能同時兩者。

### `access_tokens[]`

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 權杖名稱。 |
| `token` | `string` | 是* | 權杖值。 |
| `token_file` | `string` | 否 | 從檔案讀取權杖。 |
| `enabled` | `bool` | 否 | 不移除權杖而停用它。 |
| `comment` | `string` | 否 | 管理備註。 |

存取權杖不得與節點權杖或其他存取權杖重複。

### `git`

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `remote_url` | `string` | 否 | Git 遠端 URL。 |
| `branch` | `string` | 否 | 要同步的分支。 |
| `pull_interval` | `string` | 條件必要 | 設定 `remote_url` 時為必要項。 |
| `author_name` | `string` | 否 | 控制器寫入的提交作者名稱。 |
| `author_email` | `string` | 否 | 提交作者電子郵件。 |
| `auth.username` | `string` | 否 | Git 使用者名稱。 |
| `auth.token` | `string` | 否 | Git 權杖。 |
| `auth.token_file` | `string` | 否 | 從檔案讀取 Git 權杖。 |

### `secrets`

整個區段是可選的。若此區段存在，則以下規則適用：

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `provider` | `string` | 是 | 必須為 `age`。 |
| `identity_file` | `string` | 是 | Age 私鑰路徑。 |
| `recipient_file` | `string` | 否 | Age 接收者檔案路徑。若省略，從 `identity_file` 衍生接收者。 |
| `armor` | `bool` | 否 | ASCII armoring 加密輸出。預設為 `true`。 |

### `backup`

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `default_schedule` | `string` | 服務備份的預設 cron 排程。 |

### `updates`

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `default_check_schedule` | `string` | 映像檔更新檢查的預設 cron 排程。 |
| `auto_apply` | `bool` | 預設自動套用更新。 |
| `backup_before_update` | `bool` | 套用更新前備份資料。 |
| `digest_pin` | `bool` | 以摘要固定映像檔。 |
| `semver.default_allow` | `[]string` | 允許的 semver 版本升級層級：`patch`、`minor`、`major`。 |
| `forge_auth.github` | `object` 或 `[]object` | GitHub API 驗證。 |
| `forge_auth.gitlab` | `object` 或 `[]object` | GitLab API 驗證。 |
| `forge_auth.forgejo` | `object` 或 `[]object` | Forgejo API 驗證。 |

每個 forge 驗證項目支援：

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `url` | `string` | Forge 基礎 URL。 |
| `token` | `string` | API 權杖。 |
| `token_file` | `string` | 從檔案讀取 API 權杖。 |
| `api_url` | `string` | API URL 覆蓋。 |

### `auto_deploy`

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `infra` | `bool` | Git 變更後自動部署基礎架構服務。 |
| `services` | `bool` | Git 變更後自動部署一般服務。 |

### `dns`

| 提供者鍵 | 憑證鍵 | 共用鍵 |
|--------------|-----------------|-------------|
| `cloudflare` | `api_token`、`api_token_file` | `zones` |
| `alidns` | `access_key_id`、`access_key_id_file`、`access_key_secret`、`access_key_secret_file`、`security_token`、`security_token_file`、`region_id` | `zones` |
| `dnspod` | `secret_id`、`secret_id_file`、`secret_key`、`secret_key_file`、`session_token`、`session_token_file`、`region` | `zones` |
| `route53` | `access_key_id`、`access_key_id_file`、`secret_access_key`、`secret_access_key_file`、`session_token`、`session_token_file`、`region`、`profile`、`hosted_zone_id` | `zones` |
| `huaweicloud` | `access_key_id`、`access_key_id_file`、`secret_access_key`、`secret_access_key_file`、`region_id` | `zones` |

### `rustic`

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `main_nodes` | `[]string` | 執行 Rustic 操作的節點 ID。每個必須引用 `controller.nodes`。 |
| `maintenance.forget_schedule` | `string` | `rustic forget` 的 cron 排程。 |
| `maintenance.prune_schedule` | `string` | `rustic prune` 的 cron 排程。 |

### `notifications.alertmanager`

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `enabled` | `bool` | 當區段存在時預設啟用。 |
| `listen_path` | `string` | Webhook 路徑。預設為 `/api/v1/alerts`。必須以 `/` 開頭。 |

### `notifications.smtp`

| 鍵 | 型別 | 啟用時必要 | 說明 |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | 否 | 當區段存在時預設啟用。 |
| `host` | `string` | 是 | SMTP 主機。 |
| `port` | `int` | 是 | SMTP 連接埠，1 到 65535。 |
| `encryption` | `string` | 否 | `none`、`starttls` 或 `ssl_tls`。預設為 `starttls`。 |
| `username` | `string` | 否 | SMTP 使用者名稱。 |
| `password` | `string` | 否 | SMTP 密碼。 |
| `password_file` | `string` | 否 | 從檔案讀取密碼。 |
| `from` | `string` | 是 | 寄件者地址。 |
| `to` | `[]string` | 是 | 收件者清單。 |
| `on` | `[]string` | 否 | 通知事件過濾器。 |
| `task_sources` | `[]string` | 否 | 任務來源過濾器：`web`、`cli`、`others`、`schedule`、`system`。 |

### `notifications.telegram`

| 鍵 | 型別 | 啟用時必要 | 說明 |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | 否 | 當區段存在時預設啟用。 |
| `bot_token` | `string` | 是* | Telegram 機器人權杖。 |
| `bot_token_file` | `string` | 否 | 從檔案讀取機器人權杖。 |
| `chat_id` | `string` | 是 | 目標聊天室 ID。 |
| `on` | `[]string` | 否 | 通知事件過濾器。 |
| `task_sources` | `[]string` | 否 | 任務來源過濾器。 |

## 代理設定參考

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `controller_addr` | `string` | 是 | 代理可連線的控制器 URL。 |
| `controller_grpc` | `bool` | 否 | 使用 gRPC 而非基於 HTTP 的 Connect。 |
| `controller_headers` | `[]object` | 否 | 發送給控制器的額外 HTTP 標頭。 |
| `node_id` | `string` | 是 | 此代理的節點 ID。必須與 `controller.nodes[].id` 匹配。 |
| `token` | `string` | 是* | 與控制器設定匹配的節點權杖。 |
| `token_file` | `string` | 否 | 從檔案讀取節點權杖。 |
| `repo_dir` | `string` | 是 | 代理服務存放庫路徑。 |
| `state_dir` | `string` | 是 | 代理狀態目錄。 |
| `caddy` | `object` | 否 | 代理端 Caddy 設定。 |

*使用 `token` 或 `token_file`，不能同時兩者。

### `controller_headers[]`

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | HTTP 標頭名稱。標頭名稱以不區分大小寫的方式去重。 |
| `value` | `string` | 是* | 標頭值。 |
| `value_file` | `string` | 否 | 從檔案讀取標頭值。 |

### `caddy`

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `generated_dir` | `string` | 產生的 Caddy 設定目錄。預設為 `<state_dir>/caddy/generated`。 |

## Web 環境變數

Web 伺服器讀取環境變數。在 Docker Compose 中，這些透過 `.env` 設定。

| 變數 | 必要 | 說明 |
|----------|----------|-------------|
| `WEB_CONTROLLER_ADDR` | 是 | 從 Web 伺服器處理程序看到的控制器位址。在 Docker Compose 中：`http://controller:7001`。 |
| `WEB_BROWSER_CONTROLLER_ADDR` | 是 | 從瀏覽器看到的控制器位址。 |
| `WEB_CONTROLLER_ACCESS_TOKEN` | 是 | 控制器存取權杖。必須與 `controller.access_tokens[].token` 匹配。 |
| `WEB_CONTROLLER_HEADERS` | 否 | Web 伺服器呼叫控制器時發送的額外標頭的 JSON 物件。 |
| `WEB_LOGIN_USERNAME` | 是 | Web 登入使用者名稱。 |
| `WEB_LOGIN_PASSWORD_HASH` | 是 | Argon2 密碼雜湊。 |
| `WEB_SESSION_SECRET` | 是 | 隨機的會話簽名密鑰。 |
| `ORIGIN` | 取決於部署 | Web 伺服器的公開來源。 |
| `HOST` | 否 | 主機繫結位址。 |
| `PORT` | 否 | Web 伺服器連接埠。 |

## 內聯值與 `_file` 值

許多類似密鑰的欄位同時支援內聯值和檔案引用。範例：

- `token` / `token_file`
- `password` / `password_file`
- `api_token` / `api_token_file`
- `value` / `value_file`

僅使用一種形式。若同時設定兩者，啟動會失敗。
