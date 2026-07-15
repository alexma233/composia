---
title: "備份"
date: '2026-05-26T00:00:00+08:00'
weight: 40
---

Composia 透過 Rustic 自動化備份。備份與還原任務在代理端執行，控制器則產生執行時期設定。

## 架構

備份需要一個 Rustic 基礎架構服務。存放庫中必須恰好宣告一個帶有 `infra.rustic` 的服務：

```yaml {filename="rustic/composia-meta.yaml"}
name: rustic
nodes:
  - main
infra:
  rustic:
    compose_service: rustic
    profile: default
    data_protect_dir: /data-protect
```

Rustic compose 服務是一個執行 `rustic` 二進位檔的普通 Docker 容器。它必須有一個磁碟區，將代理端的 `{StateDir}/data-protect` 對應到 `data_protect_dir` 設定的路徑。

## 控制器設定

```yaml
controller:
  backup:
    default_schedule: "0 2 * * *"
  rustic:
    main_nodes:
      - "main"
    maintenance:
      forget_schedule: "0 1 * * Sun"
      prune_schedule: "0 3 * * Sun"
```

| 鍵 | 說明 |
|-----|-------------|
| `backup.default_schedule` | 服務備份的預設 cron 排程。 |
| `rustic.main_nodes` | 執行 Rustic 操作的節點 ID。每個都必須引用已設定的節點。 |
| `rustic.maintenance.forget_schedule` | `rustic forget` 的 cron 排程。 |
| `rustic.maintenance.prune_schedule` | `rustic prune` 的 cron 排程。 |

## 服務資料保護

在 `composia-meta.yaml` 中的 `data_protect` 下定義要備份的內容：

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

### 資料策略

| 策略 | 用途 |
|----------|---------|
| `files.copy` | 將來源路徑以唯讀方式 bind-mount 到 Rustic 容器中備份。用於可即時讀取的資料。 |
| `files.copy_after_stop` | 停止 compose 專案，bind-mount 來源路徑，備份，然後重新啟動。用於需要靜止狀態的資料。 |
| `database.pgdumpall` | 在 compose 服務內執行 `pg_dumpall`。需要設定 `service`。 |
| `database.pgimport` | 透過 `psql` 還原 PostgreSQL 傾印。需要設定 `service`。 |

### 資料操作欄位

| 鍵 | 型別 | 對以下項目為必要 | 說明 |
|-----|------|-------------|-------------|
| `strategy` | `string` | 全部 | 備份或還原策略。 |
| `service` | `string` | `database.*` | Compose 服務名稱。 |
| `include` | `[]string` | `files.*` | 要包含的路徑。服務路徑（相對於服務根目錄，以 `./` 開頭或包含 `/`）或 Docker 磁碟區名稱（不含路徑分隔符的純名稱）。 |

### 包含路徑型別

路徑可以引用：

- **服務路徑**：服務目錄內的檔案或目錄。透過 `-v` 以唯讀方式 bind-mount 到 Rustic 容器中。
- **具名磁碟區**：Docker 磁碟區名稱。透過 `-v` 以唯讀方式 bind-mount 到 Rustic 容器中（無需臨時容器）。

## 備份排程

為受保護的資料項目啟用排程備份：

```yaml
backup:
  data:
    - name: db
      provider: rustic
      enabled: true
      schedule: "0 2 * * *"
    - name: uploads
      enabled: true
      schedule: "0 3 * * Sun"
```

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 必須引用具有備份操作的 `data_protect.data[].name`。 |
| `provider` | `string` | 否 | 備份提供者名稱。 |
| `enabled` | `bool` | 否 | 啟用或停用此備份。 |
| `schedule` | `string` | 否 | Cron 表達式。`"none"` 停用排程但保留項目。 |

當設定 `schedule` 時，控制器會排程週期性的 `backup` 任務。若服務項目未指定自己的排程，則使用控制器的 `backup.default_schedule` 作為後備。

## 備份執行方式

備份任務在代理端執行以下步驟：

1. **渲染**：從控制器下載服務包與 Rustic 包。讀取控制器產生的 `.composia-backup.json`。
2. **備份**：針對執行時期設定中的每個資料項目：
   - `files.*`: 在 `data-protect` 下建立空暫存目錄，為每個 include 路徑或磁碟區加入 `-v` bind mount 參數，然後執行 `docker compose run -v ... rustic backup` 並加上識別服務與資料項目的標籤。資料不會被複製到代理端的 state 目錄中。
   - `database.pgdumpall`: 執行 `docker compose exec <service> pg_dumpall`，將 SQL 傾印寫入 `data-protect` 下的暫存檔案，然後執行 `docker compose run rustic backup` 備份暫存目錄。
   - 將結果（快照 ID）回報給控制器。
3. 所有項目備份完成後，任務結束。

備份成品以 Rustic 快照 ID 識別。標籤包含 `composia-service:<name>` 和 `composia-data:<name>`，用於後續的還原與 forget 操作。

## 還原

透過 Web UI 從備份頁面觸發還原，或使用 CLI：

```bash
composia backup restore --wait --follow --timeout 30m main <backup-id>
```

The first argument is the target node. Use `--wait --follow` to block until the restore finishes and stream task logs.

還原流程：

1. **渲染**：下載服務包與 Rustic 包。讀取 `.composia-restore.json`。
2. **還原**：針對每個項目：
   - `files.copy` / `files.copy_after_stop`: 清理還原目標（目標必須已存在），在 `data-protect` 下建立空暫存目錄，將每個目標路徑或 Docker 磁碟區 bind-mount 到暫存目錄樹中，然後執行 `docker compose run -v ... rustic restore <snapshot_id> <staging_dir>`。還原的資料直接寫入目標位置——無需還原後的複製步驟。
   - `files.copy_after_stop`: 額外在還原前停止 compose 專案，還原後重新啟動。
   - `database.pgimport`: 執行 `docker compose run rustic restore <snapshot_id>` 還原到暫存目錄，然後使用還原的 SQL 傾印執行 `docker compose exec <service> psql`。

`files.*` 策略還原時，服務路徑目標必須先存在於代理端上（用於判斷 bind-mount 的檔案/目錄語義）。Docker 磁碟區目標會在還原前被清空。

## Rustic 維護

維護任務使用 Rustic 基礎架構服務：

- **`rustic_init`**：執行 `docker compose run rustic init` 以初始化 Rustic 存放庫。每個 Rustic 設定使用一次。
- **`rustic_forget`**：執行 `docker compose run rustic forget` 並套用標籤過濾。可限定範圍為服務、資料項目或整個存放庫。
- **`rustic_prune`**：執行 `docker compose run rustic prune` 以移除未引用的資料。

從 Web UI 或 CLI 觸發維護：

```bash
composia rustic init --wait --follow main
composia rustic forget --service my-app --data uploads --wait --follow main
composia rustic prune --wait --follow main
```

Use `--wait --follow` when you want the CLI to wait for the maintenance task and stream logs.

## 參見

- [服務設定](/docs/guide/service/) — 資料保護與備份排程。
- [遷移](/docs/guide/migrate/) — 在節點間移動服務，透過備份保留資料。
