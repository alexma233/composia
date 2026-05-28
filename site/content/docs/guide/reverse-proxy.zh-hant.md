---
title: "反向代理"
date: '2026-05-26T00:00:00+08:00'
weight: 30
---

Composia 與 Caddy 整合以管理反向代理。Caddy 基礎架構服務作為普通的 Docker Compose 服務執行，Composia 在部署和停止時同步 Caddy 設定檔。

## 架構

```
Controller repo
  ├── caddy/
  │   ├── docker-compose.yaml   (Caddy Compose 服務)
  │   ├── Caddyfile             (主要 Caddy 設定，匯入產生的檔案)
  │   └── composia-meta.yaml    (宣告 infra.caddy)
  ├── my-app/
  │   ├── docker-compose.yaml
  │   ├── Caddyfile             (服務專用的 Caddy 設定)
  │   └── composia-meta.yaml    (宣告 network.caddy)
  └── ...
```

在部署時，Composia 將每個服務的 Caddyfile 複製到產生的目錄中，然後觸發 Caddy 重新載入。

## 基礎架構設定

在存放庫中宣告恰好一個 Caddy 基礎架構服務：

```yaml {filename="caddy/composia-meta.yaml"}
name: caddy
nodes:
  - main
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
```

Caddy 服務目錄中的主要 Caddyfile 應匯入產生的檔案：

```caddy {filename="caddy/Caddyfile"}
import /etc/caddy/generated/*.caddy
```

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `compose_service` | `string` | Compose 服務名稱。預設為 `caddy`。 |
| `config_dir` | `string` | 容器內的 Caddy 設定目錄。預設為 `/etc/caddy`。 |

存放庫中只有一個服務可以宣告為 Caddy 基礎架構。

## 服務設定

對於需要反向代理項目的每個服務，在 `composia-meta.yaml` 中啟用 Caddy 並提供一個 Caddyfile：

```yaml {filename="my-app/composia-meta.yaml"}
name: my-app
nodes:
  - main
network:
  caddy:
    enabled: true
    source: Caddyfile
```

`source` 路徑相對於服務目錄，且必須保持在該目錄內。檔案可以任意命名，但慣例上使用 `Caddyfile`。

```caddy {filename="my-app/Caddyfile"}
app.example.com {
    reverse_proxy app:8080
}
```

## 同步運作方式

在部署或更新任務期間，代理在 `compose up` 之後執行 Caddy 同步步驟：

1. 從服務的 `composia-meta.yaml` 讀取 `network.caddy.source`。
2. 將來源檔案複製到 `<agent_state_dir>/caddy/generated/<service_dir>.caddy`。
3. 執行 `docker compose exec <caddy_service> caddy reload --config <Caddyfile> --adapter caddyfile`。

產生的檔案名稱來自服務目錄名稱。對於 `my-app`，檔案為 `my-app.caddy`。

在停止任務期間，產生的 Caddy 檔案會被移除。

## Caddy 同步任務

獨立的 `caddy_sync` 任務在不部署服務的情況下重建 Caddy 設定。它可以以兩種模式運作：

**完全重建** (`full_rebuild: true`)：從產生的目錄中刪除所有產生的 `.caddy` 檔案，然後重新同步所有由 Caddy 管理的服務。

**定向同步**：僅同步指定的服務目錄。

透過 Web UI 或 CLI 觸發：

```bash
composia service caddy-sync my-app
```

## Caddy 重新載入任務

`caddy_reload` 任務在 Caddy 容器內執行 `caddy reload`，不變更任何檔案。在手動編輯主要 Caddyfile 後使用：

```bash
composia node reload-caddy main
```

## 代理設定

代理設定有一個可選的 Caddy 區段：

```yaml
agent:
  caddy:
    generated_dir: "/data/state-agent/caddy/generated"
```

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `generated_dir` | `string` | 產生的 Caddy 設定目錄。預設為 `<state_dir>/caddy/generated`。 |

產生的目錄必須位於 Caddy 容器可以讀取的路徑內。Caddy compose 服務必須有一個將此目錄掛載到主要 Caddyfile 中所匯入路徑的磁碟區。
