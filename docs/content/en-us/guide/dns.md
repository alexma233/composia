# DNS Configuration

This page explains how to configure service-side DNS in Composia.

## Controller Configuration

```yaml
controller:
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"
    alidns:
      access_key_id_file: "/app/configs/alidns-access-key-id.txt"
      access_key_secret_file: "/app/configs/alidns-access-key-secret.txt"
      zones: ["example.com"]
    dnspod:
      secret_id_file: "/app/configs/dnspod-secret-id.txt"
      secret_key_file: "/app/configs/dnspod-secret-key.txt"
      zones: ["example.com"]
    route53:
      access_key_id_file: "/app/configs/aws-access-key-id.txt"
      secret_access_key_file: "/app/configs/aws-secret-access-key.txt"
      region: "us-east-1"
      zones: ["example.com"]
    huaweicloud:
      access_key_id_file: "/app/configs/huaweicloud-access-key-id.txt"
      secret_access_key_file: "/app/configs/huaweicloud-secret-access-key.txt"
      region_id: "cn-south-1"
      zones: ["example.com"]
```

Create the API Token file:

```bash
echo "your-cloudflare-api-token" > ./cloudflare-token.txt
```

**Cloudflare Token Permissions Required:**
- Zone:Read
- DNS:Edit

Supported providers: `cloudflare`, `alidns`, `dnspod`, `route53`, and `huaweicloud`.

Providers other than Cloudflare require `zones` in the controller configuration so service hostnames can be matched to DNS zones.

For platform-side field details, see [DNS Configuration in the configuration guide](./configuration/dns).

## Service DNS Configuration

Configure in the service's `composia-meta.yaml`:

```yaml
name: my-app
nodes:
  - main

network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A        # A, AAAA, or CNAME
    proxied: true         # Enable Cloudflare proxy
    ttl: 120              # TTL in seconds
    # value: "1.2.3.4"    # Optional, manually specify record value
```

`provider` can also be `alidns`, `dnspod`, `route53`, or `huaweicloud`. `proxied` and `comment` are Cloudflare-only; using them with other providers fails the DNS update.

## Automatic IP Derivation

If `value` is not specified, Composia attempts to automatically derive it from node configuration:

```yaml
controller:
  nodes:
    - id: "main"
      public_ipv4: "203.0.113.10"    # Used for A records
      public_ipv6: "2001:db8::1"      # Used for AAAA records
```

**Note:** Automatic derivation is only suitable for single-node services. For multi-node services, explicitly provide `value`.

## Trigger DNS Update

DNS updates are available in the following cases:
- Migrating a service to a new node
- Manually executing `dns_update`

Manual trigger uses the ConnectRPC method `composia.controller.v1.ServiceCommandService/RunServiceAction` with the `SERVICE_ACTION_DNS_UPDATE` action.

The direct HTTP path is `/api/controller/composia.controller.v1.ServiceCommandService/RunServiceAction`.

## DNS Configuration Examples

**Basic A Record:**

```yaml
network:
  dns:
    provider: cloudflare
    hostname: api.example.com
    record_type: A
```

**Enable Cloudflare Proxy:**

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    proxied: true
    ttl: 1    # TTL automatically managed in automatic mode
```

**IPv6 Support:**

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: AAAA
```

**Multiple Domains:**

Configure separate services for each domain or use wildcards.

## Troubleshooting

### DNS Not Updated

Check:
1. Is Controller configured with the matching provider, such as `dns.cloudflare` or `dns.route53`?
2. Are the provider credentials valid?
3. For non-Cloudflare providers, does `zones` include the hostname's DNS zone?

## Related Documentation

- [Service Definition](./service-definition) — Complete service configuration reference
- [Deployment](./deployment) — Service deployment flow
- [Caddy Configuration](./caddy) — Caddy reverse proxy configuration
