# DNS Configuration

This page documents the `controller.dns` configuration.

## Example

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

You can also inline the token:

```yaml
controller:
  dns:
    cloudflare:
      api_token: "cloudflare-token"
```

Supported providers:
- `cloudflare`
- `alidns`
- `dnspod`
- `route53`
- `huaweicloud`

Providers other than `cloudflare` require `zones`. `zones` maps service `network.dns.hostname` values to the actual DNS zone.

`route53` can also use AWS environment variables, shared configuration, or instance roles. `zones` is still required when credentials are not inline.

This page covers only the platform-side configuration.

For service-side DNS rules, auto-derived values, and record management behavior, see [DNS Configuration](../dns).
