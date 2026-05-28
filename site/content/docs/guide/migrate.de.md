---
title: "Migration"
date: '2026-05-26T00:00:00+08:00'
weight: 45
---

Migriere einen Dienst von einem Node zu einem anderen unter Wahrung der Datenintegrität. Die Migrationsaufgabe orchestriert Backup-, Stopp-, Restore-, Start- und DNS-Update-Schritte über Quell- und Ziel-Nodes hinweg.

## Konfiguration

Datenelemente, die während der Migration übertragen werden, müssen sowohl eine `backup`- als auch eine `restore`-Aktion in `data_protect` haben. Deklariere sie in `migrate`:

```yaml
name: my-app
nodes:
  - main

data_protect:
  data:
    - name: uploads
      backup:
        strategy: files.copy
        include:
          - ./data/uploads
      restore:
        strategy: files.copy
        include:
          - ./data/uploads

migrate:
  data:
    - name: uploads
```

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `name` | `string` | Ja | Muss auf einen `data_protect.data[].name` mit sowohl Backup- als auch Restore-Aktionen verweisen. |
| `enabled` | `bool` | Nein | Aktiviert oder deaktiviert die Migration für dieses Element. |

## Migration ausführen

**Web-UI:**
1. Öffne die Dienst-Detailseite.
2. Verwende die Migrationssteuerung, um Quell- und Ziel-Nodes auszuwählen.
3. Klicke auf **Migrieren**.

**CLI:**

```bash
composia service migrate my-app --to edge-1
```

## Migrationsschritte

1. **Daten exportieren** — Führe eine Backup-Aufgabe auf dem Quell-Node für jedes konfigurierte Datenelement aus.
2. **Quellinstanz stoppen** — Führe `docker compose down` aus, entferne die Caddy-Konfiguration.
3. **Caddy auf der Quelle neu laden** — Entferne den Proxy-Eintrag von der Quell-Caddy-Instanz.
4. **Daten auf dem Ziel wiederherstellen** — Führe eine Restore-Aufgabe auf dem Ziel-Node für jedes Datenelement aus.
5. **Auf dem Ziel deployen** — Führe `docker compose up -d` aus, synchronisiere die Caddy-Konfiguration.
6. **Caddy auf dem Ziel neu laden** — Wende den Proxy-Eintrag auf der Ziel-Caddy-Instanz an.
7. **DNS aktualisieren** — Aktualisiere DNS-Records, um auf den Ziel-Node zu verweisen.
8. **Konfiguration schreiben** — Aktualisiere `nodes` in `composia-meta.yaml`, committe in Git.

## Überlegungen

- Der Dienst muss auf dem Quell-Node deployed sein und der Ziel-Node muss online sein.
- Die Migration verursacht kurze Ausfallzeiten. Führe sie außerhalb der Spitzenzeiten durch.
- Die Quellinstanz wird vor der Datenübertragung gestoppt, um Konsistenz zu gewährleisten.
- Für Datenbanken verwende Exportstrategien (`database.pgdumpall` / `database.pgimport`).

## Rollback

Wenn eine Migration fehlschlägt oder abgelehnt wird, löse eine Rollback-Aufgabe über die Web-UI oder CLI aus. Die Rollback-Aufgabe unterstützt diese Wiederherstellungsaktionen:

| Aktion | Beschreibung |
|--------|-------------|
| `deploy_source` | Deploye den Dienst erneut auf dem ursprünglichen Quell-Node. |
| `stop_target` | Stoppe und bereinige den Dienst auf dem Ziel-Node. |
| `rollback_dns` | Synchronisiere DNS-Records zurück zum Quell-Node. |

Wähle die Aktionen, die zum fehlgeschlagenen Schritt passen. Wenn die Migration beispielsweise fehlschlug, nachdem das Ziel deployed wurde, aber DNS noch nicht aktualisiert wurde, benötigst du möglicherweise nur `stop_target` und `deploy_source`.

**CLI:**

```bash
composia task rollback <task-id> --deploy-source --stop-target --rollback-dns
```

Lasse Flags für Aktionen weg, die du nicht benötigst.

## Siehe auch

- [Backups](/docs/guide/backups/) — Rustic-Einrichtung und Backup-Konfiguration.
- [Dienstkonfiguration](/docs/guide/service/) — `data_protect`- und `migrate`-Feldreferenz.
