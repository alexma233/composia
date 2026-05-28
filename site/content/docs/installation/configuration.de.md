---
title: "Konfiguration"
date: '2026-05-26T00:00:00+08:00'
weight: 40
---

Diese Seite behandelt die installationsbezogene Konfiguration: Controller-Konfiguration, Agent-Konfiguration, Web-Umgebungsvariablen und Age-Schlüssel-Einrichtung.

Dienstdefinitionen befinden sich in `composia-meta.yaml`. Siehe [Dienst-Leitfaden](/docs/guide/service/) für diese Datei.

## Aufbau der Konfigurationsdatei

Controller und Agent verwenden dasselbe YAML-Dateiformat. Eine Datei kann einen oder beide Abschnitte enthalten:

```yaml
controller:
  # Controller-Einstellungen

agent:
  # Agent-Einstellungen
```

Mindestens einer von `controller` oder `agent` muss vorhanden sein.

Wenn dieselbe Konfigurationsdatei beide Abschnitte enthält, wird der lokale Agent als eingebauter Node behandelt:

- `agent.node_id` muss `main` sein.
- `controller.nodes` muss einen Eintrag mit `id: main` enthalten.
- `controller.repo_dir` und `agent.repo_dir` dürfen nicht derselbe Pfad sein.

## Vollständige Konfigurationsvorlage

Diese Vorlage zeigt jeden unterstützten installationsbezogenen Schlüssel. Sie ist eine Formreferenz, kein kopierbarer Standard. Entferne Abschnitte, die du nicht verwendest, entferne leere Listeneinträge und verwende entweder Inline-Werte oder `_file`-Werte für jedes secret-artige Feld.

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
      comment: "Web-UI-Zugriffstoken"

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

Behalte keine leeren Listeneinträge wie `controller_headers` mit einem leeren `name`. Sie werden nur zur Dokumentation der unterstützten Objektform gezeigt.

Das Web-Zugriffstoken und das Haupt-Agent-Token müssen unterschiedlich sein.

## Age-Schlüssel-Einrichtung

`controller.secrets` ist optional. Konfiguriere es nur, wenn du Composia-verwaltete verschlüsselte Secrets verwendest.

Wenn `controller.secrets` konfiguriert ist, ist `identity_file` erforderlich. `recipient_file` ist optional. Wenn es weggelassen wird, leitet Composia den Empfänger vom privaten Schlüssel ab.

Generiere einen privaten Schlüssel:

```bash
age-keygen -o age-identity.key
```

Optionale Empfängerdatei:

```bash
age-keygen -y age-identity.key > age-recipients.txt
```

Verwende den privaten Schlüssel in der Konfiguration:

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
```

Oder verwende beide Dateien:

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
  recipient_file: "/app/configs/age-recipients.txt"
```

`armor` ist optional und standardmäßig `true`.

## Controller-Konfigurationsreferenz

### Erforderliche Schlüssel

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `listen_addr` | `string` | Controller-Listening-Adresse, zum Beispiel `":7001"` oder `"127.0.0.1:7001"`. |
| `repo_dir` | `string` | Pfad des Sollzustand-Git-Repositories. |
| `state_dir` | `string` | Controller-Zustandspfad. |
| `log_dir` | `string` | Aufgabenprotokoll-Verzeichnis. |
| `nodes` | `[]object` | Konfigurierte Agent-Nodes. Der Schlüssel muss vorhanden sein, auch wenn leer. |

### Optionale Schlüssel der obersten Ebene

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `access_tokens` | `[]object` | API-Tokens für Web-UI, CLI und externe Clients. |
| `backup` | `object` | Globale Backup-Standardwerte. |
| `git` | `object` | Remote-Synchronisation des Sollzustand-Repositories. |
| `notifications` | `object` | Alertmanager-, SMTP- und Telegram-Benachrichtigungen. |
| `dns` | `object` | DNS-Provider-Anmeldeinformationen. |
| `rustic` | `object` | Rustic-Wartungseinstellungen. |
| `secrets` | `object` | Age-Verschlüsselungseinstellungen. |
| `updates` | `object` | Image-Update-Standardwerte und Forge-API-Auth. |
| `auto_deploy` | `object` | Globale Auto-Deploy-Umschalter. |

### `nodes[]`

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `id` | `string` | Ja | Eindeutige Node-ID. |
| `display_name` | `string` | Nein | In der UI angezeigter Name. |
| `enabled` | `bool` | Nein | Deaktiviert einen Node, ohne ihn zu entfernen. |
| `public_ipv4` | `string` | Nein | Öffentliche IPv4, die von DNS-Workflows verwendet wird. |
| `public_ipv6` | `string` | Nein | Öffentliche IPv6, die von DNS-Workflows verwendet wird. |
| `token` | `string` | Ja* | Agent-Auth-Token. |
| `token_file` | `string` | Nein | Liest das Token aus einer Datei. |

