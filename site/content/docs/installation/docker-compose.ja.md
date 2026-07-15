---
title: "Docker Compose"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Docker Compose スタックは、公式の [`docker-compose.yaml`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/docker-compose.yaml) からコントローラー、1 つのローカルエージェント、Web UI を実行します。

## ファイルのダウンロード

Docker Compose インストールのためにリポジトリ全体をクローンする必要はありません。Compose ファイルと環境テンプレートをダウンロードします:

```bash
curl -LO https://forgejo.alexma.top/alexma233/composia/raw/branch/main/docker-compose.yaml
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/.env.example -o .env
```

スタックを起動する前に `.env` を編集します。テンプレートはロールごとにグループ化されています。オールインワンスタックの場合はすべてのグループを保持します。各変数の意味については [設定](../configuration/) を参照してください。

ホスト上の Docker ソケットグループ ID を確認します:

```bash
stat -c '%g' /var/run/docker.sock
```

`DOCKER_SOCK_GID` をその値に設定します。

## エージェントリポジトリパス

`COMPOSIA_AGENT_REPO_DIR` は次のようにマウントされます:

```yaml
- ${COMPOSIA_AGENT_REPO_DIR}:${COMPOSIA_AGENT_REPO_DIR}
```

ホストパスとコンテナパスは同じである必要があります。エージェントはホスト Docker デーモンを呼び出し、ホスト Docker デーモンはホストファイルシステムからバインドマウントを解決します。サービスリポジトリがエージェントコンテナ内で異なるパスにマウントされている場合、Docker Compose は存在しないホストパスを生成する可能性があります。

両側で同じ絶対パスを使用してください。例:

```bash
COMPOSIA_AGENT_REPO_DIR=/data/repo-agent
```

Set `agent.repo_dir` in `config.yaml` to the same absolute path.

## 基本的な `config.yaml`

`COMPOSIA_CONFIG_DIR` 内に `config.yaml` を作成します。Docker Compose ファイルはこのディレクトリを `/app/configs` にマウントします。

```yaml {filename="config.yaml"}
controller:
  listen_addr: ":7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  access_tokens:
    - name: "web"
      token: "REPLACE_WITH_WEB_ACCESS_TOKEN"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
```

`.env` の `WEB_CONTROLLER_ACCESS_TOKEN` を `controller.access_tokens[0].token` と同じ値に設定します。

## Web パスワード

`WEB_LOGIN_PASSWORD_HASH` must be an Argon2id PHC hash. Generate it from a hidden prompt so the plaintext password is not written to shell history:

```bash
read -r -s -p 'Web password: ' COMPOSIA_WEB_PASSWORD; echo
printf '%s' "$COMPOSIA_WEB_PASSWORD" | docker run --rm -i -e NODE_NO_WARNINGS=1 node:24-alpine node -e 'const {randomBytes}=require("node:crypto");let p="";process.stdin.setEncoding("utf8");process.stdin.on("data",c=>p+=c);process.stdin.on("end",async()=>{const salt=randomBytes(16);const key=await crypto.subtle.importKey("raw-secret",Buffer.from(p),"Argon2id",false,["deriveBits"]);const bits=await crypto.subtle.deriveBits({name:"Argon2id",memory:65536,passes:3,parallelism:1,nonce:salt},key,256);const b64=b=>Buffer.from(b).toString("base64").replace(/=+$/g,"");console.log(`$argon2id$v=19$m=65536,t=3,p=1$${b64(salt)}$${b64(bits)}`);})'
unset COMPOSIA_WEB_PASSWORD
```

Paste the full `$argon2id$...` output into `.env`. The command uses Docker to run Node.js 24, so it does not require a local Node.js install.

`WEB_SESSION_SECRET` は暗号論的に安全な乱数生成器で生成します。例:

```bash
openssl rand -hex 32
```

## 起動

```bash
docker compose up -d
docker compose ps
```

`http://localhost:3000` で Web UI を開きます。

## ロール分割

Compose ファイルはロールごとにセクション分けされています:

- **コントローラースタック**: `init-repo-controller`、`init-perms-controller`、`controller`。
- **Web UI**: `web`。
- **共有初期化**: `init-config-perms`。
- **エージェントスタック**: `init-perms-agent`、`agent`。

オールインワンデプロイ以外の場合は、トポロジに合わせてこれらのセクションを明示的に分割します。コントローラーと Web は一緒にまたは別々に実行できます。各エージェントノードはエージェントスタックと独自の Docker ソケットアクセスを保持します。

## イメージ

リリースイメージは Forgejo、GHCR、Docker Hub に公開されています:

| コンポーネント | Forgejo | GHCR | Docker Hub |
|-----------|---------|------|------------|
| CLI | `forgejo.alexma.top/alexma233/composia-cli` | `ghcr.io/alexma233/composia-cli` | `alexma233/composia-cli` |
| コントローラー | `forgejo.alexma.top/alexma233/composia-controller` | `ghcr.io/alexma233/composia-controller` | `alexma233/composia-controller` |
| エージェント | `forgejo.alexma.top/alexma233/composia-agent` | `ghcr.io/alexma233/composia-agent` | `alexma233/composia-agent` |
| Web | `forgejo.alexma.top/alexma233/composia-web` | `ghcr.io/alexma233/composia-web` | `alexma233/composia-web` |

Canary イメージは Forgejo と GHCR にのみ公開されています。

## 一般的なチェック

- コントローラーが起動できない: `config.yaml` が `COMPOSIA_CONFIG_DIR` 以下に存在し、必要なコントローラーパスが存在するか作成可能であることを確認します。
- エージェントが Docker を使用できない: `DOCKER_SOCK_GID` がホストの `/var/run/docker.sock` と一致することを確認します。
- Web がコントローラーに到達できない: `WEB_CONTROLLER_ADDR` は Web サーバーコンテナ用で、`WEB_BROWSER_CONTROLLER_ADDR` はブラウザ用です。
