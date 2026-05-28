---
title: "Benachrichtigungen"
date: '2026-05-26T00:00:00+08:00'
weight: 70
---

Composia sendet Benachrichtigungen für Aufgabenergebnisse, Backup-Ereignisse, Image-Updates und Node-Statusänderungen. Drei Benachrichtigungskanäle werden unterstützt: Alertmanager, SMTP und Telegram.

## Konfiguration

Alle Kanäle werden in der Controller-Konfiguration unter `notifications` konfiguriert:

```yaml
controller:
  notifications:
    alertmanager:
      enabled: true
      listen_path: "/api/v1/alerts"
    smtp:
      enabled: true
      host: "smtp.example.com"
      port: 587
      encryption: starttls
      username: "alerts@example.com"
      password: "REPLACE"
      from: "alerts@example.com"
      to:
        - "admin@example.com"
      on:
        - task_failed
        - backup_failed
      task_sources:
        - web
        - cli
    telegram:
      enabled: true
      bot_token: "REPLACE"
      chat_id: "REPLACE"
      on:
        - task_completed
```

## Ereignistypen

Die folgenden Benachrichtigungsereignistypen sind verfügbar:

| Ereignis | Auslöser |
|-------|---------|
| `task_failed` | Eine Aufgabe endet mit Status `failed`. |
| `task_cancelled` | Eine Aufgabe wird vor Abschluss abgebrochen. |
| `task_completed` | Eine Aufgabe wird erfolgreich abgeschlossen. |
| `task_awaiting_confirmation` | Eine Migrationsaufgabe erreicht den Bestätigungsschritt. |
| `backup_completed` | Eine Backup-Aufgabe oder ein geplantes Backup wird erfolgreich abgeschlossen. |
| `backup_failed` | Eine Backup-Aufgabe oder ein Schritt schlägt fehl. |
| `image_update_available` | Eine Image-Prüfung entdeckt eine neue Version. |
| `image_update_applied` | Ein Image-Update wird angewendet. |
| `node_offline` | Ein Node sendet keine Heartbeats mehr. |
| `node_online` | Ein zuvor offline Node sendet wieder Heartbeats. |
| `alertmanager_alert` | Eine Alertmanager-Benachrichtigung wird empfangen, wenn der Controller als Alertmanager-Webhook-Empfänger konfiguriert ist. |

Jeder Kanal kann mit der `on`-Liste filtern, welche Ereignistypen er behandeln soll. Eine leere `on`-Liste liefert alle Ereignistypen.

## Aufgabenquellenfilter

SMTP- und Telegram-Kanäle unterstützen die Filterung nach der Quelle, die eine Aufgabe ausgelöst hat:

| Quelle | Beschreibung |
|--------|-------------|
| `web` | Aktionen, die über die Web-UI ausgelöst wurden. |
| `cli` | Aktionen, die über die CLI ausgelöst wurden. |
| `others` | Andere Quellen. |
| `schedule` | Geplante Aufgaben (Backups, Wartung). |
| `system` | Systemgenerierte Aufgaben. |
| `auto_deploy` | Aufgaben, die durch Auto-Deploy-Trigger generiert wurden. |

Wenn `task_sources` leer ist, werden Benachrichtigungen für alle Quelltypen gesendet.

## Alertmanager

Der Controller betreibt einen eingebetteten Alertmanager-Webhook-Empfänger. Wenn aktiviert, lauscht der Empfänger auf dem konfigurierten Pfad:

```yaml
alertmanager:
  enabled: true
  listen_path: "/api/v1/alerts"
```

| Schlüssel | Typ | Beschreibung |
|-----|------|-------------|
| `enabled` | `bool` | Standardmäßig aktiviert, wenn der Abschnitt existiert. |
| `listen_path` | `string` | HTTP-Pfad zum Empfangen von Alertmanager-Webhooks. Standardmäßig `/api/v1/alerts`. Muss mit `/` beginnen und darf keine Leerzeichen enthalten. |

Richte deine Alertmanager-Instanz mit dieser Webhook-URL auf die Adresse des Controllers aus. Benachrichtigungen werden gemäß ihren Ereignisfiltern an konfigurierte Benachrichtigungskanäle weitergeleitet.

## SMTP

SMTP liefert Benachrichtigungen per E-Mail:

| Schlüssel | Typ | Erforderlich wenn aktiviert | Beschreibung |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | Nein | Standardmäßig aktiviert, wenn der Abschnitt existiert. |
| `host` | `string` | Ja | SMTP-Server-Hostname. |
| `port` | `int` | Ja | SMTP-Port. Muss zwischen 1 und 65535 liegen. |
| `encryption` | `string` | Nein | `none`, `starttls` oder `ssl_tls`. Standardmäßig `starttls`. |
| `username` | `string` | Nein | SMTP-Authentifizierungs-Benutzername. |
| `password` | `string` | Nein | SMTP-Passwort. |
| `password_file` | `string` | Nein | Passwort aus einer Datei lesen. |
| `from` | `string` | Ja | Absenderadresse. |
| `to` | `[]string` | Ja | Empfängeradressen. |
| `on` | `[]string` | Nein | Ereignistypen, für die benachrichtigt werden soll. |
| `task_sources` | `[]string` | Nein | Aufgabenquellenfilter. |

## Telegram

Telegram sendet Benachrichtigungen über einen Bot an einen Chat:

| Schlüssel | Typ | Erforderlich wenn aktiviert | Beschreibung |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | Nein | Standardmäßig aktiviert, wenn der Abschnitt existiert. |
| `bot_token` | `string` | Ja | Telegram-Bot-Token von BotFather. |
| `bot_token_file` | `string` | Nein | Bot-Token aus einer Datei lesen. |
| `chat_id` | `string` | Ja | Ziel-Chat-ID. |
| `on` | `[]string` | Nein | Ereignistypen, für die benachrichtigt werden soll. |
| `task_sources` | `[]string` | Nein | Aufgabenquellenfilter. |