*Verwende entweder `token` oder `token_file`, nicht beides.

### `access_tokens[]`

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `name` | `string` | Ja | Token-Name. |
| `token` | `string` | Ja* | Token-Wert. |
| `token_file` | `string` | Nein | Liest das Token aus einer Datei. |
| `enabled` | `bool` | Nein | Deaktiviert ein Token, ohne es zu entfernen. |
| `comment` | `string` | Nein | Administrativer Hinweis. |

Zugriffstokens dürfen Node-Tokens oder andere Zugriffstokens nicht duplizieren.

### `git`

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `remote_url` | `string` | Nein | Git-Remote-URL. |
| `branch` | `string` | Nein | Zu synchronisierender Branch. |
| `pull_interval` | `string` | Bed. | Erforderlich, wenn `remote_url` gesetzt ist. |
| `author_name` | `string` | Nein | Commit-Autor-Name für Controller-Schreibvorgänge. |
| `author_email` | `string` | Nein | Commit-Autor-E-Mail. |
| `auth.username` | `string` | Nein | Git-Benutzername. |
| `auth.token` | `string` | Nein | Git-Token. |
| `auth.token_file` | `string` | Nein | Liest Git-Token aus einer Datei. |

### `secrets`

Dieser gesamte Abschnitt ist optional. Wenn der Abschnitt vorhanden ist, gelten diese Regeln:

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `provider` | `string` | Ja | Muss `age` sein. |
| `identity_file` | `string` | Ja | Pfad zum privaten Age-Schlüssel. |
| `recipient_file` | `string` | Nein | Pfad zur Age-Empfängerdatei. Wenn weggelassen, wird der Empfänger von `identity_file` abgeleitet. |
| `armor` | `bool` | Nein | ASCII-Armor-verschlüsselte Ausgabe. Standardmäßig `true`. |

### `backup`

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `default_schedule` | `string` | Standard-Cron-Zeitplan für Dienst-Backups. |

### `updates`

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `default_check_schedule` | `string` | Standard-Cron-Zeitplan für Image-Update-Prüfungen. |
| `auto_apply` | `bool` | Wendet Updates standardmäßig automatisch an. |
| `backup_before_update` | `bool` | Sichert Daten vor dem Anwenden von Updates. |
| `digest_pin` | `bool` | Fixiert Images per Digest. |
| `semver.default_allow` | `[]string` | Erlaubte Semver-Sprungstufen: `patch`, `minor`, `major`. |
| `forge_auth.github` | `object` oder `[]object` | GitHub-API-Auth. |
| `forge_auth.gitlab` | `object` oder `[]object` | GitLab-API-Auth. |
| `forge_auth.forgejo` | `object` oder `[]object` | Forgejo-API-Auth. |

Jeder Forge-Auth-Eintrag unterstützt:

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `url` | `string` | Forge-Basis-URL. |
| `token` | `string` | API-Token. |
| `token_file` | `string` | Liest API-Token aus einer Datei. |
| `api_url` | `string` | API-URL-Override. |

### `auto_deploy`

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `infra` | `bool` | Infrastrukturdienste nach Git-Änderungen automatisch deployen. |
| `services` | `bool` | Reguläre Dienste nach Git-Änderungen automatisch deployen. |

### `dns`

| Provider-Schlüssel | Anmeldeschlüssel | Gemeinsame Schlüssel |
|--------------|-----------------|-------------|
| `cloudflare` | `api_token`, `api_token_file` | `zones` |
| `alidns` | `access_key_id`, `access_key_id_file`, `access_key_secret`, `access_key_secret_file`, `security_token`, `security_token_file`, `region_id` | `zones` |
| `dnspod` | `secret_id`, `secret_id_file`, `secret_key`, `secret_key_file`, `session_token`, `session_token_file`, `region` | `zones` |
| `route53` | `access_key_id`, `access_key_id_file`, `secret_access_key`, `secret_access_key_file`, `session_token`, `session_token_file`, `region`, `profile`, `hosted_zone_id` | `zones` |
| `huaweicloud` | `access_key_id`, `access_key_id_file`, `secret_access_key`, `secret_access_key_file`, `region_id` | `zones` |

### `rustic`

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `main_nodes` | `[]string` | Node-IDs, die Rustic-Operationen ausführen. Jede muss auf `controller.nodes` verweisen. |
| `maintenance.forget_schedule` | `string` | Cron-Zeitplan für `rustic forget`. |
| `maintenance.prune_schedule` | `string` | Cron-Zeitplan für `rustic prune`. |

### `notifications.alertmanager`

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `enabled` | `bool` | Standardmäßig aktiviert, wenn der Abschnitt existiert. |
| `listen_path` | `string` | Webhook-Pfad. Standardmäßig `/api/v1/alerts`. Muss mit `/` beginnen. |

### `notifications.smtp`

