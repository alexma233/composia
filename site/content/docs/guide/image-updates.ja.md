---
title: "イメージ更新"
date: '2026-05-26T00:00:00+08:00'
weight: 60
---

Composia は新しいイメージタグを検出し、更新を自動的に適用できます。イメージチェックタスクはエージェント上で実行され、結果をコントローラーに報告します。

## 動作の仕組み

コントローラーはサービスの更新設定に従って定期的な `image_check` タスクをスケジュールします。各チェックは:

1. エージェントがサービスバンドルをダウンロードします。
2. `docker compose config --format json` を読み取って実行中のイメージを検出します。
3. 各イメージのローカルとリモートのダイジェストを報告します。
4. `update.images` で設定されたイメージについて、設定された検出ソースを使用して新しい候補タグをチェックします。
5. 結果をコントローラーに報告します。コントローラーは利用可能な更新を記録し、自動適用できます。

## チェックスケジュールと頻度

イメージチェックのスケジュールは、より具体的な設定を優先して次の順序で選択されます：

```text
update.images.<name>.check_schedule
→ update.check_schedule
→ controller.updates.default_check_schedule
```

これら 3 か所がすべて未設定、または選択された設定が `none` の場合、イメージチェックは自動実行されません。例の `0 */6 * * *` は組み込みのデフォルト値ではありません。

## コントローラーデフォルト

グローバルデフォルトはコントローラー設定で設定します:

```yaml
controller:
  updates:
    default_check_schedule: "0 */6 * * *"
    auto_apply: false
    backup_before_update: true
    digest_pin: false
    semver:
      default_allow:
        - patch
        - minor
    forge_auth:
      github:
        url: "https://github.com"
        token: "REPLACE"
```

サービスレベルの `update` セクションがこれらのデフォルトを上書きします。

### Forge API 認証

`forge_auth` は、コントローラーが GitHub、GitLab、Forgejo の Release API を照会するためだけに使用され、Docker Registry の認証には使用されません。公開 Release にはトークンは不要です。非公開 Release または API レート制限の緩和が必要な場合に認証を設定します。トークンはコントローラー内にのみ保持され、エージェントには送信されません。

| キー | 説明 |
|-----|------|
| `url` | Forge Web サイトのベース URL。GitHub は `https://github.com`、GitLab は `https://gitlab.com` がデフォルトで、Forgejo にはデフォルト値がありません。同じ種類の Forge を複数設定する場合は、インスタンスの識別にも使用されます。 |
| `token` | API トークンを直接設定します。 |
| `token_file` | ファイルから API トークンを読み取ります。`token` と同時には使用できません。 |
| `api_url` | リポジトリや Release のパスを含まない API のベース URL。標準 API URL が適用できない場合にのみ設定します。 |

`api_url` が未設定の場合、コントローラーは `url` に基づいて各プラットフォームの標準 API URL を使用します：

- GitHub.com：`https://api.github.com`。
- GitHub Enterprise Server：`{url}/api/v3`。
- GitLab：`{url}/api/v4`。
- Forgejo：`{url}/api/v1`。

リバースプロキシ、独立した API ドメイン、標準外のパスなどの特殊な構成でのみ `api_url` を設定します。各 Forge タイプには、単一のオブジェクトまたは複数インスタンスの配列を設定できます。たとえば、セルフホスト Forgejo の最小設定は次のとおりです：

```yaml
controller:
  updates:
    forge_auth:
      forgejo:
        url: "https://forgejo.example.com"
        token_file: "/run/secrets/forgejo-token"
```

特殊な構成では API URL を上書きできます：

```yaml
forge_auth:
  forgejo:
    url: "https://forgejo.example.com"
    api_url: "https://api.forgejo.example.com/v1"
    token_file: "/run/secrets/forgejo-token"
```

## サービス設定

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
    upstream-gh:
      sources:
        - type: github
          repo: owner/repo
      combine: first_success
      include_prerelease: false
  images:
    api:
      image: ghcr.io/example/api
      current:
        env:
          file: .env
          key: API_VERSION
      discovery: upstream-gh
      filter:
        type: semver
        allow:
          - patch
          - minor
