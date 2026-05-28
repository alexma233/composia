---
title: "DNS"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Composia manages DNS records for services that declare `network.dns`. DNS updates run as controller-side tasks.

## How it works

When a service is deployed or a DNS update is triggered manually, the controller creates a `dns_update` task. The controller worker executes it:

1. Read the service meta at the repo revision recorded in the task.
2. Build desired DNS records from `network.dns`.
3. Sync records to the DNS provider.

## Provider configuration

Configure at least one DNS provider in the controller config. The provider credentials and zone list are global:

```yaml
controller:
  dns:
    cloudflare:
      api_token: "REPLACE"
      zones:
        - "example.com"
        - "other.com"
```

Five providers are supported. Each has its own credential keys and all share a `zones` field listing managed domain zones:

| Provider | Key prefix | Credential keys |
|----------|-----------|-----------------|
| `cloudflare` | `dns.cloudflare` | `api_token`, `api_token_file` |
| `alidns` | `dns.alidns` | `access_key_id`, `access_key_secret`, `region_id`, optional `security_token` |
| `dnspod` | `dns.dnspod` | `secret_id`, `secret_key`, `region`, optional `session_token` |
| `route53` | `dns.route53` | `access_key_id`, `secret_access_key`, `region`, optional `session_token`, `profile`, `hosted_zone_id` |
| `huaweicloud` | `dns.huaweicloud` | `access_key_id`, `secret_access_key`, `region_id` |

Each credential field has a corresponding `_file` variant for reading from a file (for example `api_token_file`).

## Service DNS declaration

Declare DNS settings in the service's `composia-meta.yaml`:

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

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `provider` | `string` | Yes | `cloudflare`, `alidns`, `dnspod`, `route53`, or `huaweicloud`. |
| `hostname` | `string` | Yes | DNS hostname. The zone is matched from the configured zone list. |
| `record_type` | `string` | No | `A`, `AAAA`, or `CNAME`. When empty, the record type is inferred from the value or node addresses. |
| `value` | `string` | No | Explicit DNS record value. When empty, Composia derives the value from the target node. |
| `proxied` | `bool` | No | Enable Cloudflare proxy. Only supported by Cloudflare. |
| `ttl` | `uint32` | No | DNS TTL in seconds. |
| `comment` | `string` | No | DNS record comment. Only supported by Cloudflare. |

## Record resolution

### With an explicit value

When `value` is set, Composia uses it directly. If it is an IP address, the record type is inferred: IPv4 becomes `A`, IPv6 becomes `AAAA`. If it is a hostname, the record type must be `CNAME` (or empty, which also resolves to `CNAME`).

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    value: 203.0.113.10
```

### From node addresses

When `value` is empty, Composia uses the target node's `public_ipv4` and `public_ipv6` from the controller config:

```yaml
controller:
  nodes:
    - id: "main"
      public_ipv4: "203.0.113.10"
      public_ipv6: "2001:db8::10"
```

With an empty `record_type`, both A and AAAA records are created when the node has both addresses. If `record_type` is `A`, only the IPv4 address is used. If `record_type` is `AAAA`, only the IPv6 address is used.

Services that target more than one node must set `value` explicitly. An empty `value` with multiple target nodes produces an error.

## Triggering DNS updates

DNS records are created or updated during the deploy task flow. You can also trigger a standalone DNS update through the web UI or CLI:

```bash
composia service dns-update my-app
```

This creates a `dns_update` task. The task log shows zone resolution, record operations, and the final result.

## Cloudflare options

When the provider is `cloudflare`, `proxied` and `comment` are applied after record creation. Composia calls the Cloudflare API to patch each DNS record with the requested proxy status and comment.

Non-Cloudflare providers do not support these options. Setting `proxied` or `comment` with another provider causes the DNS update to fail.

## Zone matching

Composia matches the service hostname against the configured zones. Zones are tried from longest to shortest match. For example, with `zones: ["example.com.", "sub.example.com."]`, hostname `app.sub.example.com` matches `sub.example.com.` first.

If no zone matches the hostname, the DNS update fails.

## Stale record cleanup

DNS sync manages exactly three record types per hostname: A, AAAA, and CNAME. Any configured record type that is not present in the desired state is deleted before new records are set. For example, if a service previously had `record_type: A` and changes to `record_type: CNAME`, the old A record is removed and a new CNAME record is created.

Changing a service's hostname does not clean up records for the old hostname. If you rename `app.example.com` to `api.example.com`, the records for `app.example.com` remain in the DNS provider until you remove them manually.
