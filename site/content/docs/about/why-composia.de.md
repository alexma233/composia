---
title: "Warum Composia"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Composia ist eine selbst gehostete Steuerungsebene für Docker Compose. Definiere deine Dienste als einfache Dateien, deploye sie auf einen oder mehrere Nodes und erhalte eine einheitliche Übersicht über deine gesamte Infrastruktur.

## Es ist kein PaaS

Im Gegensatz zu selbst gehosteten PaaS-Plattformen ersetzt Composia deine Compose-Dateien nicht durch ein eigenes Anwendungsmodell. Deine Konfiguration lebt in standardmäßigen `docker-compose.yaml`- und `composia-meta.yaml`-Dateien, die dir gehören. Die Steuerungsebene koordiniert und berichtet, aber du behältst jederzeit direkten CLI- und dateibasierten Zugriff auf jeden Node.

Schalte Composia aus und deine `docker compose`-Befehle funktionieren weiterhin. Jede Operation basiert auf standardmäßigen Docker- und Compose-Grundelementen. Es gibt keinen Lock-in.

## Wie es sich vergleicht

### Dockge, Dockman

Dockge und Dockman machen die Verwaltung einzelner Compose-Stacks komfortabler. Sie konzentrieren sich auf Single-Node-Komfort mit einer Browser-UI.

Composia teilt den dateizentrierten Ansatz, fügt aber Multi-Node-Koordination hinzu: deploye einen Dienst auf einen beliebigen konfigurierten Node, erhalte eine einheitliche Ansicht aller Dienste und Nodes in einem Dashboard und nutze ein Aufgabensystem, das jede Operation mit vollständigen Logs aufzeichnet. Die CLI ist für Skripting und Automatisierung gebaut, nicht nur für gelegentliche Nutzung.

### Dokploy, Coolify

Dokploy und Coolify sind selbst gehostete PaaS-Plattformen. Sie definieren ihr eigenes Anwendungsmodell, verwalten Build-Pipelines und abstrahieren die zugrunde liegende Infrastruktur. Sobald du sie verwendest, hängt dein Deployment-Workflow von ihren Abstraktionen ab.

Composia verfolgt den entgegengesetzten Ansatz. Es arbeitet mit deinen bestehenden Compose-Dateien in deiner eigenen Verzeichnisstruktur. Es gibt keine Build-Pipeline, kein Anwendungsmodell zu lernen und keine Abstraktionsschicht zwischen dir und Docker. Composia koordiniert die Arbeit, die Docker erledigt — es versteckt Docker nicht hinter einer Plattformabstraktion.

## Design-Entscheidungen

### Dateibasierte Konfiguration

Composia verwendet SQLite für den Laufzeitzustand und Git für die Sollzustand-Konfiguration. Die gesamte Konfiguration bleibt dateibasiert, es gibt kein PostgreSQL, kein MySQL, keine externe Datenbankabhängigkeit.

Sichere deine gesamte Composia-Installation, indem du dein Git-Repository und die SQLite-Datenbankdatei sicherst. Stelle sie auf einer neuen Maschine wieder her und du bist wieder online. Keine Datenbankmigrationen, keine Verbindungspools, kein separater Datenbankserver.

### Standard-Dateien, keine Abstraktion

Ein Dienst ist ein Verzeichnis, das `docker-compose.yaml` und `composia-meta.yaml` enthält. Du organisierst Verzeichnisse, wie du willst. Du kannst jede Datei hinzufügen, die ein Compose-Projekt benötigt: Env-Dateien, Konfigurationsvorlagen, Caddyfile, benutzerdefinierte Skripte.

Composia liest diese Dateien aus deinem Git-Repository und erstellt Dienstpakete, die Agenten mit `docker compose` ausführen. Nichts wird konvertiert, übersetzt oder umgeschrieben. Deine Compose-Dateien sind die einzige Quelle der Wahrheit.

### Git-nativ

