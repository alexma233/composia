---
title: "Image-Updates"
date: '2026-05-26T00:00:00+08:00'
weight: 60
---

Composia erkennt neue Image-Tags und kann Updates automatisch anwenden. Image-Check-Aufgaben laufen auf dem Agenten und melden die Ergebnisse an den Controller.

## Wie es funktioniert

Der Controller plant regelmäßige `image_check`-Aufgaben gemäß der Update-Konfiguration des Dienstes. Jede Prüfung:

1. Der Agent lädt das Dienstpaket herunter.
2. Liest `docker compose config --format json`, um laufende Images zu erkennen.
3. Meldet lokale und entfernte Digests für jedes Image.
4. Für Images, die in `update.images` konfiguriert sind, prüft es auf neue Kandidaten-Tags unter Verwendung der konfigurierten Erkennungsquellen.
5. Meldet die Ergebnisse an den Controller. Der Controller zeichnet verfügbare Updates auf und kann sie automatisch anwenden.

## Controller-Standardwerte

Globale Standardwerte werden in der Controller-Konfiguration gesetzt:

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
        api_url: "https://api.github.com"
```

Der dienstspezifische `update`-Abschnitt überschreibt diese Standardwerte.

## Dienstkonfiguration

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

### `update` oberste Ebene

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `enabled` | `bool` | Aktiviert Update-Prüfungen für diesen Dienst. |
| `auto_apply` | `bool` | Wendet erkannte Updates automatisch an. |
| `check_schedule` | `string` | Cron-Zeitplan für Update-Prüfungen. |
| `backup_before_update` | `bool` | Führt ein Backup vor dem Anwenden eines Updates aus. |
| `backup_data` | `[]object` | Geschützte Datenelemente, die vor dem Update gesichert werden sollen. Jedes Element hat einen `name` und optional `enabled`. |
| `digest_pin` | `bool` | Fixiert Images per Digest für Reproduzierbarkeit. |
| `discovery_sources` | `map[string]object` | Benannte wiederverwendbare Erkennungskonfigurationen. |
| `images` | `map[string]object` | Update-Konfiguration pro Image. Schlüssel sind beliebige Namen, die den zu prüfenden Images entsprechen. |

### `images.<name>`

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `image` | `string` | Ja | Vollständige Image-Referenz, zum Beispiel `ghcr.io/example/api`. |
| `auto_apply` | `bool` | Nein | Per-Image-Override für automatisches Anwenden. |
| `check_schedule` | `string` | Nein | Per-Image-Prüfzeitplan. |
| `backup_before_update` | `bool` | Nein | Per-Image-Backup-Umschalter. |
| `digest_pin` | `bool` | Nein | Per-Image-Digest-Pin-Umschalter. |
| `current` | `object` | Ja | Wie die aktuell deployte Version gefunden wird. |
| `discovery` | `object` oder `string` | Ja | Erkennungskonfiguration oder Verweis auf einen benannten `discovery_sources`-Eintrag. |
| `filter` | `object` | Bed. | Versionsfilter. Erforderlich, es sei denn, der Erkennungsmodus ist `digest`. |

### `current`

Genau eine dieser Quellen muss angegeben werden:

**Statischer Tag:**

```yaml
current:
  tag: "v1.2.3"
```

**Umgebungsdatei:**

```yaml
current:
  env:
    file: .env
    key: APP_VERSION
```

Der `file`-Pfad ist relativ zum Dienstverzeichnis. Composia liest die Datei, sucht nach `SCHLÜSSEL=WERT`-Zeilen und extrahiert den Wert.

**YAML-Datei:**

```yaml
current:
  yaml:
    file: values.yaml
    path: app.image.tag
```

Der `path` ist ein durch Punkte getrennter Pfad in den YAML-Dokumentbaum. Der Wert an diesem Pfad muss ein Skalar sein.

### Erkennung

Erkennungsquellen können sein:

**Benannte Referenz** auf einen `discovery_sources`-Eintrag:

```yaml
discovery: upstream-gh
```

**Inline-Definition:**

```yaml
discovery:
  sources:
    - type: probe
  combine: first_success
  include_prerelease: false
```

Erkennungsquellentypen:

| Typ | Erforderliche Schlüssel | Verhalten |
|------|---------------|----------|
| `probe` | Keine | Semver-Sondierung: Sucht nach höheren Versionen durch Sondieren von Registry-Manifesten. Erfordert einen `semver`-Filter. |
| `registry` | Keine | Listet alle Tags aus der Image-Registry auf. |
| `auto` | Keine (optional `repo_url`) | Versucht `probe`, dann `registry` als zusammengeführte Erkennung. Muss die einzige Quelle in einer Erkennungskonfiguration sein. |
| `digest` | Keine | Vergleicht nur den entfernten Digest mit dem lokalen Digest. Kein Tag-Vergleich. `filter` muss weggelassen werden. Muss die einzige Quelle sein. |
| `github` | `repo` (`owner/repo`) | Fragt GitHub-Releases ab. Wird auf der Controller-Seite verarbeitet. |
| `gitlab` | `project` | Fragt GitLab-Releases ab. Wird auf der Controller-Seite verarbeitet. |
| `forgejo` | `repo` (`owner/repo`) | Fragt Forgejo-Releases ab. Wird auf der Controller-Seite verarbeitet. |

`combine` akzeptiert `merge` (Vereinigung aller Quellergebnisse) oder `first_success` (erste Quelle, die Ergebnisse liefert, gewinnt).

`include_prerelease` schließt Vorabversionen in GitHub-, GitLab- und Forgejo-Release-Abfragen ein.

### Filter

| Typ | Erforderliche Schlüssel | Verhalten |
|------|---------------|----------|
| `semver` | Keine | Filtert nach semantischer Version. `allow` kann `patch`, `minor`, `major` enthalten. |
| `date` | `format` | Parst Tags als Daten mit dem angegebenen Format. |
| `regex` | `pattern`, `order` | Filtert nach Regex. Order muss `numeric` oder `lexicographic` sein. |
| `latest` | Keine | Nimmt den neuesten Tag ohne Filterung. |

#### Semver-Sondierung

Mit `type: probe` und einem `semver`-Filter sucht Composia nach Kandidaten-Tags, indem es Versionsnummern konstruiert und prüft, ob das entsprechende Registry-Manifest existiert. Es sondiert Patch-, Minor- und Major-Sprünge gemäß der `allow`-Liste unter Verwendung einer exponentiellen Suche mit binärer Verfeinerung, um die höchste verfügbare Version zu finden.

## Digest-Modus

Wenn alle Erkennungsquellen in einer Konfiguration `type: digest` haben, wird kein Tag-Vergleich durchgeführt. Composia vergleicht nur den entfernten Image-Digest mit dem lokalen Digest:

```yaml
discovery:
  sources:
    - type: digest
```

Wenn `digest` als Erkennungsmodus gesetzt ist, muss `filter` weggelassen werden. Wenn ein Digest abweicht, wird ein Update als verfügbar betrachtet.

## Image-Beobachtungen

Während Deploy- und Update-Aufgaben sammelt der Agent auch Image-Beobachtungen für alle Compose-Dienste. Diese umfassen lokale und entfernte Digests, die unabhängig davon an den Controller gemeldet werden, ob `update.images` konfiguriert ist. Dies bietet Einblick in den Image-Zustand in der Web-UI und CLI.
