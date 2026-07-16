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

Der Rustic-Compose-Dienst ist ein normaler Docker-Container, der die `rustic`-Binary ausführt. Er muss ein Volume haben, das `{StateDir}/data-protect` des Agenten auf den in `data_protect_dir` gesetzten Pfad abbildet.

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
| `files.copy` | Bind-mountet Quellpfade read-only in den Rustic-Container für Backup. Für live-lesbare Daten. |
| `files.copy_after_stop` | Stoppt das Compose-Projekt, bind-mountet Quellpfade, sichert und startet neu. Für Daten, die ruhiggestellt werden müssen. |
| `database.pgdumpall` | Führt `pg_dumpall` im Compose-Dienst aus. Erfordert, dass `service` gesetzt ist. |
| `database.pgimport` | Stellt einen PostgreSQL-Dump über `psql` wieder her. Erfordert, dass `service` gesetzt ist. |

### Datenaktionsfelder

| Schlüssel | Typ | Erforderlich für | Beschreibung |
|-----|------|-------------|-------------|
| `strategy` | `string` | Alle | Backup- oder Restore-Strategie. |
| `service` | `string` | `database.*` | Name des Compose-Dienstes. |
| `include` | `[]string` | `files.*` | Einzubeziehende Pfade. Dienstpfade (relativ zum Dienststamm, beginnend mit `./` oder mit `/`) oder Docker-Volume-Namen (reiner Name ohne Pfadtrenner). |

### Include-Pfadtypen

Pfade können sich beziehen auf:

- **Dienstpfade**: Dateien oder Verzeichnisse innerhalb des Dienstverzeichnisses. Werden per `-v` read-only in den Rustic-Container bind-gemountet.
- **Benannte Volumes**: Docker-Volume-Namen. Werden per `-v` read-only in den Rustic-Container bind-gemountet (kein temporärer Container nötig).

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
   - `files.*`: Erstellt ein leeres Staging-Verzeichnis unter `data-protect`, fügt `-v` Bind-Mounts für jeden Include-Pfad oder jedes Volume hinzu und führt `docker compose run -v ... rustic backup` mit Tags aus, die den Dienst und das Datenelement identifizieren. Keine Daten werden in das Agent-State-Verzeichnis kopiert.
   - `database.pgdumpall`: Führt `docker compose exec <service> pg_dumpall` aus, schreibt den SQL-Dump in eine Staging-Datei unter `data-protect` und führt dann `docker compose run rustic backup` auf dem Staging-Verzeichnis aus.
   - Meldet das Ergebnis (Snapshot-ID) an den Controller.
3. Die Aufgabe endet, wenn alle Elemente gesichert sind.

Backup-Artefakte werden durch Rustic-Snapshot-IDs identifiziert. Tags enthalten `composia-service:<name>` und `composia-data:<name>` für spätere Restore- und Forget-Operationen.

## Restore

Löse ein Restore über die Web-UI auf der Backups-Seite oder per CLI aus:

```bash
composia backup restore --wait --follow --timeout 30m main <backup-id>
```

The first argument is the target node. Use `--wait --follow` to block until the restore finishes and stream task logs.

Der Restore-Prozess:

1. **Rendern**: Lädt das Dienstpaket und das Rustic-Paket herunter. Liest `.composia-restore.json`.
2. **Restore**: Für jedes Element:
   - `files.copy` / `files.copy_after_stop`: Bereinigt die Restore-Ziele (Ziele müssen existieren), erstellt ein leeres Staging-Verzeichnis unter `data-protect`, bind-mountet jeden Zielpfad oder jedes Docker-Volume in den Staging-Baum und führt dann `docker compose run -v ... rustic restore <snapshot_id> <staging_dir>` aus. Wiederhergestellte Daten werden direkt in die Zielorte geschrieben — kein nachträglicher Kopierschritt.
   - `files.copy_after_stop`: Stoppt zusätzlich das Compose-Projekt vor dem Restore und startet es danach neu.
   - `database.pgimport`: Führt `docker compose run rustic restore <snapshot_id>` in ein Staging-Verzeichnis aus und führt dann `docker compose exec <service> psql` mit dem wiederhergestellten SQL-Dump aus.

Restore-Ziele für `files.*`-Dienstpfade müssen auf dem Agenten bereits existieren (wird verwendet, um die Bind-Mount-Semantik für Datei/Verzeichnis zu bestimmen). Docker-Volume-Ziele werden vor dem Restore geleert.

## Rustic-Wartung

Wartungsaufgaben verwenden den Rustic-Infrastrukturdienst:

- **`rustic_init`**: Führt `docker compose run rustic init` aus, um das Rustic-Repository zu initialisieren. Einmal pro Rustic-Setup verwenden.
- **`rustic_forget`**: Führt `docker compose run rustic forget` mit Tag-Filtern aus. Begrenzt auf einen Dienst, ein Datenelement oder repo-weit.
- **`rustic_prune`**: Führt `docker compose run rustic prune` aus, um nicht referenzierte Daten zu entfernen.

Löse Wartungsaufgaben über die Web-UI oder CLI aus:

```bash
composia rustic init --wait --follow main
composia rustic forget --service my-app --data uploads --yes --wait --follow main
composia rustic prune --yes --wait --follow main
```

Use `--wait --follow` when you want the CLI to wait for the maintenance task and stream logs.

## Siehe auch

- [Dienstkonfiguration](/docs/guide/service/) — Datenschutz und Backup-Planung.
- [Migration](/docs/guide/migrate/) — Verschiebe Dienste zwischen Nodes mit Datenerhalt durch Backups.
