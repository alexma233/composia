---
title: "服務設定"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

每個服務位於控制器存放庫中的一個目錄內。服務目錄包含 `composia-meta.yaml` 和一個或多個 Docker Compose 檔案。

最小服務：

```yaml {filename="composia-meta.yaml"}
name: my-app
nodes:
  - main
```

使用預設行為時，Composia 會在同一目錄中尋找 `docker-compose.yaml`。

## 頂層鍵

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 唯一的服務名稱。 |
| `project_name` | `string` | 否 | Docker Compose 專案名稱覆蓋。預設為正規化的服務名稱。 |
| `compose_files` | `[]string` | 否 | Compose 檔案路徑，相對於服務目錄。 |
| `enabled` | `bool` | 否 | 服務是否啟用。預設為 `true`。 |
| `nodes` | `[]string` | 是 | 目標節點 ID。每個必須存在於 `controller.nodes` 中。 |
| `infra` | `object` | 否 | 宣告此服務為 Caddy、Rustic 或純設定基礎架構。 |
| `network` | `object` | 否 | Caddy 和 DNS 設定。 |
| `update` | `object` | 否 | 映像檔更新設定。 |
| `data_protect` | `object` | 否 | 備份和還原資料定義。 |
| `backup` | `object` | 否 | 受保護資料的排程備份。 |
| `migrate` | `object` | 否 | 啟用遷移的受保護資料。 |
| `auto_deploy` | `bool` | 否 | 存放庫變更後自動部署此服務。 |

`compose_files` 項目必須是相對路徑、必須保持在服務目錄內，且不得重複。

## 基礎架構服務

### `infra.caddy`

宣告存放庫的 Caddy 基礎架構服務。

```yaml
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
```

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `compose_service` | `string` | Compose 服務名稱。預設為 `caddy`。 |
| `config_dir` | `string` | Caddy 設定目錄。預設為 `/etc/caddy`。 |

只有一個服務可以宣告為 Caddy 基礎架構。

### `infra.rustic`

宣告存放庫的 Rustic 基礎架構服務。

```yaml
infra:
  rustic:
    compose_service: rustic
    profile: default
    data_protect_dir: /data-protect
    init_args:
      - --set-version
      - "2"
```

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `compose_service` | `string` | Compose 服務名稱。預設為 `rustic`。 |
| `profile` | `string` | Rustic 設定檔名稱。 |
| `data_protect_dir` | `string` | 資料保護工作流程使用的目錄。 |
| `init_args` | `[]string` | 傳遞給 `rustic init` 的額外參數。空白項目會被拒絕。 |

只有一個服務可以宣告為 Rustic 基礎架構。

### `infra.config`

宣告一個純設定基礎架構服務。

```yaml
infra:
  config: {}
```

純設定服務不能與 `infra.caddy` 或 `infra.rustic` 合併使用。它們的 `data_protect` 操作只能使用 `files.copy`。

## 網路

### `network.caddy`

```yaml
network:
  caddy:
    enabled: true
    source: Caddyfile
```

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `enabled` | `bool` | 否 | 啟用 Caddy 管理。預設為 `false`。 |
| `source` | `string` | 條件必要 | Caddyfile 路徑，相對於服務目錄。啟用時為必要項。 |

### `network.dns`

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    value: 203.0.113.10
    proxied: true
    ttl: 120
    comment: Managed by Composia
```

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `provider` | `string` | 是 | `cloudflare`、`alidns`、`dnspod`、`route53` 或 `huaweicloud`。 |
| `hostname` | `string` | 是 | DNS 主機名稱。 |
| `record_type` | `string` | 否 | 空白、`A`、`AAAA` 或 `CNAME`。 |
| `value` | `string` | 否 | DNS 記錄值。多節點服務應明確設定此項。 |
| `proxied` | `bool` | 否 | 提供者特定的代理開關，目前與 Cloudflare 相關。 |
| `ttl` | `uint32` | 否 | DNS TTL。 |
| `comment` | `string` | 否 | DNS 記錄備註。 |

## 映像檔更新

```yaml
update:
  enabled: true
  auto_apply: false
  check_schedule: "0 */6 * * *"
  backup_before_update: true
  digest_pin: false
  backup_data:
    - name: db
      enabled: true
  discovery_sources:
    upstream:
      sources:
        - type: github
          repo: owner/repo
      combine: first_success
      include_prerelease: false
  images:
    app:
      image: ghcr.io/example/app
      current:
        env:
          file: .env
          key: APP_VERSION
      discovery: upstream
      filter:
        type: semver
        allow:
          - patch
          - minor
