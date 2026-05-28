---
title: "Reverse proxy"
date: '2026-05-26T00:00:00+08:00'
weight: 30
---

Composia s'intègre avec Caddy pour la gestion du reverse proxy. Le service d'infrastructure Caddy s'exécute comme un service Docker Compose normal, et Composia synchronise les fichiers de configuration Caddy au déploiement et à l'arrêt.

## Architecture

```
Controller repo
  ├── caddy/
  │   ├── docker-compose.yaml   (service Compose Caddy)
  │   ├── Caddyfile             (configuration Caddy principale, importe les fichiers générés)
  │   └── composia-meta.yaml    (déclare infra.caddy)
  ├── my-app/
  │   ├── docker-compose.yaml
  │   ├── Caddyfile             (configuration Caddy spécifique au service)
  │   └── composia-meta.yaml    (déclare network.caddy)
  └── ...
```

Au moment du déploiement, Composia copie le Caddyfile de chaque service dans un répertoire généré, puis déclenche un rechargement Caddy.

## Configuration de l'infrastructure

Déclarez exactement un service d'infrastructure Caddy dans le dépôt :

```yaml {filename="caddy/composia-meta.yaml"}
name: caddy
nodes:
  - main
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
```

Le Caddyfile principal dans le répertoire du service Caddy doit importer les fichiers générés :

```caddy {filename="caddy/Caddyfile"}
import /etc/caddy/generated/*.caddy
```

| Clé | Type | Description |
|-----|------|-------------|
| `compose_service` | `string` | Nom du service Compose. Par défaut `caddy`. |
| `config_dir` | `string` | Répertoire de configuration Caddy à l'intérieur du conteneur. Par défaut `/etc/caddy`. |

Un seul service dans le dépôt peut être déclaré comme infrastructure Caddy.

## Configuration du service

Pour chaque service qui nécessite une entrée de reverse proxy, activez Caddy dans `composia-meta.yaml` et fournissez un Caddyfile :

```yaml {filename="my-app/composia-meta.yaml"}
name: my-app
nodes:
  - main
network:
  caddy:
    enabled: true
    source: Caddyfile
```

Le chemin `source` est relatif au répertoire du service et doit rester à l'intérieur. Le fichier peut avoir n'importe quel nom, mais `Caddyfile` est la convention.

```caddy {filename="my-app/Caddyfile"}
app.example.com {
    reverse_proxy app:8080
}
```

## Fonctionnement de la synchronisation

Pendant une tâche de déploiement ou de mise à jour, l'agent exécute une étape de synchronisation Caddy après `compose up` :

1. Lire `network.caddy.source` depuis le fichier `composia-meta.yaml` du service.
2. Copier le fichier source vers `<agent_state_dir>/caddy/generated/<service_dir>.caddy`.
3. Exécuter `docker compose exec <caddy_service> caddy reload --config <Caddyfile> --adapter caddyfile`.

Le nom du fichier généré est dérivé du nom du répertoire du service. Pour `my-app`, le fichier est `my-app.caddy`.

Pendant une tâche d'arrêt, le fichier Caddy généré est supprimé.

## Tâche de synchronisation Caddy

Une tâche autonome `caddy_sync` reconstruit la configuration Caddy sans déployer de services. Elle peut fonctionner en deux modes :

**Reconstruction complète** (`full_rebuild: true`) : supprime tous les fichiers `.caddy` générés du répertoire généré, puis resynchronise tous les services gérés par Caddy.

**Synchronisation ciblée** : synchronise uniquement les répertoires de service spécifiés.

Déclenchez via l'interface web ou la CLI :

```bash
composia service caddy-sync my-app
```

## Tâche de rechargement Caddy

Une tâche `caddy_reload` exécute `caddy reload` à l'intérieur du conteneur Caddy sans modifier aucun fichier. Utilisez-la après avoir modifié manuellement le Caddyfile principal :

```bash
composia node reload-caddy main
```

## Configuration de l'agent

La configuration de l'agent a une section Caddy optionnelle :

```yaml
agent:
  caddy:
    generated_dir: "/data/state-agent/caddy/generated"
```

| Clé | Type | Description |
|-----|------|-------------|
| `generated_dir` | `string` | Répertoire de configuration Caddy généré. Par défaut `<state_dir>/caddy/generated`. |

Le répertoire généré doit être à l'intérieur d'un chemin que le conteneur Caddy peut lire. Le service Compose Caddy doit avoir un volume montant ce répertoire vers le chemin importé dans le Caddyfile principal.
