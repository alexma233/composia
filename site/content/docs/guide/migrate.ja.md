---
title: "移行"
date: '2026-05-26T00:00:00+08:00'
weight: 45
---

データの整合性を保ったまま、あるノードから別のノードにサービスを移行します。移行タスクはソースノードとターゲットノード間でバックアップ、停止、リストア、起動、DNS 更新の各ステップを調整します。

## 設定

移行中に引き継がれるデータ項目は、`data_protect` に `backup` と `restore` の両方のアクションを持つ必要があります。`migrate` で宣言します:

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

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `name` | `string` | はい | バックアップとリストアの両方のアクションを持つ `data_protect.data[].name` を参照する必要があります。 |
| `enabled` | `bool` | いいえ | この項目の移行を有効または無効にします。 |

## 移行の実行

**Web UI:**
1. サービス詳細ページを開きます。
2. 移行コントロールを使用してソースノードとターゲットノードを選択します。
3. **移行** をクリックします。

**CLI:**

```bash
composia service my-app migrate --source main --target edge-1 --wait --follow --timeout 30m
```

## 移行手順

1. **データのエクスポート** — 設定された各データ項目について、ソースノードでバックアップタスクを実行します。
2. **ソースインスタンスの停止** — `docker compose down` を実行し、Caddy 設定を削除します。
3. **ソースでの Caddy リロード** — ソース Caddy インスタンスからプロキシエントリを削除します。
4. **ターゲットでのデータリストア** — 各データ項目について、ターゲットノードでリストアタスクを実行します。
5. **ターゲットでのデプロイ** — `docker compose up -d` を実行し、Caddy 設定を同期します。
6. **ターゲットでの Caddy リロード** — ターゲット Caddy インスタンスにプロキシエントリを適用します。
7. **DNS の更新** — DNS レコードをターゲットノードに向けるよう更新します。
8. **設定の書き込み** — `composia-meta.yaml` の `nodes` を更新し、Git にコミットします。

## 注意点

- サービスはソースノードにデプロイされている必要があり、ターゲットノードはオンラインである必要があります。
- 移行には短時間のダウンタイムが発生します。オフピーク時間に実行してください。
- 整合性を確保するため、データ転送前にソースインスタンスが停止されます。
- データベースの場合はエクスポート戦略（`database.pgdumpall` / `database.pgimport`）を使用してください。

## Rollback

State rollback is currently available in the Web UI only. Open the migration task details, choose the recovery actions that match the failed step, and start rollback there.

| Action | Description |
|--------|-------------|
| `deploy_source` | Redeploy the service on the original source node. |
| `stop_target` | Stop and clean up the service on the target node. |
| `rollback_dns` | Sync DNS records back to the source node. |

The CLI does not have a `task rollback` command yet. You can still inspect and follow the migration task with:

```bash
composia task wait --follow --timeout 30m <task-id>
```

## 関連項目

- [バックアップ](/docs/guide/backups/) — Rustic のセットアップとバックアップ設定。
- [サービス設定](/docs/guide/service/) — `data_protect` と `migrate` フィールドリファレンス。
