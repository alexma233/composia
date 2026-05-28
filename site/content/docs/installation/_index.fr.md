---
title: "Installation"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Composia a quatre binaires et images d'exécution :

| Composant | Rôle |
|-----------|---------|
| `composia-controller` | Exécute l'API, la file de tâches, le dépôt Git d'état désiré et les intégrations côté contrôleur. |
| `composia-agent` | S'exécute sur chaque nœud Docker et exécute les opérations Docker Compose. |
| `composia-web` | Interface navigateur qui communique avec le contrôleur. |
| `composia` | CLI pour les terminaux, les scripts et l'automatisation. |

## Choisir une méthode

| Méthode | Idéale pour |
|--------|----------|
| [Docker Compose](docker-compose/) | Déploiement tout-en-un rapide avec contrôleur, agent local et interface web. |
| [Gestionnaires de paquets et binaires](package-managers/) | Installations sans conteneur, paquets OS, Nix, AUR et archives manuelles. |
| [Configuration](configuration/) | Fichiers de configuration, variables d'environnement web, configuration des clés age et référence complète de la configuration globale. |

Pour les compilations depuis les sources, voir [Guide développeur : Build depuis les sources](/docs/developer-guide/source-build/).
