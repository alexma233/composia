---
title: "本地開發"
date: '2026-07-06T00:00:00+08:00'
weight: 10
---

使用 `mise` 作為本地開發的統一入口。

## 設定

```bash
mise install
mise run setup
```

`mise run setup` 會準備 `dev/` 下的本地檔案、建立開發權杖和 age 金鑰檔案，並在需要時初始化控制器存放庫。現有本地檔案不會被覆寫。

## 應用程式堆疊

啟動控制器、代理和 Web UI：

```bash
mise run dev
```

- 控制器：<http://127.0.0.1:7001>
- Web UI：<http://127.0.0.1:5173>

檢視日誌或停止堆疊：

```bash
mise run dev:logs
mise run dev:down
```

## 文件

預設情況下，文件不會隨應用程式堆疊啟動。

```bash
mise run dev:docs # 僅文件，監聽 :5174
mise run dev:all  # 應用程式和文件
```

## 檢查

提交前執行標準本地檢查：

```bash
mise run check
```

需要時執行較慢的後端檢查：

```bash
mise run check:full
```

## Protobuf 產生

修改 `proto/**` 後，重新產生已納入版本控制的 Protobuf 輸出和 API 文件：

```bash
mise run gen
```

提交 `gen/go/`、`web/src/lib/gen/` 和 `site/content/docs/developer-guide/api/` 下的產生檔案。

## E2E

使用實際的控制器和代理測試堆疊執行全部本地 E2E 測試：

```bash
mise run e2e
```