```

### `update`

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `enabled` | `bool` | 為此服務啟用更新檢查。 |
| `auto_apply` | `bool` | 自動套用偵測到的更新。 |
| `check_schedule` | `string` | 更新檢查的 cron 排程。 |
| `backup_before_update` | `bool` | 在套用更新前執行備份。 |
| `backup_data` | `[]object` | 更新前要備份的受保護資料項目。 |
| `digest_pin` | `bool` | 以摘要固定映像檔。 |
| `discovery_sources` | `map[string]object` | 可重用的發現來源。具名來源不能引用另一個來源。 |
| `images` | `map[string]object` | 每個映像檔的更新定義。 |

### `update.backup_data[]`

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `name` | `string` | 受保護資料項目名稱。 |
| `enabled` | `bool` | 包含或排除此項目。 |

### `update.images.<name>`

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `image` | `string` | 是 | 映像檔存放庫。 |
| `auto_apply` | `bool` | 否 | 每個映像檔的自動套用覆蓋。 |
| `check_schedule` | `string` | 否 | 每個映像檔的檢查排程。 |
| `backup_before_update` | `bool` | 否 | 每個映像檔的備份開關。 |
| `digest_pin` | `bool` | 否 | 每個映像檔的摘要固定開關。 |
| `current` | `object` | 是 | 目前版本來源。 |
| `discovery` | `object` 或 `string` | 是 | 發現設定或具名發現來源引用。 |
| `filter` | `object` | 條件必要 | 除非發現為 `digest`，否則為必要項。 |

### `current`

恰好指定以下之一：

| 鍵 | 說明 |
|-----|-------------|
| `tag` | 靜態目前標籤。 |
| `env.file` + `env.key` | 從 env 檔案讀取目前標籤。`file` 必須是相對路徑且保持在服務目錄內。 |
| `yaml.file` + `yaml.path` | 從 YAML 檔案讀取目前標籤。`file` 必須是相對路徑且保持在服務目錄內。 |

### `discovery`

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `sources` | `[]object` | 至少一個來源。 |
| `combine` | `string` | 空白、`merge` 或 `first_success`。 |
| `include_prerelease` | `bool` | 包含預發行版本。 |

發現來源型別：

| 型別 | 必要鍵 | 備註 |
|------|---------------|-------|
| `auto` | 無 | `repo_url` 是可選的，若設定必須是有效 URL。必須是唯一來源。 |
| `probe` | 無 | 當存在過濾器時需要 `semver` 過濾器。 |
| `registry` | 無 | 登錄庫標籤發現。 |
| `digest` | 無 | 必須是唯一來源。必須省略 `filter`。 |
| `github` | `repo` | `repo` 為 `owner/repo`。 |
| `gitlab` | `project` | GitLab 專案 ID 或路徑。 |
| `forgejo` | `repo` | `repo` 為 `owner/repo`。 |

### `filter`

| 型別 | 必要鍵 | 備註 |
|------|---------------|-------|
| `semver` | 無 | `allow` 可包含 `patch`、`minor`、`major`。 |
| `date` | `format` | 用於解析標籤的日期格式。 |
| `regex` | `pattern`、`order` | `order` 必須為 `numeric` 或 `lexicographic`。 |
| `latest` | 無 | 使用最新的候選項。 |

## 資料保護

```yaml
data_protect:
  data:
    - name: db
      backup:
        strategy: database.pgdumpall
        service: postgres
      restore:
        strategy: database.pgimport
        service: postgres
    - name: uploads
      backup:
        strategy: files.copy_after_stop
        include:
          - ./uploads
      restore:
        strategy: files.copy
        include:
          - ./uploads
```

### `data_protect.data[]`

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 唯一的資料項目名稱。 |
| `backup` | `object` | 否 | 備份操作。 |
| `restore` | `object` | 否 | 還原操作。 |

### 資料操作

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `strategy` | `string` | 是 | `files.copy`、`files.copy_after_stop`、`database.pgdumpall` 或 `database.pgimport`。 |
| `service` | `string` | 條件必要 | `database.*` 策略的必要項。Compose 服務名稱。 |
| `include` | `[]string` | 條件必要 | `files.*` 策略的必要項。 |

## 備份

```yaml
backup:
  data:
    - name: db
      provider: rustic
      enabled: true
      schedule: "0 2 * * *"
```

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 必須引用具有備份操作的 `data_protect.data[].name`。 |
| `provider` | `string` | 否 | 備份提供者名稱。 |
| `enabled` | `bool` | 否 | 啟用或停用此備份項目。 |
| `schedule` | `string` | 否 | Cron 排程。 |

## 遷移

```yaml
migrate:
  data:
    - name: db
      enabled: true
```

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 必須引用同時具有備份和還原操作的 `data_protect.data[].name`。 |
| `enabled` | `bool` | 否 | 啟用或停用此項目的遷移。 |
