---
title: "バックアップ"
date: '2026-05-26T00:00:00+08:00'
weight: 40
---

Composia は Rustic を通じてバックアップを自動化します。バックアップとリストアのタスクはエージェント上で実行され、コントローラーがランタイム設定を生成します。

## アーキテクチャ

バックアップには Rustic インフラストラクチャサービスが必要です。リポジトリには `infra.rustic` を持つサービスを 1 つだけ宣言する必要があります:

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

Rustic compose サービスは `rustic` バイナリを実行する通常の Docker コンテナです。データ保護ディレクトリ用のボリュームが必要です。

## コントローラー設定

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

| キー | 説明 |
|-----|-------------|
| `backup.default_schedule` | サービスバックアップのデフォルト cron スケジュール。 |
| `rustic.main_nodes` | Rustic 操作を実行するノード ID。それぞれ設定されたノードを参照する必要があります。 |
| `rustic.maintenance.forget_schedule` | `rustic forget` の cron スケジュール。 |
| `rustic.maintenance.prune_schedule` | `rustic prune` の cron スケジュール。 |

## サービスデータ保護

`composia-meta.yaml` の `data_protect` でバックアップ対象を定義します:

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

### データ戦略

| 戦略 | 目的 |
|----------|---------|
| `files.copy` | ファイルとディレクトリをコピー。読み取り中のデータに使用します。 |
| `files.copy_after_stop` | compose プロジェクトを停止し、ファイルをコピーしてから再起動。静止状態が必要なデータに使用します。 |
| `database.pgdumpall` | compose サービス内で `pg_dumpall` を実行。`service` の設定が必須です。 |
| `database.pgimport` | `psql` で PostgreSQL ダンプをリストア。`service` の設定が必須です。 |

### データアクションフィールド

| キー | 型 | 必須 | 説明 |
|-----|------|-------------|-------------|
| `strategy` | `string` | すべて | バックアップまたはリストアの戦略。 |
| `service` | `string` | `database.*` | Compose サービス名。 |
| `include` | `[]string` | `files.*` | 含めるパス。サービスディレクトリからの相対パス。サービスルート内に留まります。 |

### include パスの種類

パスは以下を参照できます:

- **サービスパス**: サービスディレクトリ内のファイルまたはディレクトリ。直接コピーされます。
- **名前付きボリューム**: Docker ボリューム名。ボリュームをマウントする一時コンテナを起動してバックアップします。

## バックアップスケジュール

保護されたデータ項目のスケジュールバックアップを有効にします:

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

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `name` | `string` | はい | バックアップアクションを持つ `data_protect.data[].name` を参照する必要があります。 |
| `provider` | `string` | いいえ | バックアッププロバイダー名。 |
| `enabled` | `bool` | いいえ | このバックアップを有効または無効にします。 |
| `schedule` | `string` | いいえ | cron 式。`"none"` はエントリを保持したままスケジュールを無効にします。 |

`schedule` が設定されている場合、コントローラーは定期的な `backup` タスクをスケジュールします。サービスエントリが独自のスケジュールを指定していない場合、コントローラーの `backup.default_schedule` がフォールバックとして使用されます。

## バックアップの実行方法

バックアップタスクはエージェント上で以下のステップを実行します:

1. **レンダリング**: コントローラーからサービスバンドルと Rustic バンドルをダウンロード。コントローラーが生成した `.composia-backup.json` を読み取ります。
2. **バックアップ**: ランタイム設定の各データ項目について:
   - バックアップ戦略（`files.copy`、`files.copy_after_stop`、`database.pgdumpall`）に従ってデータをステージングします。
   - サービスとデータ項目を識別するタグを付けて `docker compose run rustic backup` を実行します。
   - 結果（スナップショット ID）をコントローラーに報告します。
3. すべての項目がバックアップされたらタスクが完了します。

バックアップ成果物は Rustic スナップショット ID で識別されます。タグには後続のリストアや forget 操作用に `composia-service:<name>` と `composia-data:<name>` が含まれます。

## リストア

Web UI のバックアップページから、または CLI でリストアをトリガーします:

```bash
composia backup restore <backup-id>
```

リストアプロセス:

1. **レンダリング**: サービスバンドルと Rustic バンドルをダウンロード。`.composia-restore.json` を読み取ります。
2. **リストア**: 各項目について:
   - `docker compose run rustic restore <snapshot_id> <target_dir>` を実行します。
   - リストア戦略に従って復元されたデータを適用します:
     - `files.copy`: サービスディレクトリのファイルを置き換えます。
     - `files.copy_after_stop`: compose を停止し、ファイルを置き換え、compose を再起動します。
     - `database.pgimport`: 復元された SQL ダンプで `docker compose exec <service> psql` を実行します。

## Rustic メンテナンス

メンテナンスタスクは Rustic インフラストラクチャサービスを使用します:

- **`rustic_init`**: `docker compose run rustic init` を実行して Rustic リポジトリを初期化します。Rustic セットアップごとに 1 回使用します。
- **`rustic_forget`**: タグフィルター付きで `docker compose run rustic forget` を実行します。サービス、データ項目、またはリポジトリ全体にスコープを設定できます。
- **`rustic_prune`**: `docker compose run rustic prune` を実行して参照されていないデータを削除します。

Web UI または CLI からメンテナンスをトリガーします:

```bash
composia node init-rustic main
composia node forget-rustic main
composia node prune-rustic main
```

## 関連項目

- [サービス設定](/docs/guide/service/) — データ保護とバックアップスケジュール。
- [移行](/docs/guide/migrate/) — バックアップを通じてデータを保持したままノード間でサービスを移動。
