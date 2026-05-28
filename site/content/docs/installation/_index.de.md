---
title: "Installation"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Composia hat vier Laufzeit-Binärdateien und -Images:

| Komponente | Zweck |
|-----------|---------|
| `composia-controller` | Führt die API, die Aufgabenwarteschlange, das Sollzustand-Git-Repository und controller-seitige Integrationen aus. |
| `composia-agent` | Läuft auf jedem Docker-Node und führt Docker-Compose-Operationen aus. |
| `composia-web` | Browser-UI, die mit dem Controller kommuniziert. |
| `composia` | CLI für Terminals, Skripte und Automatisierung. |

## Wähle eine Methode

| Methode | Am besten für |
|--------|----------|
| [Docker Compose](docker-compose/) | Schnelle All-in-One-Bereitstellung mit Controller, lokalem Agent und Web-UI. |
| [Paketmanager und Binärdateien](package-managers/) | Nicht-Container-Installationen, Betriebssystempakete, Nix, AUR und manuelle Archive. |
| [Konfiguration](configuration/) | Konfigurationsdateien, Web-Umgebungsvariablen, Age-Schlüssel-Einrichtung und vollständige globale Konfigurationsreferenz. |

Für Source-Builds siehe [Entwickler-Leitfaden: Source Build](/docs/developer-guide/source-build/).
