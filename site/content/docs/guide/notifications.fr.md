---
title: "Notifications"
date: '2026-05-26T00:00:00+08:00'
weight: 70
---

Composia envoie des notifications pour les résultats de tâches, les événements de sauvegarde, les mises à jour d'images et les changements d'état des nœuds. Trois canaux de notification sont pris en charge : Alertmanager, SMTP et Telegram.

## Configuration

Tous les canaux sont configurés dans la configuration du contrôleur sous `notifications` :

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

## Types d'événements

Les types d'événements de notification suivants sont disponibles :

| Événement | Déclencheur |
|-------|---------|
| `task_failed` | Une tâche se termine avec le statut `failed`. |
| `task_cancelled` | Une tâche est annulée avant la fin. |
| `task_completed` | Une tâche se termine avec succès. |
| `task_awaiting_confirmation` | Une tâche de migration atteint l'étape de confirmation. |
| `backup_completed` | Une tâche de sauvegarde ou une sauvegarde planifiée se termine avec succès. |
| `backup_failed` | Une tâche ou une étape de sauvegarde échoue. |
| `image_update_available` | Une vérification d'image découvre une nouvelle version. |
| `image_update_applied` | Une mise à jour d'image est appliquée. |
| `node_offline` | Un nœud cesse d'envoyer des heartbeats. |
| `node_online` | Un nœud précédemment hors ligne reprend les heartbeats. |
| `alertmanager_alert` | Une alerte Alertmanager est reçue lorsque le contrôleur est configuré comme récepteur de webhook Alertmanager. |

Chaque canal peut filtrer les types d'événements qu'il doit traiter en utilisant la liste `on`. Une liste `on` vide délivre tous les types d'événements.

## Filtres de source de tâche

Les canaux SMTP et Telegram prennent en charge le filtrage par la source qui a déclenché une tâche :

| Source | Description |
|--------|-------------|
| `web` | Actions déclenchées via l'interface web. |
| `cli` | Actions déclenchées via la CLI. |
| `others` | Autres sources. |
| `schedule` | Tâches planifiées (sauvegardes, maintenance). |
| `system` | Tâches générées par le système. |
| `auto_deploy` | Tâches générées par les déclencheurs de déploiement automatique. |

Lorsque `task_sources` est vide, les notifications sont envoyées pour tous les types de sources.

## Alertmanager

Le contrôleur exécute un récepteur de webhook Alertmanager intégré. Lorsqu'il est activé, le récepteur écoute sur le chemin configuré :

```yaml
alertmanager:
  enabled: true
  listen_path: "/api/v1/alerts"
```

| Clé | Type | Description |
|-----|------|-------------|
| `enabled` | `bool` | Activé par défaut lorsque la section existe. |
| `listen_path` | `string` | Chemin HTTP pour recevoir les webhooks Alertmanager. Par défaut `/api/v1/alerts`. Doit commencer par `/` et ne pas contenir d'espaces. |

Pointez votre instance Alertmanager vers l'adresse du contrôleur avec cette URL de webhook. Les alertes sont transmises aux canaux de notification configurés selon leurs filtres d'événements.

## SMTP

SMTP délivre les notifications par e-mail :

| Clé | Type | Requis si activé | Description |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | Non | Activé par défaut lorsque la section existe. |
| `host` | `string` | Oui | Nom d'hôte du serveur SMTP. |
| `port` | `int` | Oui | Port SMTP. Doit être entre 1 et 65535. |
| `encryption` | `string` | Non | `none`, `starttls` ou `ssl_tls`. Par défaut `starttls`. |
| `username` | `string` | Non | Nom d'utilisateur d'authentification SMTP. |
| `password` | `string` | Non | Mot de passe SMTP. |
| `password_file` | `string` | Non | Lire le mot de passe depuis un fichier. |
| `from` | `string` | Oui | Adresse de l'expéditeur. |
| `to` | `[]string` | Oui | Adresses des destinataires. |
| `on` | `[]string` | Non | Types d'événements pour lesquels notifier. |
| `task_sources` | `[]string` | Non | Filtres de source de tâche. |

## Telegram

Telegram envoie des notifications à une discussion via un bot :

| Clé | Type | Requis si activé | Description |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | Non | Activé par défaut lorsque la section existe. |
| `bot_token` | `string` | Oui | Jeton du bot Telegram depuis BotFather. |
| `bot_token_file` | `string` | Non | Lire le jeton du bot depuis un fichier. |
| `chat_id` | `string` | Oui | ID de la discussion cible. |
| `on` | `[]string` | Non | Types d'événements pour lesquels notifier. |
| `task_sources` | `[]string` | Non | Filtres de source de tâche. |
