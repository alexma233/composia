# Configuration Verification

For local source-based development, validate configuration with the development examples:

```bash
# Start the Controller with the dev config
go run ./cmd/composia controller -config ./dev/config.controller.yaml

# Start the main Agent with the shared dev config
go run ./cmd/composia agent -config ./dev/config.controller.yaml
```

Keep local development config under `./dev/` and do not commit local tokens or key files.
