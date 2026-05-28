---
title: "設定"
date: '2026-05-26T00:00:00+08:00'
weight: 40
---

このページではインストールレベルの設定（コントローラー設定、エージェント設定、Web 環境変数、age 鍵のセットアップ）を説明します。

サービス定義は `composia-meta.yaml` に記述します。このファイルについては [サービスガイド](/docs/guide/service/) を参照してください。

## 設定ファイルの形式

コントローラーとエージェントは同じ YAML ファイル形式を使用します。ファイルにはどちらかのセクション、または両方を含めることができます:

```yaml
controller:
  # コントローラー設定

agent:
  # エージェント設定
```

`controller` または `agent` の少なくとも 1 つが存在する必要があります。

同じ設定ファイルに両方のセクションが含まれる場合、ローカルエージェントは組み込みノードとして扱われます:

- `agent.node_id` は `main` である必要があります。
- `controller.nodes` には `id: main` のエントリが含まれている必要があります。
- `controller.repo_dir` と `agent.repo_dir` は同じパスであってはいけません。

## 完全な設定テンプレート

このテンプレートはサポートされているすべてのインストールレベルのキーを示しています。形のリファレンスであり、コピー＆ペースト用のデフォルトではありません。使用しないセクションは削除し、空のリスト項目は削除し、各シークレット系フィールドにはインライン値または `_file` 値のいずれかを使用してください。

```yaml {filename="config.yaml"}
controller:
  listen_addr: ":7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"

  access_tokens:
    - name: "web"
      token: "REPLACE_WITH_WEB_ACCESS_TOKEN"
      token_file: ""
      enabled: true
      comment: "Web UI アクセストークン"

  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      public_ipv4: ""
      public_ipv6: ""
      token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
      token_file: ""

  git:
    remote_url: ""
    branch: "main"
    pull_interval: ""
    author_name: "Composia"
    author_email: "composia@example.com"
    auth:
      username: ""
      token: ""
      token_file: ""

  backup:
    default_schedule: ""

  updates:
    default_check_schedule: ""
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
        token: ""
        token_file: ""
        api_url: "https://api.github.com"
      gitlab:
        url: "https://gitlab.com"
        token: ""
        token_file: ""
        api_url: "https://gitlab.com/api/v4"
      forgejo:
        url: "https://forgejo.example.com"
        token: ""
        token_file: ""
        api_url: ""

  auto_deploy:
    infra: false
    services: false

  dns:
    cloudflare:
      api_token: ""
      api_token_file: ""
      zones: []
    alidns:
      access_key_id: ""
      access_key_id_file: ""
      access_key_secret: ""
      access_key_secret_file: ""
      security_token: ""
      security_token_file: ""
      region_id: ""
      zones: []
    dnspod:
      secret_id: ""
      secret_id_file: ""
      secret_key: ""
      secret_key_file: ""
      session_token: ""
      session_token_file: ""
      region: ""
      zones: []
    route53:
      access_key_id: ""
      access_key_id_file: ""
      secret_access_key: ""
      secret_access_key_file: ""
      session_token: ""
      session_token_file: ""
      region: ""
      profile: ""
      hosted_zone_id: ""
      zones: []
    huaweicloud:
      access_key_id: ""
      access_key_id_file: ""
      secret_access_key: ""
      secret_access_key_file: ""
      region_id: ""
      zones: []

  rustic:
    main_nodes:
      - "main"
    maintenance:
      forget_schedule: ""
      prune_schedule: ""

  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
    recipient_file: ""
    armor: true

  notifications:
    alertmanager:
      enabled: true
      listen_path: "/api/v1/alerts"
    smtp:
      enabled: false
      host: ""
      port: 587
      encryption: starttls
      username: ""
      password: ""
      password_file: ""
      from: ""
      to: []
      on: []
      task_sources: []
    telegram:
      enabled: false
      bot_token: ""
      bot_token_file: ""
      chat_id: ""
      on: []
      task_sources: []

agent:
  controller_addr: "http://controller:7001"
  controller_grpc: false
  controller_headers:
    - name: ""
      value: ""
      value_file: ""
  node_id: "main"
  token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
  token_file: ""
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
  caddy:
    generated_dir: ""
```

