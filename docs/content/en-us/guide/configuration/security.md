# Configuration Security

This page collects the baseline recommendations for config files, tokens, and key files.

## Token Handling

- Use strong random strings for Controller and node tokens
- Do not reuse development tokens in production
- Rotate tokens regularly

## Config Mounts

Use read-only mounts for the config directory:

```yaml
# docker-compose.yaml
volumes:
  - ./config:/app/configs:ro
```

## Key File Handling

- Do not commit local tokens or key files to the repository
- Mount the age private key and public key separately to `identity_file` and `recipient_file`
- Only mount key files into the containers that need them
