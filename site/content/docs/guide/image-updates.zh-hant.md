---
title: "映像檔更新"
date: '2026-05-26T00:00:00+08:00'
weight: 60
---

Composia 偵測新的映像檔標籤並可自動套用更新。映像檔檢查任務在代理端執行並將結果回報給控制器。

## 運作方式

控制器根據服務的更新設定排程週期性的 `image_check` 任務。每次檢查：

1. 代理下載服務包。
2. 讀取 `docker compose config --format json` 以發現執行中的映像檔。
3. 回報每個映像檔的本機與遠端摘要。
4. 對於在 `update.images` 中設定的映像檔，使用已設定的發現來源檢查新的候選標籤。
5. 將結果回報給控制器。控制器記錄可用的更新並可自動套用。

## 控制器預設值

全域預設值在控制器設定中設定：

```yaml
controller:
  updates:
    default_check_schedule: "0 */6 * * *"
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
        token: "REPLACE"
        api_url: "https://api.github.com"
```

服務層級的 `update` 區段會覆蓋這些預設值。

## 服務設定

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
    upstream-gh:
      sources:
        - type: github
          repo: owner/repo
      combine: first_success
      include_prerelease: false
  images:
    api:
      image: ghcr.io/example/api
      current:
        env:
          file: .env
          key: API_VERSION
      discovery: upstream-gh
      filter:
        type: semver
        allow:
          - patch
          - minor
```

### `update` 頂層

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `enabled` | `bool` | 為此服務啟用更新檢查。 |
| `auto_apply` | `bool` | 自動套用偵測到的更新。 |
| `check_schedule` | `string` | 更新檢查的 cron 排程。 |
| `backup_before_update` | `bool` | 在套用更新前執行備份。 |
| `backup_data` | `[]object` | 更新前要備份的受保護資料項目。每個項目都有 `name` 和可選的 `enabled`。 |
| `digest_pin` | `bool` | 以摘要固定映像檔，確保可重現性。 |
| `discovery_sources` | `map[string]object` | 具名的可重用發現設定。 |
| `images` | `map[string]object` | 每個映像檔的更新設定。鍵為任意名稱，對應要檢查的映像檔。 |

### `images.<name>`

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `image` | `string` | 是 | 完整映像檔引用，例如 `ghcr.io/example/api`。 |
| `auto_apply` | `bool` | 否 | 每個映像檔的自動套用覆蓋。 |
| `check_schedule` | `string` | 否 | 每個映像檔的檢查排程。 |
| `backup_before_update` | `bool` | 否 | 每個映像檔的備份開關。 |
| `digest_pin` | `bool` | 否 | 每個映像檔的摘要固定開關。 |
| `current` | `object` | 是 | 如何找到目前部署的版本。 |
| `discovery` | `object` 或 `string` | 是 | 發現設定或對具名 `discovery_sources` 項目的引用。 |
| `filter` | `object` | 條件必要 | 版本過濾器。除非發現模式為 `digest`，否則為必要項。 |

### `current`

必須恰好指定以下來源之一：

**靜態標籤：**

```yaml
current:
  tag: "v1.2.3"
```

**環境檔案：**

```yaml
current:
  env:
    file: .env
    key: APP_VERSION
```

`file` 路徑相對於服務目錄。Composia 讀取該檔案，尋找 `KEY=VALUE` 行，並提取該值。

**YAML 檔案：**

```yaml
current:
  yaml:
    file: values.yaml
    path: app.image.tag
```

`path` 是以點分隔的 YAML 文件樹路徑。該路徑上的值必須為純量。

### 發現

發現來源可以是：

**具名引用**，指向 `discovery_sources` 項目：

```yaml
discovery: upstream-gh
```

**內聯定義：**

```yaml
discovery:
  sources:
    - type: probe
  combine: first_success
  include_prerelease: false
```

發現來源型別：

| 型別 | 必要鍵 | 行為 |
|------|---------------|----------|
| `probe` | 無 | Semver 探測：透過探測登錄庫清單來搜尋更高版本。需要 `semver` 過濾器。 |
| `registry` | 無 | 列出映像檔登錄庫中的所有標籤。 |
| `auto` | 無（可選 `repo_url`） | 嘗試 `probe` 再嘗試 `registry`，作為合併的發現方式。必須是發現設定中的唯一來源。 |
| `digest` | 無 | 僅比較遠端摘要與本地摘要。不進行標籤比較。必須省略 `filter`。必須是唯一來源。 |
| `github` | `repo` (`owner/repo`) | 查詢 GitHub 發行版本。在控制器端處理。 |
| `gitlab` | `project` | 查詢 GitLab 發行版本。在控制器端處理。 |
| `forgejo` | `repo` (`owner/repo`) | 查詢 Forgejo 發行版本。在控制器端處理。 |

`combine` 接受 `merge`（所有來源結果的聯集）或 `first_success`（首次返回結果的來源獲勝）。

`include_prerelease` 在 GitHub、GitLab 和 Forgejo 的發行版本查詢中包含預發行版本。

### 過濾器

| 型別 | 必要鍵 | 行為 |
|------|---------------|----------|
| `semver` | 無 | 按語意化版本過濾。`allow` 可包含 `patch`、`minor`、`major`。 |
| `date` | `format` | 使用指定的格式將標籤解析為日期。 |
| `regex` | `pattern`、`order` | 按正則表達式過濾。`order` 必須為 `numeric` 或 `lexicographic`。 |
| `latest` | 無 | 取最新標籤，不進行過濾。 |

#### Semver 探測

使用 `type: probe` 和 `semver` 過濾器時，Composia 透過構造版本號並檢查對應的登錄庫清單是否存在來搜尋候選標籤。它會根據 `allow` 清單探測修補、次要和主要版本的升級，使用指數搜尋配合二分法精煉，以找到最高可用版本。

## 摘要模式

當設定中的所有發現來源均為 `type: digest` 時，不進行標籤比較。Composia 僅比較遠端映像檔摘要與本地摘要：

```yaml
discovery:
  sources:
    - type: digest
```

當發現模式設為 `digest` 時，必須省略 `filter`。如果摘要不同，則視為有可用更新。

## 映像檔觀察

在部署和更新任務期間，代理也會收集所有 compose 服務的映像檔觀察。這些包括本地與遠端摘要，無論是否設定了 `update.images` 都會回報給控制器。這提供了 Web UI 和 CLI 中的映像檔狀態可見性。
