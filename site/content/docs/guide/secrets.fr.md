---
title: "Secrets"
date: '2026-05-26T00:00:00+08:00'
weight: 50
---

Composia gère les fichiers de secrets chiffrés dans le dépôt d'état désiré en utilisant le chiffrement age. Le chiffrement et le déchiffrement ont lieu sur le contrôleur. Les agents n'accèdent jamais à la clé privée age.

## Configuration

Les secrets nécessitent une paire de clés age. Configurez-la dans la configuration du contrôleur :

```yaml
controller:
  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
```

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `provider` | `string` | Oui | Doit être `age`. |
| `identity_file` | `string` | Oui | Chemin vers le fichier de clé privée age. |
| `recipient_file` | `string` | Non | Chemin vers le fichier contenant les destinataires age (clés publiques). Si omis, le destinataire est dérivé de la clé privée. |
| `armor` | `bool` | Non | Utiliser une sortie en armure ASCII. Par défaut `true`. |

Générez une paire de clés :

```bash
age-keygen -o age-identity.key
```

Optionnel : extrayez la clé publique comme destinataire :

```bash
age-keygen -y age-identity.key > age-recipients.txt
```

## Comment les secrets sont stockés

Les fichiers de secrets dans le dépôt ont une extension `.enc` par convention. Ils sont stockés sous forme de texte chiffré age :

```
my-app/
├── docker-compose.yaml
├── composia-meta.yaml
└── .secret.env.enc        (chiffré avec age)
```

Le contrôleur chiffre le texte en clair à l'écriture et déchiffre à la lecture. Le dépôt contient uniquement du texte chiffré. Les secrets n'apparaissent jamais en clair dans le dépôt, dans les journaux de tâches ou en transit vers les agents.

## Comment les secrets parviennent aux agents

Pendant l'étape de rendu d'une tâche de déploiement ou de mise à jour, le contrôleur :

1. Lit les fichiers chiffrés depuis le répertoire du service dans le dépôt.
2. Déchiffre chaque fichier en utilisant la clé privée age.
3. Supprime le suffixe `.enc` et injecte le contenu déchiffré sous son nom d'exécution. Par exemple, `.secret.env.enc` devient `.secret.env`.

Le bundle est transmis à l'agent via la connexion de rapport de l'agent. L'agent écrit le bundle sur disque et procède à `docker compose up`. L'environnement de secret déchiffré est disponible pour les services Compose sans que l'agent ne voie jamais la clé privée.

Les références de fichiers dans `composia-meta.yaml` utilisent les noms du dépôt, comme `.env.enc` ; Composia les convertit en `.env` avant l'utilisation à l'exécution. Les fichiers Compose, Caddyfiles et scripts utilisent directement le nom d'exécution `.env`.

## Utilisation de la CLI

Écrire un fichier de secret chiffré :

```bash
composia service my-app edit .secret.env.enc
```

Lire et déchiffrer un fichier de secret :

```bash
composia repo get my-app/.secret.env.enc
```

Modifier un secret sur place (ouvre votre éditeur) :

```bash
composia repo update --file ./local-plain.env my-app/.secret.env.enc
```

Toutes les opérations d'écriture de secrets incluent une vérification de révision de base pour éviter les conflits avec des modifications concurrentes.

## Règles de chemin de fichier

Les chemins de fichiers de secrets doivent :

- Être relatifs au répertoire du service (pas absolus).
- Ne pas contenir de séquences de traversée de chemin comme `../`.
- Pointer vers un fichier à l'intérieur du répertoire du service.

Le contrôleur localise le service, résout le chemin du fichier relativement au répertoire du service et opère sur le fichier du dépôt.

## Conditions d'erreur

- **Secrets non configurés** : les accès Repo aux fichiers `.enc` retournent `FailedPrecondition`.
- **Fichier introuvable** : `GetRepoFile` retourne `NotFound`.
- **Conflit de révision de base** : `UpdateRepoFile` protège HEAD avec CAS.
