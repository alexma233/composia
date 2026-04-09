# API Reference

Composia defines its RPC surface in `proto/` and generates reference pages directly from protobuf comments.

## Protocol

- Transport: `ConnectRPC`
- Default body encoding: `application/json`
- Auth: `Authorization: Bearer <token>` for controller-facing APIs
- Required Connect header for JSON calls: `Connect-Protocol-Version: 1`

## References

- [Controller API Reference](./controller-reference)
- [Agent Internal API Reference](./agent-internal-reference)

## Regenerate

Run the generator from the repository root:

```bash
bun run docs:api:generate
```

The generated Markdown files are committed under `docs/content/en-us/guide/api/` so the VitePress site can publish them directly.