空の `name` を持つ `controller_headers` のような空のリスト項目は保持しないでください。これらはサポートされているオブジェクトの形を文書化するためにのみ表示されています。

Web アクセストークンとメインエージェントトークンは異なるものである必要があります。

## age 鍵のセットアップ

`controller.secrets` はオプションです。Composia 管理の暗号化シークレットを使用する場合にのみ設定してください。

`controller.secrets` が設定されている場合、`identity_file` は必須です。`recipient_file` はオプションです。省略した場合、Composia は秘密鍵から受信者を導出します。

秘密鍵の生成:

```bash
age-keygen -o age-identity.key
```

オプションの受信者ファイル:

```bash
age-keygen -y age-identity.key > age-recipients.txt
```

設定で秘密鍵を使用:

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
```

または両方のファイルを使用:

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
  recipient_file: "/app/configs/age-recipients.txt"
```

`armor` はオプションで、デフォルトは `true` です。

## コントローラー設定リファレンス

### 必須キー

| キー | 型 | 説明 |
|-----|------|-------------|
| `listen_addr` | `string` | コントローラーのリッスンアドレス。例: `":7001"` または `"127.0.0.1:7001"`。 |
| `repo_dir` | `string` | 期待状態 Git リポジトリのパス。 |
| `state_dir` | `string` | コントローラー状態のパス。 |
| `log_dir` | `string` | タスクログディレクトリ。 |
| `nodes` | `[]object` | 設定されたエージェントノード。空でもキーが存在する必要があります。 |

### オプションのトップレベルキー

| キー | 型 | 説明 |
|-----|------|-------------|
| `access_tokens` | `[]object` | Web UI、CLI、外部クライアント用の API トークン。 |
| `backup` | `object` | グローバルバックアップデフォルト。 |
| `git` | `object` | 期待状態リポジトリのリモート同期。 |
| `notifications` | `object` | Alertmanager、SMTP、Telegram 通知。 |
| `dns` | `object` | DNS プロバイダー認証情報。 |
| `rustic` | `object` | Rustic メンテナンス設定。 |
| `secrets` | `object` | Age 暗号化設定。 |
| `updates` | `object` | イメージ更新デフォルトと forge API 認証。 |
| `auto_deploy` | `object` | グローバル自動デプロイトグル。 |

### `nodes[]`

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `id` | `string` | はい | 一意のノード ID。 |
| `display_name` | `string` | いいえ | UI に表示される名前。 |
| `enabled` | `bool` | いいえ | 削除せずにノードを無効化します。 |
| `public_ipv4` | `string` | いいえ | DNS ワークフローで使用されるパブリック IPv4。 |
| `public_ipv6` | `string` | いいえ | DNS ワークフローで使用されるパブリック IPv6。 |
| `token` | `string` | はい* | エージェント認証トークン。 |
| `token_file` | `string` | いいえ | ファイルからトークンを読み取ります。 |

*`token` または `token_file` のいずれかを使用し、両方は使用しないでください。

### `access_tokens[]`

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `name` | `string` | はい | トークン名。 |
| `token` | `string` | はい* | トークン値。 |
| `token_file` | `string` | いいえ | ファイルからトークンを読み取ります。 |
| `enabled` | `bool` | いいえ | 削除せずにトークンを無効化します。 |
| `comment` | `string` | いいえ | 管理用メモ。 |

アクセストークンはノードトークンや他のアクセストークンと重複してはいけません。

### `git`

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `remote_url` | `string` | いいえ | Git リモート URL。 |
| `branch` | `string` | いいえ | 同期するブランチ。 |
| `pull_interval` | `string` | 条件付き | `remote_url` が設定されている場合に必須。 |
| `author_name` | `string` | いいえ | コントローラー書き込みのコミット作成者名。 |
| `author_email` | `string` | いいえ | コミット作成者メール。 |
| `auth.username` | `string` | いいえ | Git ユーザー名。 |
| `auth.token` | `string` | いいえ | Git トークン。 |
| `auth.token_file` | `string` | いいえ | ファイルから Git トークンを読み取ります。 |

### `secrets`

