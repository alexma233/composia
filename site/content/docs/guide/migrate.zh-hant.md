---
title: "遷移"
date: '2026-05-26T00:00:00+08:00'
weight: 45
---

將服務從一個節點遷移到另一個節點，同時保持資料完整性。遷移任務協調跨來源和目標節點的備份、停止、還原、啟動和 DNS 更新步驟。

## 設定

遷移期間攜帶的資料項目必須在 `data_protect` 中同時具有 `backup` 和 `restore` 操作。在 `migrate` 中宣告它們：

```yaml
name: my-app
nodes:
  - main

data_protect:
  data:
    - name: uploads
      backup:
        strategy: files.copy
        include:
          - ./data/uploads
      restore:
        strategy: files.copy
        include:
          - ./data/uploads

migrate:
  data:
    - name: uploads
```

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 必須引用同時具有備份和還原操作的 `data_protect.data[].name`。 |
| `enabled` | `bool` | 否 | 啟用或停用此項目的遷移。 |

## 執行遷移

**Web UI：**
1. 開啟服務詳細頁面。
2. 使用遷移控制項選擇來源和目標節點。
3. 點擊 **遷移**。

**CLI：**

```bash
composia service my-app migrate --source main --target edge-1 --wait --follow --timeout 30m
```

## 遷移步驟

1. **匯出資料** — 在來源節點上為每個已設定的資料項目執行備份任務。
2. **停止來源執行實例** — 執行 `docker compose down`，移除 Caddy 設定。
3. **在來源端重新載入 Caddy** — 從來源 Caddy 執行實例中移除代理項目。
4. **在目標端還原資料** — 在目標節點上為每個資料項目執行還原任務。
5. **在目標端部署** — 執行 `docker compose up -d`，同步 Caddy 設定。
6. **在目標端重新載入 Caddy** — 在目標 Caddy 執行實例上套用代理項目。
7. **更新 DNS** — 更新 DNS 記錄以指向目標節點。
8. **寫入設定** — 更新 `composia-meta.yaml` 中的 `nodes`，提交到 Git。

## 注意事項

- 服務必須部署在來源節點上，且目標節點必須在線。
- 遷移會造成短暫停機。請在離峰時段執行。
- 為確保一致性，資料傳輸前會先停止來源執行實例。
- 對於資料庫，使用匯出策略（`database.pgdumpall` / `database.pgimport`）。

## Rollback

State rollback is currently available in the Web UI only. Open the migration task details, choose the recovery actions that match the failed step, and start rollback there.

| Action | Description |
|--------|-------------|
| `deploy_source` | Redeploy the service on the original source node. |
| `stop_target` | Stop and clean up the service on the target node. |
| `rollback_dns` | Sync DNS records back to the source node. |

The CLI does not have a `task rollback` command yet. You can still inspect and follow the migration task with:

```bash
composia task wait <task-id> --follow --timeout 30m
```

## 參見

- [備份](/docs/guide/backups/) — Rustic 設定與備份設定。
- [服務設定](/docs/guide/service/) — `data_protect` 和 `migrate` 欄位參考。
