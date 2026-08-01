---
title: "密鑰"
date: '2026-05-26T00:00:00+08:00'
weight: 50
---

Composia 使用 age 加密管理期望狀態存放庫中的加密密鑰檔案。加密和解密在控制器端進行。代理從不接觸 age 私鑰。

## 設定

密鑰需要一組 age 金鑰對。在控制器設定中設定：

```yaml
controller:
  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
```

| 鍵 | 型別 | 必要 | 說明 |
|-----|------|----------|-------------|
| `provider` | `string` | 是 | 必須為 `age`。 |
| `identity_file` | `string` | 是 | age 私鑰檔案的路徑。 |
| `recipient_file` | `string` | 否 | 包含 age 接收者（公鑰）的檔案路徑。若省略，接收者從私鑰衍生。 |
| `armor` | `bool` | 否 | 使用 ASCII armoring 輸出。預設為 `true`。 |

產生金鑰對：

```bash
age-keygen -o age-identity.key
```

可選：提取公鑰作為接收者：

```bash
age-keygen -y age-identity.key > age-recipients.txt
```

## 密鑰的儲存方式

存放庫中的密鑰檔案慣例上具有 `.enc` 副檔名。它們以 age 加密的密文形式儲存：

```
my-app/
├── docker-compose.yaml
├── composia-meta.yaml
└── .secret.env.enc        (以 age 加密)
```

控制器在寫入時加密明文，在讀取時解密。存放庫中僅包含密文。密鑰永遠不會以明文形式出現在存放庫、任務日誌或傳輸給代理的過程中。

## 密鑰如何送達代理

在部署或更新任務的渲染步驟中，控制器：

1. 從存放庫中的服務目錄讀取加密檔案。
2. 使用 age 私鑰解密每個檔案。
3. 移除 `.enc` 副檔名，並以執行階段檔名注入解密內容。例如，`.secret.env.enc` 會成為 `.secret.env`。

服務包透過代理回報連線以串流方式傳送給代理。代理將服務包寫入磁碟，然後執行 `docker compose up`。解密的密鑰環境可供 Compose 服務使用，而代理從未見過私鑰。

`composia-meta.yaml` 中的檔案引用使用存放庫檔名，例如 `.env.enc`；Composia 在執行階段使用前將其轉換為 `.env`。Compose、Caddyfile、指令碼等原生檔案內部直接填寫執行階段檔名 `.env`。

## CLI 使用

寫入加密密鑰檔案：

```bash
composia service my-app edit .secret.env.enc
```

讀取並解密一個密鑰檔案：

```bash
composia repo get my-app/.secret.env.enc
```

原地編輯密鑰（開啟您的編輯器）：

```bash
composia repo update --file ./local-plain.env my-app/.secret.env.enc
```

所有密鑰寫入操作都包含基礎修訂版本檢查，以防止與並行變更發生衝突。

## 檔案路徑規則

密鑰檔案路徑必須：

- 相對於服務目錄（不能是絕對路徑）。
- 不包含路徑遍歷序列，如 `../`。
- 指向服務目錄內的檔案。

控制器定位服務，相對於服務目錄解析檔案路徑，並對存放庫檔案進行操作。

## 錯誤狀況

- **密鑰未設定**：當 `controller.secrets` 未設定時，Repo API 對 `.enc` 檔案的讀寫返回 `FailedPrecondition`。
- **檔案不存在**：`GetRepoFile` 返回 `NotFound`。
- **基礎修訂版本衝突**：`UpdateRepoFile` 使用 CAS（比較並交換）保護存放庫 HEAD。