Der Controller speichert den Sollzustand in einem Git-Repository. Jede Änderung ist ein Commit mit Autor und Nachricht. Du erhältst Versionshistorie, Rollback-Fähigkeit und die Möglichkeit, mit einem entfernten Repository zu synchronisieren. Verwende jeden Git-Workflow, den du bereits kennst.

### CLI und API zuerst

Alles, was du in der Web-UI tun kannst, kannst du mit der `composia`-CLI tun. Die CLI verwendet dieselbe öffentliche API wie das Web-Frontend. Skripting, CI-Pipelines und KI-Agenten kommunizieren mit Composia über dieselbe Schnittstelle.

Die Web-UI ist eine SvelteKit-Anwendung, die dieselbe Controller-API aufruft. Es gibt keine separate Management-API oder nur intern zugängliche Endpunkte.

## Was du bekommst

**Multi-Node-Deployment.** Definiere, auf welchen Nodes ein Dienst laufen soll, in `composia-meta.yaml`. Composia deployt den Dienst auf alle Ziel-Nodes und meldet den Status von jedem einzelnen.

**Web-Dashboard.** Durchsuche und bearbeite Repo-Dateien, sieh Live-Container-Logs, inspiziere Docker-Ressourcen (Container, Images, Netzwerke, Volumes) und öffne interaktive Terminals in laufende Container. Das Dashboard funktioniert auf Mobilgeräten.

**Backup und Restore.** Automatisierte Backups mit Rustic, mit geplanten Durchläufen, Snapshot-Management und On-Demand-Restores. Schütze Dateien, Verzeichnisse, benannte Volumes und PostgreSQL-Datenbanken.

**DNS-Management.** Automatische DNS-Record-Erstellung für Cloudflare, AliDNS, DNSPod, Route53 und Huawei Cloud. Records werden beim Deploy synchronisiert und beim Stopp entfernt.

**Reverse-Proxy.** Caddy-Integration, die dienstspezifische Caddyfile-Konfigurationen synchronisiert und Neuladungen automatisch auslöst. Generierte Konfigurationsdateien befinden sich auf dem Agenten und werden vom Caddy-Infrastrukturdienst importiert.

**Image-Updates.** Automatische Erkennung neuer Image-Versionen aus Docker-Registries und GitHub-, GitLab- oder Forgejo-Releases. Unterstützt Semver-, Datums-, Regex- und Latest-Filter. Wende Updates automatisch an oder überprüfe sie vor der Anwendung.

**Benachrichtigungen.** E-Mail- (SMTP), Telegram- und Alertmanager-Benachrichtigungen für Aufgabenergebnisse, Backup-Ereignisse, Image-Updates und Node-Statusänderungen. Filtere nach Ereignistyp und Aufgabenquelle.

**Verschlüsselte Secrets.** Age-basierte Verschlüsselung für Dienst-Secret-Dateien. Secrets werden verschlüsselt im Repository gespeichert und nur auf dem Controller entschlüsselt. Agenten erhalten entschlüsselten Inhalt in Dienstpaketen, ohne jemals auf den privaten Schlüssel zuzugreifen.

**Aufgabensystem.** Jede Operation ist eine nachverfolgte Aufgabe mit schrittweisem Fortschritt, vollständiger Log-Ausgabe und Abschlussstatus. Führe Aufgaben erneut aus, inspiziere Aufgabenschritte und verfolge Logs in Echtzeit.

**Prometheus-Metriken.** Der Controller stellt Prometheus-Metriken auf seinem HTTP-Server bereit.

## Für wen es gedacht ist

Composia ist für Power-User und Operations-Teams gebaut, die:

- Bereits Docker Compose verwenden und Multi-Node-Koordination ohne Änderung ihres Workflows wünschen.
- Klartext-Konfiguration in Git dem Durchklicken eines Webformulars vorziehen.
- Automatisierung (Backups, DNS, Updates) wünschen, aber ihre Compose-Dateien nicht einer Plattform übergeben wollen.
- Eine CLI benötigen, die sie skripten und integrieren können, nicht nur eine Browser-UI.
- Dateibasierte Konfiguration, Lock-in-freie und abhängigkeitsarme Infrastruktur schätzen.
