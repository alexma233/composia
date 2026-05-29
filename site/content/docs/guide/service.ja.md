---
title: "サービス設定"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

各サービスはコントローラーリポジトリ内のディレクトリに存在します。サービスディレクトリには `composia-meta.yaml` と 1 つ以上の Docker Compose ファイルが含まれます。

最小限のサービス:

```yaml {filename="composia-meta.yaml"}
name: my-app
nodes:
  - main
```

デフォルトの動作では、Composia は同じディレクトリ内の `docker-compose.yaml` を探します。

## トップレベルキー

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `name` | `string` | はい | 一意のサービス名。 |
| `project_name` | `string` | いいえ | Docker Compose プロジェクト名の上書き。デフォルトは正規化されたサービス名。 |
| `compose_files` | `[]string` | いいえ | サービスディレクトリからの相対 Compose ファイルパス。 |
| `enabled` | `bool` | いいえ | サービスがアクティブかどうか。デフォルトは `true`。 |
| `nodes` | `[]string` | はい | ターゲットノード ID。それぞれ `controller.nodes` に存在する必要があります。 |
| `infra` | `object` | いいえ | このサービスを Caddy、Rustic、または config-only インフラストラクチャとして宣言します。 |
| `network` | `object` | いいえ | Caddy と DNS の設定。 |
| `update` | `object` | いいえ | イメージ更新設定。 |
| `data_protect` | `object` | いいえ | バックアップとリストアのデータ定義。 |
| `backup` | `object` | いいえ | 保護データのスケジュールバックアップ。 |
| `migrate` | `object` | いいえ | 移行が有効な保護データ。 |
| `auto_deploy` | `bool` | いいえ | リポジトリ変更後にこのサービスを自動デプロイします。 |

`compose_files` エントリは相対パスで、サービスディレクトリ内に留まり、重複してはいけません。

## インフラストラクチャサービス

### `infra.caddy`

リポジトリの Caddy インフラストラクチャサービスを宣言します。

```yaml
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
```

| キー | 型 | 説明 |
|-----|------|-------------|
| `compose_service` | `string` | Compose サービス名。デフォルトは `caddy`。 |
| `config_dir` | `string` | Caddy 設定ディレクトリ。デフォルトは `/etc/caddy`。 |

Caddy インフラストラクチャとして宣言できるサービスは 1 つだけです。

### `infra.rustic`

リポジトリの Rustic インフラストラクチャサービスを宣言します。

```yaml
infra:
  rustic:
    compose_service: rustic
    profile: default
    data_protect_dir: /data-protect
    init_args:
      - --set-version
      - "2"
```

| キー | 型 | 説明 |
|-----|------|-------------|
| `compose_service` | `string` | Compose サービス名。デフォルトは `rustic`。 |
| `profile` | `string` | Rustic プロファイル名。 |
| `data_protect_dir` | `string` | エージェントの `{StateDir}/data-protect` にマッピングされるコンテナ内パス。 |
| `init_args` | `[]string` | `rustic init` に渡される追加引数。空のエントリは拒否されます。 |

Rustic インフラストラクチャとして宣言できるサービスは 1 つだけです。

### `infra.config`

config-only インフラストラクチャサービスを宣言します。

```yaml
infra:
  config: {}
```

Config-only サービスは `infra.caddy` または `infra.rustic` と組み合わせることはできません。その `data_protect` アクションは `files.copy` のみ使用できます。

## ネットワーク

### `network.caddy`

```yaml
network:
  caddy:
    enabled: true
    source: Caddyfile
```

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `enabled` | `bool` | いいえ | Caddy 管理を有効にします。デフォルトは `false`。 |
| `source` | `string` | 条件付き | サービスディレクトリからの相対 Caddyfile パス。有効時に必須。 |

### `network.dns`

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    value: 203.0.113.10
    proxied: true
    ttl: 120
    comment: Managed by Composia
```

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `provider` | `string` | はい | `cloudflare`、`alidns`、`dnspod`、`route53`、`huaweicloud` のいずれか。 |
| `hostname` | `string` | はい | DNS ホスト名。 |
| `record_type` | `string` | いいえ | 空、`A`、`AAAA`、`CNAME`。 |
| `value` | `string` | いいえ | DNS レコード値。マルチノードサービスは明示的に設定することを推奨します。 |
| `proxied` | `bool` | いいえ | プロバイダー固有のプロキシトグル。現在は Cloudflare に関連します。 |
| `ttl` | `uint32` | いいえ | DNS TTL。 |
| `comment` | `string` | いいえ | DNS レコードコメント。 |

## イメージ更新

```yaml
update:
  enabled: true
  auto_apply: false
  check_schedule: "0 */6 * * *"
  backup_before_update: true
  digest_pin: false
  backup_data:
    - name: db
      enabled: true
  discovery_sources:
    upstream:
      sources:
        - type: github
          repo: owner/repo
      combine: first_success
      include_prerelease: false
  images:
    app:
      image: ghcr.io/example/app
      current:
        env:
          file: .env
          key: APP_VERSION
      discovery: upstream
      filter:
        type: semver
        allow:
          - patch
          - minor
