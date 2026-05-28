---
title: "DNS"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Composia gère les enregistrements DNS pour les services qui déclarent `network.dns`. Les mises à jour DNS s'exécutent comme des tâches côté contrôleur.

## Fonctionnement

Lorsqu'un service est déployé ou qu'une mise à jour DNS est déclenchée manuellement, le contrôleur crée une tâche `dns_update`. Le worker du contrôleur l'exécute :

1. Lire les métadonnées du service à la révision du dépôt enregistrée dans la tâche.
2. Construire les enregistrements DNS souhaités à partir de `network.dns`.
3. Synchroniser les enregistrements avec le fournisseur DNS.

## Configuration du fournisseur

Configurez au moins un fournisseur DNS dans la configuration du contrôleur. Les identifiants du fournisseur et la liste des zones sont globaux :

```yaml
controller:
  dns:
    cloudflare:
      api_token: "REPLACE"
      zones:
        - "example.com"
        - "other.com"
```

Cinq fournisseurs sont pris en charge. Chacun a ses propres clés d'identifiants et partage un champ `zones` listant les zones de domaine gérées :

| Fournisseur | Préfixe de clé | Clés d'identifiants |
|----------|-----------|-----------------|
| `cloudflare` | `dns.cloudflare` | `api_token`, `api_token_file` |
| `alidns` | `dns.alidns` | `access_key_id`, `access_key_secret`, `region_id`, optionnel `security_token` |
| `dnspod` | `dns.dnspod` | `secret_id`, `secret_key`, `region`, optionnel `session_token` |
| `route53` | `dns.route53` | `access_key_id`, `secret_access_key`, `region`, optionnel `session_token`, `profile`, `hosted_zone_id` |
| `huaweicloud` | `dns.huaweicloud` | `access_key_id`, `secret_access_key`, `region_id` |

Chaque champ d'identifiant a une variante `_file` correspondante pour la lecture depuis un fichier (par exemple `api_token_file`).

## Déclaration DNS du service

Déclarez les paramètres DNS dans le fichier `composia-meta.yaml` du service :

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    value: 203.0.113.10
    proxied: true
    ttl: 120
    comment: "Managed by Composia"
```

| Clé | Type | Requis | Description |
|-----|------|----------|-------------|
| `provider` | `string` | Oui | `cloudflare`, `alidns`, `dnspod`, `route53` ou `huaweicloud`. |
| `hostname` | `string` | Oui | Nom d'hôte DNS. La zone est mise en correspondance avec la liste des zones configurées. |
| `record_type` | `string` | Non | `A`, `AAAA` ou `CNAME`. Si vide, le type d'enregistrement est déduit de la valeur ou des adresses du nœud. |
| `value` | `string` | Non | Valeur explicite de l'enregistrement DNS. Si vide, Composia déduit la valeur du nœud cible. |
| `proxied` | `bool` | Non | Activer le proxy Cloudflare. Uniquement pris en charge par Cloudflare. |
| `ttl` | `uint32` | Non | TTL DNS en secondes. |
| `comment` | `string` | Non | Commentaire de l'enregistrement DNS. Uniquement pris en charge par Cloudflare. |

## Résolution des enregistrements

### Avec une valeur explicite

Lorsque `value` est défini, Composia l'utilise directement. S'il s'agit d'une adresse IP, le type d'enregistrement est déduit : IPv4 devient `A`, IPv6 devient `AAAA`. S'il s'agit d'un nom d'hôte, le type d'enregistrement doit être `CNAME` (ou vide, ce qui résout également en `CNAME`).

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    value: 203.0.113.10
```

### À partir des adresses du nœud

Lorsque `value` est vide, Composia utilise les adresses `public_ipv4` et `public_ipv6` du nœud cible depuis la configuration du contrôleur :

```yaml
controller:
  nodes:
    - id: "main"
      public_ipv4: "203.0.113.10"
      public_ipv6: "2001:db8::10"
```

Avec un `record_type` vide, les enregistrements A et AAAA sont créés lorsque le nœud a les deux adresses. Si `record_type` est `A`, seule l'adresse IPv4 est utilisée. Si `record_type` est `AAAA`, seule l'adresse IPv6 est utilisée.

Les services qui ciblent plus d'un nœud doivent définir `value` explicitement. Un `value` vide avec plusieurs nœuds cibles produit une erreur.

## Déclencher les mises à jour DNS

Les enregistrements DNS sont créés ou mis à jour pendant le flux de tâche de déploiement. Vous pouvez également déclencher une mise à jour DNS autonome via l'interface web ou la CLI :

```bash
composia service dns-update my-app
```

Cela crée une tâche `dns_update`. Le journal de la tâche montre la résolution de zone, les opérations d'enregistrement et le résultat final.

## Options Cloudflare

Lorsque le fournisseur est `cloudflare`, `proxied` et `comment` sont appliqués après la création de l'enregistrement. Composia appelle l'API Cloudflare pour modifier chaque enregistrement DNS avec le statut de proxy et le commentaire demandés.

Les fournisseurs autres que Cloudflare ne prennent pas en charge ces options. Définir `proxied` ou `comment` avec un autre fournisseur fait échouer la mise à jour DNS.

## Mise en correspondance des zones

Composia met en correspondance le nom d'hôte du service avec les zones configurées. Les zones sont essayées de la correspondance la plus longue à la plus courte. Par exemple, avec `zones: ["example.com.", "sub.example.com."]`, le nom d'hôte `app.sub.example.com` correspond d'abord à `sub.example.com.`.

Si aucune zone ne correspond au nom d'hôte, la mise à jour DNS échoue.

## Nettoyage des enregistrements obsolètes

La synchronisation DNS gère exactement trois types d'enregistrements par nom d'hôte : A, AAAA et CNAME. Tout type d'enregistrement configuré qui n'est pas présent dans l'état souhaité est supprimé avant que les nouveaux enregistrements ne soient définis. Par exemple, si un service avait précédemment `record_type: A` et passe à `record_type: CNAME`, l'ancien enregistrement A est supprimé et un nouvel enregistrement CNAME est créé.

Changer le nom d'hôte d'un service ne nettoie pas les enregistrements de l'ancien nom d'hôte. Si vous renommez `app.example.com` en `api.example.com`, les enregistrements pour `app.example.com` restent chez le fournisseur DNS jusqu'à ce que vous les supprimiez manuellement.