このセクション全体はオプションです。セクションが存在する場合、以下のルールが適用されます:

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `provider` | `string` | はい | `age` である必要があります。 |
| `identity_file` | `string` | はい | Age 秘密鍵のパス。 |
| `recipient_file` | `string` | いいえ | Age 受信者ファイルのパス。省略した場合、受信者は `identity_file` から導出されます。 |
| `armor` | `bool` | いいえ | ASCII アーマー暗号化出力。デフォルトは `true`。 |

### `backup`

| キー | 型 | 説明 |
|-----|------|-------------|
| `default_schedule` | `string` | サービスバックアップのデフォルト cron スケジュール。 |

### `updates`

| キー | 型 | 説明 |
|-----|------|-------------|
| `default_check_schedule` | `string` | イメージ更新チェックのデフォルト cron スケジュール。 |
| `auto_apply` | `bool` | デフォルトで更新を自動適用します。 |
| `backup_before_update` | `bool` | 更新を適用する前にデータをバックアップします。 |
| `digest_pin` | `bool` | イメージをダイジェストで固定します。 |
| `semver.default_allow` | `[]string` | 許可される semver バンプレベル: `patch`、`minor`、`major`。 |
| `forge_auth.github` | `object` または `[]object` | GitHub API 認証。 |
| `forge_auth.gitlab` | `object` または `[]object` | GitLab API 認証。 |
| `forge_auth.forgejo` | `object` または `[]object` | Forgejo API 認証。 |

各 forge 認証エントリは以下をサポートします:

| キー | 型 | 説明 |
|-----|------|-------------|
| `url` | `string` | Forge のベース URL。 |
| `token` | `string` | API トークン。 |
| `token_file` | `string` | ファイルから API トークンを読み取ります。 |
| `api_url` | `string` | API URL の上書き。 |

### `auto_deploy`

| キー | 型 | 説明 |
|-----|------|-------------|
| `infra` | `bool` | Git 変更後にインフラストラクチャサービスを自動デプロイします。 |
| `services` | `bool` | Git 変更後に通常のサービスを自動デプロイします。 |

### `dns`

| プロバイダーキー | 認証情報キー | 共通キー |
|--------------|-----------------|-------------|
| `cloudflare` | `api_token`, `api_token_file` | `zones` |
| `alidns` | `access_key_id`, `access_key_id_file`, `access_key_secret`, `access_key_secret_file`, `security_token`, `security_token_file`, `region_id` | `zones` |
| `dnspod` | `secret_id`, `secret_id_file`, `secret_key`, `secret_key_file`, `session_token`, `session_token_file`, `region` | `zones` |
| `route53` | `access_key_id`, `access_key_id_file`, `secret_access_key`, `secret_access_key_file`, `session_token`, `session_token_file`, `region`, `profile`, `hosted_zone_id` | `zones` |
| `huaweicloud` | `access_key_id`, `access_key_id_file`, `secret_access_key`, `secret_access_key_file`, `region_id` | `zones` |

### `rustic`

| キー | 型 | 説明 |
|-----|------|-------------|
| `main_nodes` | `[]string` | Rustic 操作を実行するノード ID。それぞれ `controller.nodes` を参照する必要があります。 |
| `maintenance.forget_schedule` | `string` | `rustic forget` の cron スケジュール。 |
| `maintenance.prune_schedule` | `string` | `rustic prune` の cron スケジュール。 |

### `notifications.alertmanager`

| キー | 型 | 説明 |
|-----|------|-------------|
| `enabled` | `bool` | セクションが存在する場合、デフォルトで有効。 |
| `listen_path` | `string` | Webhook パス。デフォルトは `/api/v1/alerts`。`/` で始まる必要があります。 |

### `notifications.smtp`

| キー | 型 | 有効時に必須 | 説明 |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | いいえ | セクションが存在する場合、デフォルトで有効。 |
| `host` | `string` | はい | SMTP ホスト。 |
| `port` | `int` | はい | SMTP ポート（1 から 65535）。 |
| `encryption` | `string` | いいえ | `none`、`starttls`、`ssl_tls`。デフォルトは `starttls`。 |
| `username` | `string` | いいえ | SMTP ユーザー名。 |
| `password` | `string` | いいえ | SMTP パスワード。 |
| `password_file` | `string` | いいえ | ファイルからパスワードを読み取ります。 |
| `from` | `string` | はい | 送信者アドレス。 |
| `to` | `[]string` | はい | 受信者リスト。 |
| `on` | `[]string` | いいえ | 通知イベントフィルター。 |
| `task_sources` | `[]string` | いいえ | タスクソースフィルター: `web`、`cli`、`others`、`schedule`、`system`。 |