```

### `update`

| キー | 型 | 説明 |
|-----|------|-------------|
| `enabled` | `bool` | このサービスの更新チェックを有効にします。 |
| `auto_apply` | `bool` | 検出された更新を自動的に適用します。 |
| `check_schedule` | `string` | 更新チェックの cron スケジュール。 |
| `backup_before_update` | `bool` | 更新を適用する前にバックアップを実行します。 |
| `backup_data` | `[]object` | 更新前にバックアップする保護データ項目。 |
| `digest_pin` | `bool` | イメージをダイジェストで固定します。 |
| `discovery_sources` | `map[string]object` | 再利用可能な検出ソース。名前付きソースは別のソースを参照できません。 |
| `images` | `map[string]object` | イメージごとの更新定義。 |

### `update.backup_data[]`

| キー | 型 | 説明 |
|-----|------|-------------|
| `name` | `string` | 保護データ項目名。 |
| `enabled` | `bool` | この項目を含めるか除外するか。 |

### `update.images.<name>`

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `image` | `string` | はい | イメージリポジトリ。 |
| `auto_apply` | `bool` | いいえ | イメージごとの自動適用の上書き。 |
| `check_schedule` | `string` | いいえ | イメージごとのチェックスケジュール。 |
| `backup_before_update` | `bool` | いいえ | イメージごとのバックアップトグル。 |
| `digest_pin` | `bool` | いいえ | イメージごとのダイジェスト固定トグル。 |
| `current` | `object` | はい | 現在のバージョンソース。 |
| `discovery` | `object` または `string` | はい | 検出設定または名前付き検出ソース参照。 |
| `filter` | `object` | 条件付き | 検出が `digest` でない限り必須。 |

### `current`

以下のいずれか 1 つを指定します:

| キー | 説明 |
|-----|-------------|
| `tag` | 静的な現在のタグ。 |
| `env.file` + `env.key` | env ファイルから現在のタグを読み取ります。`file` は相対パスでサービスディレクトリ内に留まる必要があります。 |
| `yaml.file` + `yaml.path` | YAML ファイルから現在のタグを読み取ります。`file` は相対パスでサービスディレクトリ内に留まる必要があります。 |

### `discovery`

| キー | 型 | 説明 |
|-----|------|-------------|
| `sources` | `[]object` | 少なくとも 1 つのソース。 |
| `combine` | `string` | 空、`merge`、`first_success`。 |
| `include_prerelease` | `bool` | プレリリースバージョンを含めます。 |

検出ソースタイプ:

| タイプ | 必須キー | 注意事項 |
|------|---------------|-------|
| `auto` | なし | `repo_url` はオプションで、設定する場合は有効な URL である必要があります。唯一のソースである必要があります。 |
| `probe` | なし | フィルターが存在する場合、`semver` フィルターが必要です。 |
| `registry` | なし | レジストリタグ検出。 |
| `digest` | なし | 唯一のソースである必要があります。`filter` は省略する必要があります。 |
| `github` | `repo` | `repo` は `owner/repo` 形式。 |
| `gitlab` | `project` | GitLab プロジェクト ID またはパス。 |
| `forgejo` | `repo` | `repo` は `owner/repo` 形式。 |

### `filter`

| タイプ | 必須キー | 注意事項 |
|------|---------------|-------|
| `semver` | なし | `allow` には `patch`、`minor`、`major` を含められます。 |
| `date` | `format` | タグの解析に使用される日付フォーマット。 |
| `regex` | `pattern`, `order` | `order` は `numeric` または `lexicographic` である必要があります。 |
| `latest` | なし | 最新の候補を使用します。 |

## データ保護

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

### `data_protect.data[]`

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `name` | `string` | はい | 一意のデータ項目名。 |
| `backup` | `object` | いいえ | バックアップアクション。 |
| `restore` | `object` | いいえ | リストアアクション。 |

### データアクション

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `strategy` | `string` | はい | `files.copy`、`files.copy_after_stop`、`database.pgdumpall`、`database.pgimport`。 |
| `service` | `string` | 条件付き | `database.*` 戦略で必須。Compose サービス名。 |
| `include` | `[]string` | 条件付き | `files.*` 戦略で必須。`./...` または `/` を含むパスはサービスパス、裸の名前は Docker ボリューム名です。 |

## バックアップ

```yaml
backup:
  data:
    - name: db
      provider: rustic
      enabled: true
      schedule: "0 2 * * *"
```

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `name` | `string` | はい | バックアップアクションを持つ `data_protect.data[].name` を参照する必要があります。 |
| `provider` | `string` | いいえ | バックアッププロバイダー名。 |
| `enabled` | `bool` | いいえ | このバックアップエントリを有効または無効にします。 |
| `schedule` | `string` | いいえ | cron スケジュール。 |

## 移行

```yaml
migrate:
  data:
    - name: db
      enabled: true
```

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `name` | `string` | はい | バックアップとリストアの両方のアクションを持つ `data_protect.data[].name` を参照する必要があります。 |
| `enabled` | `bool` | いいえ | この項目の移行を有効または無効にします。 |