```

### `update` トップレベル

| キー | 型 | 説明 |
|-----|------|-------------|
| `enabled` | `bool` | このサービスの更新チェックを有効にします。 |
| `auto_apply` | `bool` | 検出された更新を自動的に適用します。 |
| `check_schedule` | `string` | 更新チェックの cron スケジュール。 |
| `backup_before_update` | `bool` | 更新を適用する前にバックアップを実行します。 |
| `backup_data` | `[]object` | 更新前にバックアップする保護データ項目。各項目は `name` とオプションの `enabled` を持ちます。 |
| `digest_pin` | `bool` | 再現性のためにイメージをダイジェストで固定します。 |
| `discovery_sources` | `map[string]object` | 名前付きの再利用可能な検出設定。 |
| `images` | `map[string]object` | イメージごとの更新設定。キーはチェックするイメージに一致する任意の名前です。 |

### `images.<name>`

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `image` | `string` | はい | 完全なイメージ参照（例: `ghcr.io/example/api`）。 |
| `auto_apply` | `bool` | いいえ | イメージごとの自動適用の上書き。 |
| `check_schedule` | `string` | いいえ | イメージごとのチェックスケジュール。 |
| `backup_before_update` | `bool` | いいえ | イメージごとのバックアップトグル。 |
| `digest_pin` | `bool` | いいえ | イメージごとのダイジェスト固定トグル。 |
| `current` | `object` | はい | 現在デプロイされているバージョンの見つけ方。 |
| `discovery` | `object` または `string` | はい | 検出設定または名前付き `discovery_sources` エントリへの参照。 |
| `filter` | `object` | 条件付き | バージョンフィルター。検出モードが `digest` でない限り必須。 |

### `current`

以下のいずれか 1 つを指定する必要があります:

**静的タグ:**

```yaml
current:
  tag: "v1.2.3"
```

**環境ファイル:**

```yaml
current:
  env:
    file: .env
    key: APP_VERSION
```

`file` パスはサービスディレクトリからの相対パスです。Composia はファイルを読み取り、`KEY=VALUE` 行を探して値を抽出します。

**YAML ファイル:**

```yaml
current:
  yaml:
    file: values.yaml
    path: app.image.tag
```

`path` は YAML ドキュメントツリー内のドット区切りパスです。そのパスの値はスカラーである必要があります。

### 検出

検出ソースには以下があります:

**名前付き参照**（`discovery_sources` エントリへの参照）:

```yaml
discovery: upstream-gh
```

**インライン定義:**

```yaml
discovery:
  sources:
    - type: probe
  combine: first_success
  include_prerelease: false
```

検出ソースタイプ:

| タイプ | 必須キー | 動作 |
|------|---------------|----------|
| `probe` | なし | Semver プロービング: レジストリマニフェストをプローブしてより高いバージョンを検索します。`semver` フィルターが必要です。 |
| `registry` | なし | イメージレジストリからすべてのタグをリストします。 |
| `auto` | なし（オプション `repo_url`） | マージされた検出として `probe` を試し、その後 `registry` を試します。検出設定内で唯一のソースである必要があります。 |
| `digest` | なし | リモートダイジェストとローカルダイジェストのみを比較します。タグ比較は行いません。`filter` は省略する必要があります。唯一のソースである必要があります。 |
| `github` | `repo`（`owner/repo`） | GitHub リリースをクエリします。コントローラー側で処理されます。 |
| `gitlab` | `project` | GitLab リリースをクエリします。コントローラー側で処理されます。 |
| `forgejo` | `repo`（`owner/repo`） | Forgejo リリースをクエリします。コントローラー側で処理されます。 |

`combine` は `merge`（すべてのソース結果の和集合）または `first_success`（結果を返した最初のソースが優先）を受け付けます。

`include_prerelease` は GitHub、GitLab、Forgejo のリリースクエリにプレリリースバージョンを含めます。

### フィルター

| タイプ | 必須キー | 動作 |
|------|---------------|----------|
| `semver` | なし | セマンティックバージョンでフィルター。`allow` には `patch`、`minor`、`major` を含められます。 |
| `date` | `format` | 指定されたフォーマットを使用してタグを日付として解析します。 |
| `regex` | `pattern`, `order` | 正規表現でフィルター。`order` は `numeric` または `lexicographic` である必要があります。 |
| `latest` | なし | フィルターなしで最新のタグを取得します。 |

#### Semver プロービング

`type: probe` と `semver` フィルターを使用すると、Composia はバージョン番号を構築し、対応するレジストリマニフェストが存在するかをチェックして候補タグを検索します。`allow` リストに従って patch、minor、major のバンプをプローブし、指数的検索とバイナリ絞り込みを使用して利用可能な最高バージョンを見つけます。

## ダイジェストモード

設定内のすべての検出ソースが `type: digest` の場合、タグ比較は行われません。Composia はリモートイメージダイジェストをローカルダイジェストと比較するだけです:

```yaml
discovery:
  sources:
    - type: digest
```

検出モードとして `digest` が設定されている場合、`filter` は省略する必要があります。ダイジェストが異なる場合、更新が利用可能と見なされます。

## イメージオブザベーション

デプロイおよび更新タスク中、エージェントはすべての compose サービスのイメージオブザベーションも収集します。これにはローカルとリモートのダイジェストが含まれ、`update.images` が設定されているかどうかに関わらずコントローラーに報告されます。これにより Web UI と CLI でイメージ状態の可視性が提供されます。
