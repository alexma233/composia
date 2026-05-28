---
title: "Reverse-Proxy"
date: '2026-05-26T00:00:00+08:00'
weight: 30
---

Composia integriert sich mit Caddy für die Reverse-Proxy-Verwaltung. Der Caddy-Infrastrukturdienst läuft als normaler Docker-Compose-Dienst, und Composia synchronisiert Caddy-Konfigurationsdateien beim Deploy und Stopp.

## Architektur

```
Controller-Repo
  ├── caddy/
  │   ├── docker-compose.yaml   (Caddy-Compose-Dienst)
  │   ├── Caddyfile             (Haupt-Caddy-Konfiguration, importiert generierte Dateien)
  │   └── composia-meta.yaml    (deklariert infra.caddy)
  ├── my-app/
  │   ├── docker-compose.yaml
  │   ├── Caddyfile             (dienstspezifische Caddy-Konfiguration)
  │   └── composia-meta.yaml    (deklariert network.caddy)
  └── ...
```

Beim Deploy kopiert Composia die Caddyfile jedes Dienstes in ein generiertes Verzeichnis und löst dann ein Caddy-Reload aus.

## Infrastruktur-Einrichtung

Deklariere genau einen Caddy-Infrastrukturdienst im Repo:

```yaml {filename="caddy/composia-meta.yaml"}
name: caddy
nodes:
  - main
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
```

Die Haupt-Caddyfile im Caddy-Dienstverzeichnis sollte die generierten Dateien importieren:

```caddy {filename="caddy/Caddyfile"}
import /etc/caddy/generated/*.caddy
```

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `compose_service` | `string` | Name des Compose-Dienstes. Standardmäßig `caddy`. |
| `config_dir` | `string` | Caddy-Konfigurationsverzeichnis im Container. Standardmäßig `/etc/caddy`. |

Nur ein Dienst im Repository kann als Caddy-Infrastruktur deklariert werden.

## Dienstkonfiguration

Aktiviere Caddy in `composia-meta.yaml` für jeden Dienst, der einen Reverse-Proxy-Eintrag benötigt, und stelle eine Caddyfile bereit:

```yaml {filename="my-app/composia-meta.yaml"}
name: my-app
nodes:
  - main
network:
  caddy:
    enabled: true
    source: Caddyfile
```

Der `source`-Pfad ist relativ zum Dienstverzeichnis und muss darin bleiben. Die Datei kann beliebig benannt werden, aber `Caddyfile` ist die Konvention.

```caddy {filename="my-app/Caddyfile"}
app.example.com {
    reverse_proxy app:8080
}
```

## Wie die Synchronisation funktioniert

Während einer Deploy- oder Update-Aufgabe führt der Agent einen Caddy-Sync-Schritt nach `compose up` aus:

1. Liest `network.caddy.source` aus der `composia-meta.yaml` des Dienstes.
2. Kopiert die Quelldatei nach `<agent_state_dir>/caddy/generated/<service_dir>.caddy`.
3. Führt `docker compose exec <caddy_service> caddy reload --config <Caddyfile> --adapter caddyfile` aus.

Der generierte Dateiname wird vom Dienstverzeichnisnamen abgeleitet. Für `my-app` lautet die Datei `my-app.caddy`.

Während einer Stopp-Aufgabe wird die generierte Caddy-Datei entfernt.

## Caddy-Sync-Aufgabe

Eine eigenständige `caddy_sync`-Aufgabe erstellt die Caddy-Konfiguration neu, ohne Dienste zu deployen. Sie kann in zwei Modi arbeiten:

**Vollständiger Neuaufbau** (`full_rebuild: true`): Löscht alle generierten `.caddy`-Dateien aus dem generierten Verzeichnis und synchronisiert dann alle Caddy-verwalteten Dienste neu.

**Gezielte Synchronisation**: Synchronisiert nur die angegebenen Dienstverzeichnisse.

Über die Web-UI oder CLI auslösen:

```bash
composia service caddy-sync my-app
```

## Caddy-Reload-Aufgabe

Eine `caddy_reload`-Aufgabe führt `caddy reload` im Caddy-Container aus, ohne Dateien zu ändern. Verwende sie nach manueller Bearbeitung der Haupt-Caddyfile:

```bash
composia node reload-caddy main
```

## Agent-Konfiguration

Die Agent-Konfiguration hat einen optionalen Caddy-Abschnitt:

```yaml
agent:
  caddy:
    generated_dir: "/data/state-agent/caddy/generated"
```

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `generated_dir` | `string` | Generiertes Caddy-Konfigurationsverzeichnis. Standardmäßig `<state_dir>/caddy/generated`. |

Das generierte Verzeichnis muss sich in einem Pfad befinden, den der Caddy-Container lesen kann. Der Caddy-Compose-Dienst muss ein Volume haben, das dieses Verzeichnis auf den Pfad einbindet, der in der Haupt-Caddyfile importiert wird.