### `notifications.telegram`

| キー | 型 | 有効時に必須 | 説明 |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | いいえ | セクションが存在する場合、デフォルトで有効。 |
| `bot_token` | `string` | はい* | Telegram ボットトークン。 |
| `bot_token_file` | `string` | いいえ | ファイルからボットトークンを読み取ります。 |
| `chat_id` | `string` | はい | ターゲットチャット ID。 |
| `on` | `[]string` | いいえ | 通知イベントフィルター。 |
| `task_sources` | `[]string` | いいえ | タスクソースフィルター。 |

## エージェント設定リファレンス

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `controller_addr` | `string` | はい | エージェントから到達可能なコントローラー URL。 |
| `controller_grpc` | `bool` | いいえ | HTTP 上の Connect の代わりに gRPC を使用します。 |
| `controller_headers` | `[]object` | いいえ | コントローラーに送信される追加の HTTP ヘッダー。 |
| `node_id` | `string` | はい | このエージェントのノード ID。`controller.nodes[].id` と一致する必要があります。 |
| `token` | `string` | はい* | コントローラー設定と一致するノードトークン。 |
| `token_file` | `string` | いいえ | ファイルからノードトークンを読み取ります。 |
| `repo_dir` | `string` | はい | エージェントサービスリポジトリのパス。 |
| `state_dir` | `string` | はい | エージェント状態ディレクトリ。 |
| `caddy` | `object` | いいえ | エージェント側の Caddy 設定。 |

*`token` または `token_file` のいずれかを使用し、両方は使用しないでください。

### `controller_headers[]`

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `name` | `string` | はい | HTTP ヘッダー名。ヘッダー名は大文字小文字を区別せずに重複排除されます。 |
| `value` | `string` | はい* | ヘッダー値。 |
| `value_file` | `string` | いいえ | ファイルからヘッダー値を読み取ります。 |

### `caddy`

| キー | 型 | 説明 |
|-----|------|-------------|
| `generated_dir` | `string` | 生成された Caddy 設定ディレクトリ。デフォルトは `<state_dir>/caddy/generated`。 |

## Web 環境変数

Web サーバーは環境変数を読み取ります。Docker Compose ではこれらは `.env` を通じて設定されます。

| 変数 | 必須 | 説明 |
|----------|----------|-------------|
| `WEB_CONTROLLER_ADDR` | はい | Web サーバープロセスからのコントローラーアドレス。Docker Compose では `http://controller:7001`。 |
| `WEB_BROWSER_CONTROLLER_ADDR` | はい | ブラウザからのコントローラーアドレス。 |
| `WEB_CONTROLLER_ACCESS_TOKEN` | はい | コントローラーアクセストークン。`controller.access_tokens[].token` と一致する必要があります。 |
| `WEB_CONTROLLER_HEADERS` | いいえ | Web サーバーがコントローラーを呼び出す際に送信する追加ヘッダーの JSON オブジェクト。 |
| `WEB_LOGIN_USERNAME` | はい | Web ログインユーザー名。 |
| `WEB_LOGIN_PASSWORD_HASH` | はい | Argon2 パスワードハッシュ。 |
| `WEB_SESSION_SECRET` | はい | ランダムなセッション署名シークレット。 |
| `ORIGIN` | デプロイ依存 | Web サーバーの公開オリジン。 |
| `HOST` | いいえ | ホストバインドアドレス。 |
| `PORT` | いいえ | Web サーバーポート。 |

## インライン値と `_file` 値

多くのシークレット系フィールドはインライン値とファイル参照の両方をサポートしています。例:

- `token` / `token_file`
- `password` / `password_file`
- `api_token` / `api_token_file`
- `value` / `value_file`

いずれか 1 つの形式のみを使用してください。両方が設定されている場合、起動に失敗します。
