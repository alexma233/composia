---
title: "Backups"
date: '2026-05-26T00:00:00+08:00'
weight: 40
---

Composia automatisiert Backups durch Rustic. Backup- und Restore-Aufgaben laufen auf dem Agenten, während der Controller die Laufzeitkonfiguration generiert.

## Architektur

Backups erfordern einen Rustic-Infrastrukturdienst. Das Repository muss genau einen Dienst mit `infra.rustic` deklarieren:

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

Der Rustic-Compose-Dienst ist ein normaler Docker-Container, der die `rustic`-Binary ausführt. Er sollte ein Volume für das Data-Protect-Verzeichnis haben.

## Controller-Konfiguration

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

| Schlüssel | Beschreibung |
|-----|-------------|
| `backup.default_schedule` | Standard-Cron-Zeitplan für Dienst-Backups. |
| `rustic.main_nodes` | Node-IDs, auf denen Rustic-Operationen ausgeführt werden. Jede muss auf einen konfigurierten Node verweisen. |
| `rustic.maintenance.forget_schedule` | Cron-Zeitplan für `rustic forget`. |
| `rustic.maintenance.prune_schedule` | Cron-Zeitplan für `rustic prune`. |

## Dienst-Datenschutz

Definiere, was gesichert werden soll, in `composia-meta.yaml` unter `data_protect`:

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

### Datenstrategien

| Strategie | Zweck |
|----------|---------|
| `files.copy` | Kopiert Dateien und Verzeichnisse. Für live-lesbare Daten. |
| `files.copy_after_stop` | Stoppt das Compose-Projekt, kopiert Dateien, startet neu. Für Daten, die ruhiggestellt werden müssen. |
| `database.pgdumpall` | Führt `pg_dumpall` im Compose-Dienst aus. Erfordert, dass `service` gesetzt ist. |
| `database.pgimport` | Stellt einen PostgreSQL-Dump über `psql` wieder her. Erfordert, dass `service` gesetzt ist. |

### Datenaktionsfelder

| Schlüssel | Typ | Erforderlich für | Beschreibung |
|-----|------|-------------|-------------|
| `strategy` | `string` | Alle | Backup- oder Restore-Strategie. |
| `service` | `string` | `database.*` | Name des Compose-Dienstes. |
| `include` | `[]string` | `files.*` | Pfade zum Einbeziehen, relativ zum Dienstverzeichnis. Bleibt innerhalb des Dienststamms. |

### Include-Pfadtypen

Pfade können sich beziehen auf:

- **Dienstpfade**: Dateien oder Verzeichnisse innerhalb des Dienstverzeichnisses. Direkt kopiert.
- **Benannte Volumes**: Docker-Volume-Namen. Gesichert durch Starten eines temporären Containers, der das Volume einbindet.

## Backup-Zeitpläne

Aktiviere geplante Backups für geschützte Datenelemente:

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

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `name` | `string` | Ja | Muss auf einen `data_protect.data[].name` verweisen, der eine Backup-Aktion hat. |
| `provider` | `string` | Nein | Name des Backup-Providers. |
| `enabled` | `bool` | Nein | Aktiviert oder deaktiviert dieses Backup. |
| `schedule` | `string` | Nein | Cron-Ausdruck. `"none"` deaktiviert die Planung, behält aber den Eintrag. |

Wenn `schedule` gesetzt ist, plant der Controller wiederkehrende `backup`-Aufgaben. Der `backup.default_schedule` des Controllers wird als Fallback verwendet, wenn ein Diensteintrag keinen eigenen Zeitplan angibt.

## Wie ein Backup abläuft

Eine Backup-Aufgabe führt diese Schritte auf dem Agenten aus:

1. **Rendern**: Lädt das Dienstpaket und das Rustic-Paket vom Controller herunter. Liest `.composia-backup.json`, das vom Controller generiert wurde.
2. **Backup**: Für jedes Datenelement in der Laufzeitkonfiguration:
   - Stellt die Daten gemäß der Backup-Strategie bereit (`files.copy`, `files.copy_after_stop`, `database.pgdumpall`).
   - Führt `docker compose run rustic backup` mit Tags aus, die den Dienst und das Datenelement identifizieren.
   - Meldet das Ergebnis (Snapshot-ID) an den Controller.
3. Die Aufgabe endet, wenn alle Elemente gesichert sind.

Backup-Artefakte werden durch Rustic-Snapshot-IDs identifiziert. Tags enthalten `composia-service:<name>` und `composia-data:<name>` für spätere Restore- und Forget-Operationen.

## Restore

Löse ein Restore über die Web-UI auf der Backups-Seite oder per CLI aus:

```bash
composia backup restore <backup-id>
```

Der Restore-Prozess:

1. **Rendern**: Lädt das Dienstpaket und das Rustic-Paket herunter. Liest `.composia-restore.json`.
2. **Restore**: Für jedes Element:
   - Führt `docker compose run rustic restore <snapshot_id> <target_dir>` aus.
   - Wendet die wiederhergestellten Daten gemäß der Restore-Strategie an:
     - `files.copy`: Ersetzt Dateien im Dienstverzeichnis.
     - `files.copy_after_stop`: Stoppt Compose, ersetzt Dateien, startet Compose neu.
     - `database.pgimport`: Führt `docker compose exec <service> psql` mit dem wiederhergestellten SQL-Dump aus.

## Rustic-Wartung

Wartungsaufgaben verwenden den Rustic-Infrastrukturdienst:

- **`rustic_init`**: Führt `docker compose run rustic init` aus, um das Rustic-Repository zu initialisieren. Einmal pro Rustic-Setup verwenden.
- **`rustic_forget`**: Führt `docker compose run rustic forget` mit Tag-Filtern aus. Begrenzt auf einen Dienst, ein Datenelement oder repo-weit.
- **`rustic_prune`**: Führt `docker compose run rustic prune` aus, um nicht referenzierte Daten zu entfernen.

Löse Wartungsaufgaben über die Web-UI oder CLI aus:

```bash
composia node init-rustic main
composia node forget-rustic main
composia node prune-rustic main
```

## Siehe auch

- [Dienstkonfiguration](/docs/guide/service/) — Datenschutz und Backup-Planung.
- [Migration](/docs/guide/migrate/) — Verschiebe Dienste zwischen Nodes mit Datenerhalt durch Backups.
