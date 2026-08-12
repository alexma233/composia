---
title: "ローカル開発"
date: '2026-07-06T00:00:00+08:00'
weight: 10
---

ローカル開発の共通エントリーポイントとして `mise` を使用します。

## セットアップ

```bash
mise install
mise run setup
```

`mise run setup` は `dev/` 以下のローカルファイル、開発用トークン、age 鍵ファイルを準備し、必要に応じてコントローラーリポジトリを初期化します。既存のローカルファイルは上書きしません。

## アプリスタック

コントローラー、エージェント、Web UI を起動します：

```bash
mise run dev
```

- コントローラー：<http://127.0.0.1:7001>
- Web UI：<http://127.0.0.1:5173>

ログの追跡またはスタックの停止：

```bash
mise run dev:logs
mise run dev:down
```

## ドキュメント

デフォルトでは、ドキュメントはアプリスタックと同時に起動しません。

```bash
mise run dev:docs # :5174 でドキュメントのみ起動
mise run dev:all  # アプリとドキュメントを起動
```

## チェック

コミット前に標準のローカルチェックを実行します：

```bash
mise run check
```

必要に応じて時間のかかるバックエンドチェックを実行します：

```bash
mise run check:full
```

## Protobuf 生成

`proto/**` の変更後、追跡対象の Protobuf 出力と API ドキュメントを再生成します：

```bash
mise run gen
```

`gen/go/`、`web/src/lib/gen/`、`site/content/docs/developer-guide/api/` の生成ファイルをコミットします。

## E2E

実際のコントローラーとエージェントのフィクスチャスタックに対して、すべてのローカル E2E テストを実行します：

```bash
mise run e2e
```
