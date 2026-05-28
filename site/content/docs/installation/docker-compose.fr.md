---
title: "Docker Compose"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

La pile Docker Compose exécute le contrôleur, un agent local et l'interface web à partir du fichier [`docker-compose.yaml`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/docker-compose.yaml) canonique.

## Télécharger les fichiers

Vous n'avez pas besoin de cloner tout le dépôt pour une installation Docker Compose. Téléchargez le fichier Compose et le modèle d'environnement :

```bash
curl -LO https://forgejo.alexma.top/alexma233/composia/raw/branch/main/docker-compose.yaml
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/.env.example -o .env
```

Modifiez `.env` avant de démarrer la pile. Le modèle est groupé par rôle ; pour la pile tout-en-un, conservez tous les groupes. Voir [Configuration](../configuration/) pour la signification de chaque variable.

Trouvez l'ID de groupe du socket Docker sur l'hôte :

```bash
stat -c '%g' /var/run/docker.sock
```

Définissez `DOCKER_SOCK_GID` à cette valeur.

## Chemin du dépôt de l'agent

`COMPOSIA_AGENT_REPO_DIR` est monté comme :

```yaml
- ${COMPOSIA_AGENT_REPO_DIR}:${COMPOSIA_AGENT_REPO_DIR}
```

Le chemin hôte et le chemin conteneur doivent être identiques. L'agent invoque le démon Docker de l'hôte, et le démon Docker de l'hôte résout les montages bind à partir du système de fichiers hôte. Si le dépôt de services est monté à un chemin différent à l'intérieur du conteneur de l'agent, Docker Compose peut générer des chemins hôte qui n'existent pas.

Utilisez le même chemin absolu des deux côtés, par exemple :

```bash
COMPOSIA_AGENT_REPO_DIR=/srv/composia/repo-agent
```

## `config.yaml` de base

Créez `config.yaml` dans `COMPOSIA_CONFIG_DIR`. Le fichier Docker Compose monte ce répertoire vers `/app/configs`.

```yaml {filename="config.yaml"}
controller:
  listen_addr: ":7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  access_tokens:
    - name: "web"
      token: "REPLACE_WITH_WEB_ACCESS_TOKEN"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
```

Définissez `WEB_CONTROLLER_ACCESS_TOKEN` dans `.env` à la même valeur que `controller.access_tokens[0].token`.

## Mot de passe web

`WEB_LOGIN_PASSWORD_HASH` doit être un hachage de mot de passe Argon2. Utilisez un outil de hachage de mot de passe compatible Argon2 et collez le hachage encodé complet dans `.env`.

Générez `WEB_SESSION_SECRET` avec n'importe quel générateur aléatoire cryptographiquement sécurisé, par exemple :

```bash
openssl rand -hex 32
```

## Démarrage

```bash
docker compose up -d
docker compose ps
```

Ouvrez l'interface web à `http://localhost:3000`.

## Séparation des rôles

Le fichier Compose est sectionné par rôle :

- **Pile contrôleur** : `init-repo-controller`, `init-perms-controller`, `controller`.
- **Interface web** : `web`.
- **Initialisation partagée** : `init-config-perms`.
- **Pile agent** : `init-perms-agent`, `agent`.

Pour tout déploiement au-delà du tout-en-un, séparez explicitement ces sections pour votre topologie. Le contrôleur et le web peuvent s'exécuter ensemble ou séparément. Chaque nœud agent conserve la pile agent et son propre accès au socket Docker.

## Images

Les images de release sont publiées sur Forgejo, GHCR et Docker Hub :

| Composant | Forgejo | GHCR | Docker Hub |
|-----------|---------|------|------------|
| CLI | `forgejo.alexma.top/alexma233/composia-cli` | `ghcr.io/alexma233/composia-cli` | `alexma233/composia-cli` |
| Contrôleur | `forgejo.alexma.top/alexma233/composia-controller` | `ghcr.io/alexma233/composia-controller` | `alexma233/composia-controller` |
| Agent | `forgejo.alexma.top/alexma233/composia-agent` | `ghcr.io/alexma233/composia-agent` | `alexma233/composia-agent` |
| Web | `forgejo.alexma.top/alexma233/composia-web` | `ghcr.io/alexma233/composia-web` | `alexma233/composia-web` |

Les images canary sont publiées uniquement sur Forgejo et GHCR.

## Vérifications courantes

- Le contrôleur ne peut pas démarrer : vérifiez que `config.yaml` existe dans `COMPOSIA_CONFIG_DIR` et que les chemins requis du contrôleur existent ou peuvent être créés.
- L'agent ne peut pas utiliser Docker : vérifiez que `DOCKER_SOCK_GID` correspond à `/var/run/docker.sock` sur l'hôte.
- Le web ne peut pas atteindre le contrôleur : `WEB_CONTROLLER_ADDR` est pour le conteneur du serveur web, tandis que `WEB_BROWSER_CONTROLLER_ADDR` est pour le navigateur.
