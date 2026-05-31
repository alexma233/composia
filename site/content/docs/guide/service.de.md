---
title: "Dienstkonfiguration"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Jeder Dienst befindet sich in einem Verzeichnis der obersten Ebene innerhalb des Controller-Repositories. Ein Dienstverzeichnis enthält `composia-meta.yaml` und eine oder mehrere Docker-Compose-Dateien.

Minimaler Dienst:

```yaml {filename="composia-meta.yaml"}
name: my-app
nodes:
  - main
```

Mit dem Standardverhalten sucht Composia nach `docker-compose.yaml` im selben Verzeichnis.

## Schlüssel der obersten Ebene

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `name` | `string` | Ja | Eindeutiger Dienstname. |
| `project_name` | `string` | Nein | Docker-Compose-Projektname-Override. Standardmäßig ein normalisierter Dienstname. |
| `compose_files` | `[]string` | Nein | Compose-Dateipfade relativ zum Dienstverzeichnis. |
| `enabled` | `bool` | Nein | Ob der Dienst aktiv ist. Standardmäßig `true`. |
| `nodes` | `[]string` | Ja | Ziel-Node-IDs. Jede muss in `controller.nodes` existieren. |
| `infra` | `object` | Nein | Deklariert diesen Dienst als Caddy-, Rustic- oder Config-Only-Infrastruktur. |
| `network` | `object` | Nein | Caddy- und DNS-Einstellungen. |
| `update` | `object` | Nein | Image-Update-Einstellungen. |
| `data_protect` | `object` | Nein | Backup- und Restore-Datendefinitionen. |
| `backup` | `object` | Nein | Geplante Backups für geschützte Daten. |
| `migrate` | `object` | Nein | Migrationsfähige geschützte Daten. |
| `auto_deploy` | `bool` | Nein | Diesen Dienst nach Repository-Änderungen automatisch deployen. |

`compose_files`-Einträge müssen relative Pfade sein, innerhalb des Dienstverzeichnisses bleiben und dürfen nicht dupliziert sein.

## Infrastrukturdienste

### `infra.caddy`

Deklariert den Caddy-Infrastrukturdienst des Repositories.

```yaml
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
```

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `compose_service` | `string` | Name des Compose-Dienstes. Standardmäßig `caddy`. |
| `config_dir` | `string` | Caddy-Konfigurationsverzeichnis. Standardmäßig `/etc/caddy`. |

Nur ein Dienst kann als Caddy-Infrastruktur deklariert werden.

### `infra.rustic`

Deklariert den Rustic-Infrastrukturdienst des Repositories.

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

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `compose_service` | `string` | Name des Compose-Dienstes. Standardmäßig `rustic`. |
| `profile` | `string` | Rustic-Profilname. |
| `data_protect_dir` | `string` | Container-Pfad, der auf `{StateDir}/data-protect` des Agenten gemappt wird. |
| `init_args` | `[]string` | Zusätzliche Argumente, die an `rustic init` übergeben werden. Leere Einträge werden abgelehnt. |

Nur ein Dienst kann als Rustic-Infrastruktur deklariert werden.

### `infra.config`

Deklariert einen reinen Konfigurations-Infrastrukturdienst.

```yaml
infra:
  config: {}
```

Config-Only-Dienste können nicht mit `infra.caddy` oder `infra.rustic` kombiniert werden. Ihre `data_protect`-Aktionen können nur `files.copy` verwenden.

## Netzwerk

### `network.caddy`

```yaml
network:
  caddy:
    enabled: true
    source: Caddyfile
```

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `enabled` | `bool` | Nein | Aktiviert die Caddy-Verwaltung. Standardmäßig `false`. |
| `source` | `string` | Bed. | Caddyfile-Pfad relativ zum Dienstverzeichnis. Erforderlich, wenn aktiviert. |

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
    comment: Verwaltet von Composia
