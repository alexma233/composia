# DNS Configuration

This page documents the `controller.dns` configuration.

## Example

```yaml
controller:
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"
```

This page covers only the platform-side configuration.

For service-side DNS rules, auto-derived values, and record management behavior, see [DNS Configuration](../dns).
