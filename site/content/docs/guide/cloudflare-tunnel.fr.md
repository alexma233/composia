---
title: "Cloudflare Tunnel"
date: '2026-05-31T00:00:00+08:00'
weight: 25
---

Composia peut gérer les règles d'entrée de tunnel Cloudflare configurées à distance pour les services qui déclarent `network.cloudflare_tunnel`. La synchronisation des tunnels s'exécute comme une tâche côté contrôleur car la configuration du tunnel distant Cloudflare est un état global.

## Fonctionnement

Lorsqu'un service est déployé, mis à jour, arrêté ou synchronisé manuellement, le contrôleur crée une tâche `cloudflare_tunnel_sync`. Le worker du contrôleur l'exécute :

1. Lire toutes les métadonnées de service à la révision du dépôt de la tâche.
2. Construire les règles d'entrée de tunnel à partir des services qui déclarent `network.cloudflare_tunnel`.
3. Envoyer la liste complète des entrées à Cloudflare avec `PUT /accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations`.
4. S'assurer que chaque nom d'hôte a un CNAME proxifié pointant vers `{tunnel_id}.cfargotunnel.com`.

Cloudflare exige une règle d'entrée catch-all. Composia ajoute `http_status:404` par défaut.

## Configuration du contrôleur

Les identifiants de tunnel et les informations d'identification Cloudflare appartiennent à la configuration du contrôleur, pas aux métadonnées de service :

```yaml
controller:
  cloudflare_tunnel:
    account_id: "REPLACE"
    api_token_file: /run/secrets/cloudflare-api-token
    tunnels:
      edge:
        tunnel_id: "c1744f8b-faa1-48a4-9e5c-02ac921467fa"
        fallback_service: http_status:404
    nodes:
      main:
        tunnel: edge
```

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `account_id` | `string` | Oui | Identifiant du compte Cloudflare. |
| `api_token` / `api_token_file` | `string` | Oui | Jeton API avec les autorisations de configuration de tunnel Cloudflare et d'écriture DNS. |
| `tunnels` | `map` | Oui | Alias de tunnel mappés aux identifiants de tunnel Cloudflare. |
| `nodes` | `map` | Non | Mappage nœud-tunnel par défaut utilisé lorsqu'un service ne spécifie pas `tunnel`. |

Le `tunnel_id` n'est pas le secret du connecteur, mais il reste des métadonnées d'infrastructure au niveau du contrôleur. Les jetons ou identifiants du connecteur cloudflared doivent rester dans les secrets du nœud/agent utilisés par le service `cloudflared`.

## Déclaration de service

Déclarez l'entrée de tunnel dans le fichier `composia-meta.yaml` du service :

```yaml
network:
  cloudflare_tunnel:
    hostname: app.example.com
    service: http://app:8080
    origin_request:
      no_tls_verify: false
      http_host_header: app.internal
```

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `hostname` | `string` | Oui | Nom d'hôte public routé par le tunnel Cloudflare. |
| `service` | `string` | Oui | URL d'origine utilisée par `cloudflared`, par exemple `http://app:8080`. |
| `tunnel` | `string` | Non | Alias du tunnel. Lorsqu'il est omis, Composia le déduit du mappage du nœud cible. |
| `path` | `string` | Non | Filtre de chemin optionnel pour la règle d'entrée. |
| `origin_request` | `object` | Non | Paramètres d'origine Cloudflare. Le support initial inclut `no_tls_verify`, `http_host_header`, `origin_server_name`, `connect_timeout` et `tls_timeout`. |

## Sélection du tunnel

Composia résout le tunnel pour chaque service avec ces règles :

1. Si `network.cloudflare_tunnel.tunnel` est défini, cet alias est utilisé.
2. Si le service cible un seul nœud, Composia utilise `controller.cloudflare_tunnel.nodes.<node>.tunnel`.
3. Si le service cible plusieurs nœuds et que tous les nœuds correspondent au même tunnel, ce tunnel est utilisé.
4. Si les nœuds cibles correspondent à des tunnels différents, le service doit définir `network.cloudflare_tunnel.tunnel` explicitement.

## Comportement à l'arrêt

Lorsqu'un service arrêté avait déclaré `network.cloudflare_tunnel`, la synchronisation de tunnel suivante exclut ce service et supprime son CNAME. Les synchronisations ultérieures n'incluent que les services avec des instances en cours d'exécution, donc redéployer le service le rajoute.

## Synchronisation manuelle

Utilisez la CLI pour synchroniser la configuration de tunnel d'un service :

```bash
composia service my-app tunnel-sync
```

Cela synchronise l'état complet du tunnel configuré, pas seulement le service sélectionné, car la configuration du tunnel distant Cloudflare est mise à jour comme un seul document.
