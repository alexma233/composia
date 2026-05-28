---
title: "安裝"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Composia 有四個執行時期二進位檔和映像檔：

| 元件 | 用途 |
|-----------|---------|
| `composia-controller` | 執行 API、任務佇列、期望狀態 Git 存放庫以及控制器端整合。 |
| `composia-agent` | 在每個 Docker 節點上執行 Docker Compose 操作。 |
| `composia-web` | 與控制器通訊的瀏覽器 UI。 |
| `composia` | 用於終端機、腳本和自動化的 CLI。 |

## 選擇安裝方式

| 方式 | 最適合 |
|--------|----------|
| [Docker Compose](docker-compose/) | 快速的整合式部署，包含控制器、本地代理和 Web UI。 |
| [套件管理器與二進位檔](package-managers/) | 非容器安裝、OS 套件、Nix、AUR 及手動壓縮檔。 |
| [設定](configuration/) | 設定檔、Web 環境變數、age 金鑰設定與完整全域設定參考。 |

如需原始碼建置，請參見[開發者指南：原始碼建置](/docs/developer-guide/source-build/)。