| Schlüssel | Typ | Erforderlich wenn aktiviert | Beschreibung |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | Nein | Standardmäßig aktiviert, wenn der Abschnitt existiert. |
| `host` | `string` | Ja | SMTP-Host. |
| `port` | `int` | Ja | SMTP-Port, 1 bis 65535. |
| `encryption` | `string` | Nein | `none`, `starttls` oder `ssl_tls`. Standardmäßig `starttls`. |
| `username` | `string` | Nein | SMTP-Benutzername. |
| `password` | `string` | Nein | SMTP-Passwort. |
| `password_file` | `string` | Nein | Liest Passwort aus einer Datei. |
| `from` | `string` | Ja | Absenderadresse. |
| `to` | `[]string` | Ja | Empfängerliste. |
| `on` | `[]string` | Nein | Benachrichtigungsereignisfilter. |
| `task_sources` | `[]string` | Nein | Aufgabenquellenfilter: `web`, `cli`, `others`, `schedule`, `system`. |

### `notifications.telegram`

| Schlüssel | Typ | Erforderlich wenn aktiviert | Beschreibung |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | Nein | Standardmäßig aktiviert, wenn der Abschnitt existiert. |
| `bot_token` | `string` | Ja* | Telegram-Bot-Token. |
| `bot_token_file` | `string` | Nein | Liest Bot-Token aus einer Datei. |
| `chat_id` | `string` | Ja | Ziel-Chat-ID. |
| `on` | `[]string` | Nein | Benachrichtigungsereignisfilter. |
| `task_sources` | `[]string` | Nein | Aufgabenquellenfilter. |

## Agent-Konfigurationsreferenz

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `controller_addr` | `string` | Ja | Vom Agenten aus erreichbare Controller-URL. |
| `controller_grpc` | `bool` | Nein | Verwendet gRPC anstelle von Connect über HTTP. |
| `controller_headers` | `[]object` | Nein | Zusätzliche HTTP-Header, die an den Controller gesendet werden. |
| `node_id` | `string` | Ja | Node-ID dieses Agenten. Muss mit `controller.nodes[].id` übereinstimmen. |
| `token` | `string` | Ja* | Node-Token, das mit der Controller-Konfiguration übereinstimmt. |
| `token_file` | `string` | Nein | Liest Node-Token aus einer Datei. |
| `repo_dir` | `string` | Ja | Agent-Dienst-Repository-Pfad. |
| `state_dir` | `string` | Ja | Agent-Zustandsverzeichnis. |
| `caddy` | `object` | Nein | Agent-seitige Caddy-Einstellungen. |

*Verwende entweder `token` oder `token_file`, nicht beides.

### `controller_headers[]`

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `name` | `string` | Ja | HTTP-Header-Name. Header-Namen werden case-insensitiv dedupliziert. |
| `value` | `string` | Ja* | Header-Wert. |
| `value_file` | `string` | Nein | Liest Header-Wert aus einer Datei. |

### `caddy`

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `generated_dir` | `string` | Generiertes Caddy-Konfigurationsverzeichnis. Standardmäßig `<state_dir>/caddy/generated`. |

## Web-Umgebungsvariablen

Der Webserver liest Umgebungsvariablen. In Docker Compose werden diese über `.env` gesetzt.

| Variable | Erforderlich | Beschreibung |
|----------|----------|-------------|
| `WEB_CONTROLLER_ADDR` | Ja | Controller-Adresse aus Sicht des Webserver-Prozesses. In Docker Compose: `http://controller:7001`. |
| `WEB_BROWSER_CONTROLLER_ADDR` | Ja | Controller-Adresse aus Sicht des Browsers. |
| `WEB_CONTROLLER_ACCESS_TOKEN` | Ja | Controller-Zugriffstoken. Muss mit `controller.access_tokens[].token` übereinstimmen. |
| `WEB_CONTROLLER_HEADERS` | Nein | JSON-Objekt mit zusätzlichen Headern, die der Webserver beim Aufruf des Controllers sendet. |
| `WEB_LOGIN_USERNAME` | Ja | Web-Login-Benutzername. |
| `WEB_LOGIN_PASSWORD_HASH` | Ja | Argon2-Passwort-Hash. |
| `WEB_SESSION_SECRET` | Ja | Zufälliges Sitzungs-Signierungsgeheimnis. |
| `ORIGIN` | Deployment-abhängig | Öffentlicher Ursprung des Webservers. |
| `HOST` | Nein | Host-Bind-Adresse. |
| `PORT` | Nein | Webserver-Port. |

## Inline-Werte und `_file`-Werte

Viele secret-artige Felder unterstützen sowohl Inline-Werte als auch Dateireferenzen. Beispiele:

- `token` / `token_file`
- `password` / `password_file`
- `api_token` / `api_token_file`
- `value` / `value_file`

Verwende nur eine Form. Wenn beide gesetzt sind, schlägt der Start fehl.
