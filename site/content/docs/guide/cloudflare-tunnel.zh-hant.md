---
title: "Cloudflare Tunnel"
date: '2026-05-31T00:00:00+08:00'
weight: 25
---

Composia 可以為聲明了 `network.cloudflare_tunnel` 的服務管理遠端配置的 Cloudflare Tunnel 入口規則。隧道同步作為控制器端任務執行，因為 Cloudflare 的遠端隧道配置是全域狀態。

## 工作原理

當服務被部署、更新、停止或手動同步時，控制器會建立一個 `cloudflare_tunnel_sync` 任務。控制器工作器執行以下步驟：

1. 讀取任務儲存庫版本中的所有服務元資料。
2. 從聲明了 `network.cloudflare_tunnel` 的服務構建隧道入口規則。
3. 通過 `PUT /accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations` 將完整的入口列表傳送到 Cloudflare。
4. 確保每個主機名稱都有一個指向 `{tunnel_id}.cfargotunnel.com` 的代理 CNAME 記錄。

Cloudflare 要求一個萬用入口規則。Composia 預設追加 `http_status:404`。

## 控制器設定

隧道 ID 和 Cloudflare 憑證應放在控制器設定中，而不是服務元資料中：

```yaml
controller:
  cloudflare_tunnel:
    account_id: "REPLACE"
    api_token_file: /run/secrets/cloudflare-api-token
    tunnels:
      edge:
        tunnel_id: "c1744f8b-faa1-48a4-9e5c-02ac921467fa"
        fallback_service: http_status:404
    nodes:
      main:
        tunnel: edge
```

| 鍵 | 類型 | 必要 | 描述 |
|-----|------|----------|-------------|
| `account_id` | `string` | 是 | Cloudflare 帳戶 ID。 |
| `api_token` / `api_token_file` | `string` | 是 | 具有 Cloudflare Tunnel 設定和 DNS 寫入權限的 API 權杖。 |
| `tunnels` | `map` | 是 | 對應到 Cloudflare 隧道 ID 的隧道別名。 |
| `nodes` | `map` | 否 | 當服務未指定 `tunnel` 時使用的預設節點到隧道對應。 |

`tunnel_id` 不是連接器金鑰，但它仍然是控制器層級的基礎設施元資料。Cloudflared 連接器權杖或憑證應保留在 `cloudflared` 服務使用的節點/代理金鑰中。

## 服務聲明

在服務的 `composia-meta.yaml` 中聲明隧道入口：

```yaml
network:
  cloudflare_tunnel:
    hostname: app.example.com
    service: http://app:8080
    origin_request:
      no_tls_verify: false
      http_host_header: app.internal
```

| 鍵 | 類型 | 必要 | 描述 |
|-----|------|----------|-------------|
| `hostname` | `string` | 是 | 由 Cloudflare Tunnel 路由的公共主機名稱。 |
| `service` | `string` | 是 | `cloudflared` 使用的來源 URL，例如 `http://app:8080`。 |
| `tunnel` | `string` | 否 | 隧道別名。省略時，Composia 從目標節點對應中推導。 |
| `path` | `string` | 否 | 入口規則的可選路徑匹配器。 |
| `origin_request` | `object` | 否 | Cloudflare 來源參數。初始支援包括 `no_tls_verify`、`http_host_header`、`origin_server_name`、`connect_timeout` 和 `tls_timeout`。 |

## 隧道選擇

Composia 按以下規則為每個服務解析隧道：

1. 如果設定了 `network.cloudflare_tunnel.tunnel`，則使用該別名。
2. 如果服務只針對一個節點，Composia 使用 `controller.cloudflare_tunnel.nodes.<node>.tunnel`。
3. 如果服務針對多個節點且所有節點對應到同一個隧道，則使用該隧道。
4. 如果目標節點對應到不同的隧道，服務必須顯式設定 `network.cloudflare_tunnel.tunnel`。

## 停止行為

當已停止的服務聲明了 `network.cloudflare_tunnel` 時，後續的隧道同步會排除該服務並刪除其 CNAME 記錄。之後的同步只包含有執行中實例的服務，因此重新部署服務會將其重新加入。

## 手動同步

使用 CLI 同步服務的隧道設定：

```bash
composia service my-app tunnel-sync
```

這會將完整的已設定隧道狀態同步到 Cloudflare，而不僅僅是選定的服務，因為 Cloudflare 的遠端隧道設定是作為單個文件更新的。