```

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `provider` | `string` | Ja | `cloudflare`, `alidns`, `dnspod`, `route53` oder `huaweicloud`. |
| `hostname` | `string` | Ja | DNS-Hostname. |
| `record_type` | `string` | Nein | Leer, `A`, `AAAA` oder `CNAME`. |
| `value` | `string` | Nein | DNS-Record-Wert. Multi-Node-Dienste sollten diesen explizit setzen. |
| `proxied` | `bool` | Nein | Provider-spezifischer Proxy-Umschalter, derzeit relevant für Cloudflare. |
| `ttl` | `uint32` | Nein | DNS-TTL. |
| `comment` | `string` | Nein | DNS-Record-Kommentar. |

## Image-Updates

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

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `enabled` | `bool` | Aktiviert Update-Prüfungen für diesen Dienst. |
| `auto_apply` | `bool` | Wendet erkannte Updates automatisch an. |
| `check_schedule` | `string` | Cron-Zeitplan für Update-Prüfungen. |
| `backup_before_update` | `bool` | Führt Backups vor dem Anwenden von Updates aus. |
| `backup_data` | `[]object` | Geschützte Datenelemente, die vor dem Update gesichert werden sollen. |
| `digest_pin` | `bool` | Fixiert Images per Digest. |
| `discovery_sources` | `map[string]object` | Wiederverwendbare Erkennungsquellen. Benannte Quellen können nicht auf eine andere Quelle verweisen. |
| `images` | `map[string]object` | Update-Definitionen pro Image. |

### `update.backup_data[]`

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `name` | `string` | Name des geschützten Datenelements. |
| `enabled` | `bool` | Dieses Element ein- oder ausschließen. |

### `update.images.<name>`

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `image` | `string` | Ja | Image-Repository. |
| `auto_apply` | `bool` | Nein | Per-Image-Override für automatisches Anwenden. |
| `check_schedule` | `string` | Nein | Per-Image-Prüfzeitplan. |
| `backup_before_update` | `bool` | Nein | Per-Image-Backup-Umschalter. |
| `digest_pin` | `bool` | Nein | Per-Image-Digest-Pin-Umschalter. |
| `current` | `object` | Ja | Aktuelle Versionsquelle. |
| `discovery` | `object` oder `string` | Ja | Erkennungskonfiguration oder benannte Erkennungsquellenreferenz. |
| `filter` | `object` | Bed. | Erforderlich, es sei denn, die Erkennung ist `digest`. |

### `current`

Gib genau eines davon an:

| Schlüssel | Beschreibung |
|-----|-------------|
| `tag` | Statischer aktueller Tag. |
| `env.file` + `env.key` | Liest den aktuellen Tag aus einer Env-Datei. `file` muss relativ sein und innerhalb des Dienstverzeichnisses bleiben. |
| `yaml.file` + `yaml.path` | Liest den aktuellen Tag aus einer YAML-Datei. `file` muss relativ sein und innerhalb des Dienstverzeichnisses bleiben. |

### `discovery`

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `sources` | `[]object` | Mindestens eine Quelle. |
| `combine` | `string` | Leer, `merge` oder `first_success`. |
| `include_prerelease` | `bool` | Vorabversionen einschließen. |

Erkennungsquellentypen:

| Typ | Erforderliche Schlüssel | Hinweise |
|------|---------------|-------|
| `auto` | Keine | `repo_url` ist optional und muss, wenn gesetzt, eine gültige URL sein. Muss die einzige Quelle sein. |
| `probe` | Keine | Erfordert `semver`-Filter, wenn ein Filter vorhanden ist. |
| `registry` | Keine | Registry-Tag-Erkennung. |
| `digest` | Keine | Muss die einzige Quelle sein. `filter` muss weggelassen werden. |
| `github` | `repo` | `repo` ist `owner/repo`. |
| `gitlab` | `project` | GitLab-Projekt-ID oder -Pfad. |
| `forgejo` | `repo` | `repo` ist `owner/repo`. |

### `filter`

| Typ | Erforderliche Schlüssel | Hinweise |
|------|---------------|-------|
| `semver` | Keine | `allow` kann `patch`, `minor`, `major` enthalten. |
| `date` | `format` | Datumsformat zum Parsen von Tags. |
| `regex` | `pattern`, `order` | `order` muss `numeric` oder `lexicographic` sein. |
| `latest` | Keine | Verwendet den neuesten Kandidaten. |

## Datenschutz

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

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `name` | `string` | Ja | Eindeutiger Name des Datenelements. |
| `backup` | `object` | Nein | Backup-Aktion. |
| `restore` | `object` | Nein | Restore-Aktion. |

### Datenaktion

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `strategy` | `string` | Ja | `files.copy`, `files.copy_after_stop`, `database.pgdumpall` oder `database.pgimport`. |
| `service` | `string` | Bed. | Erforderlich für `database.*`-Strategien. Name des Compose-Dienstes. |
| `include` | `[]string` | Bed. | Erforderlich für `files.*`-Strategien. `./...` oder Pfade mit `/` sind Dienstpfade; reine Namen sind Docker-Volume-Namen. |

## Backups

```yaml
backup:
  data:
    - name: db
      provider: rustic
      enabled: true
      schedule: "0 2 * * *"
```

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `name` | `string` | Ja | Muss auf einen `data_protect.data[].name` mit einer Backup-Aktion verweisen. |
| `provider` | `string` | Nein | Name des Backup-Providers. |
| `enabled` | `bool` | Nein | Aktiviert oder deaktiviert diesen Backup-Eintrag. |
| `schedule` | `string` | Nein | Cron-Zeitplan. |

## Migration

```yaml
migrate:
  data:
    - name: db
      enabled: true
```

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `name` | `string` | Ja | Muss auf einen `data_protect.data[].name` mit sowohl Backup- als auch Restore-Aktionen verweisen. |
| `enabled` | `bool` | Nein | Aktiviert oder deaktiviert die Migration für dieses Element. |
