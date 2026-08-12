---
title: "Lokale Entwicklung"
date: '2026-07-06T00:00:00+08:00'
weight: 10
---

Verwende `mise` als zentralen Einstiegspunkt für die lokale Entwicklung.

## Einrichtung

```bash
mise install
mise run setup
```

`mise run setup` bereitet lokale Dateien unter `dev/` vor, erstellt Entwicklungstokens und Age-Schlüsseldateien und initialisiert bei Bedarf das Controller-Repository. Vorhandene lokale Dateien werden nicht überschrieben.

## App-Stack

Starte Controller, Agent und Web-UI:

```bash
mise run dev
```

- Controller: <http://127.0.0.1:7001>
- Web-UI: <http://127.0.0.1:5173>

Logs verfolgen oder den Stack stoppen:

```bash
mise run dev:logs
mise run dev:down
```

## Dokumentation

Die Dokumentation wird standardmäßig nicht mit dem App-Stack gestartet.

```bash
mise run dev:docs # nur Dokumentation auf :5174
mise run dev:all  # App und Dokumentation
```

## Prüfungen

Führe vor dem Commit die normalen lokalen Prüfungen aus:

```bash
mise run check
```

Langsamere Backend-Prüfungen:

```bash
mise run check:full
```

## Protobuf-Generierung

Generiere nach Änderungen unter `proto/**` den eingecheckten Protobuf-Code und die API-Dokumentation neu:

```bash
mise run gen
```

Committe die generierten Dateien unter `gen/go/`, `web/src/lib/gen/` und `site/content/docs/developer-guide/api/`.

## E2E

Führe alle lokalen E2E-Tests gegen den echten Controller- und Agent-Fixture-Stack aus:

```bash
mise run e2e
```
