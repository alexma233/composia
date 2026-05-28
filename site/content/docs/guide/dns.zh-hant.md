---
title: "DNS"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Composia 管理宣告 `network.dns` 的服務的 DNS 記錄。DNS 更新以控制器端任務的形式執行。

## 運作方式

當服務部署完成或手動觸發 DNS 更新時，控制器會建立一個 `dns_update` 任務。控制器工作器執行該任務：

1. 讀取任務中記錄的存放庫修訂版本中的服務中繼資料。
2. 從 `network.dns` 建置期望的 DNS 記錄。
3. 將記錄同步到 DNS 提供者。

## 提供者設定

在控制器設定中至少設定一個 DNS 提供者。提供者憑證與區域清單是全域的：

```yaml
controller:
  dns:
    cloudflare:
      api_token: "REPLACE"
      zones:
        - "example.com"
        - "other.com"
```

支援五種提供者。每個都有各自的憑證鍵，並共用一個列出受管網域區域的 `zones` 欄位：

| 提供者 | 鍵前綴 | 憑證鍵 |
|----------|-----------|-----------------|
| `cloudflare` | `dns.cloudflare` | `api_token`、`api_token_file` |
| `alidns` | `dns.alidns` | `access_key_id`、`access_key_secret`、`region_id`、可選 `security_token` |
| `dnspod` | `dns.dnspod` | `secret_id`、`secret_key`、`region`、可選 `session_token` |
| `route53` | `dns.route53` | `access_key_id`、`secret_access_key`、`region`、可選 `session_token`、`profile`、`hosted_zone_id` |
| `huaweicloud` | `dns.huaweicloud` | `access_key_id`、`secret_access_key`、`region_id` |

每個憑證欄位都有對應的 `_file` 變體，用於從檔案讀取（例如 `api_token_file`）。

## 服務 DNS 宣告

在服務的 `composia-meta.yaml` 中宣告 DNS 設定：

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    value: 203.0.113.10
    proxied: true
    ttl: 120
    comment: "Managed by Composia"
```

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `provider` | `string` | 是 | `cloudflare`、`alidns`、`dnspod`、`route53` 或 `huaweicloud`。 |
| `hostname` | `string` | 是 | DNS 主機名稱。區域從已設定的區域清單中進行比對。 |
| `record_type` | `string` | 否 | `A`、`AAAA` 或 `CNAME`。為空白時，記錄型別從值或節點位址推斷。 |
| `value` | `string` | 否 | 明確的 DNS 記錄值。為空白時，Composia 從目標節點衍生該值。 |
| `proxied` | `bool` | 否 | 啟用 Cloudflare 代理。僅 Cloudflare 支援。 |
| `ttl` | `uint32` | 否 | DNS TTL，以秒為單位。 |
| `comment` | `string` | 否 | DNS 記錄備註。僅 Cloudflare 支援。 |

## 記錄解析

### 使用明確的值

當設定 `value` 時，Composia 直接使用它。如果它是 IP 位址，則記錄型別會被推斷：IPv4 為 `A`，IPv6 為 `AAAA`。如果它是主機名稱，記錄型別必須為 `CNAME`（或空白，同樣會解析為 `CNAME`）。

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    value: 203.0.113.10
```

### 從節點位址

當 `value` 為空白時，Composia 使用控制器設定中目標節點的 `public_ipv4` 和 `public_ipv6`：

```yaml
controller:
  nodes:
    - id: "main"
      public_ipv4: "203.0.113.10"
      public_ipv6: "2001:db8::10"
```

當 `record_type` 為空白且節點同時擁有兩個位址時，會建立 A 和 AAAA 兩筆記錄。若 `record_type` 為 `A`，則僅使用 IPv4 位址。若 `record_type` 為 `AAAA`，則僅使用 IPv6 位址。

目標超過一個節點的服務必須明確設定 `value`。多目標節點但 `value` 為空白會產生錯誤。

## 觸發 DNS 更新

DNS 記錄在部署任務流程中建立或更新。您也可以透過 Web UI 或 CLI 觸發獨立的 DNS 更新：

```bash
composia service dns-update my-app
```

這會建立一個 `dns_update` 任務。任務日誌會顯示區域解析、記錄操作與最終結果。

## Cloudflare 選項

當提供者為 `cloudflare` 時，`proxied` 和 `comment` 會在記錄建立後套用。Composia 會呼叫 Cloudflare API 以要求的代理狀態和備註修補每個 DNS 記錄。

非 Cloudflare 提供者不支援這些選項。對其他提供者設定 `proxied` 或 `comment` 會導致 DNS 更新失敗。

## 區域比對

Composia 將服務主機名稱與已設定的區域進行比對。區域從最長到最短的匹配順序嘗試。例如，使用 `zones: ["example.com.", "sub.example.com."]` 時，主機名稱 `app.sub.example.com` 會先匹配 `sub.example.com.`。

若沒有任何區域匹配主機名稱，則 DNS 更新失敗。

## 過期記錄清理

DNS 同步精確管理每個主機名稱的三種記錄型別：A、AAAA 和 CNAME。在設定新記錄之前，會先刪除期望狀態中不存在的任何已設定記錄型別。例如，如果服務先前有 `record_type: A` 而後改為 `record_type: CNAME`，則舊的 A 記錄會被移除並建立新的 CNAME 記錄。

變更服務的主機名稱不會清理舊主機名稱的記錄。如果您將 `app.example.com` 改名為 `api.example.com`，`app.example.com` 的記錄會保留在 DNS 提供者中，直到您手動移除為止。
