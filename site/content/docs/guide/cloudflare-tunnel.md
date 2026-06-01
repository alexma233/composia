---
title: "Cloudflare Tunnel"
date: '2026-05-31T00:00:00+08:00'
weight: 25
---

Composia can manage remotely configured Cloudflare Tunnel ingress rules for services that declare `network.cloudflare_tunnel`. Tunnel sync runs as a controller-side task because Cloudflare's remote tunnel configuration is global state.

## How it works

When a service is deployed, updated, stopped, or manually synced, the controller creates a `cloudflare_tunnel_sync` task. The controller worker executes it:

1. Read all service metadata at the task repo revision.
2. Build tunnel ingress rules from services that declare `network.cloudflare_tunnel`.
3. Send the full ingress list to Cloudflare with `PUT /accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations`.
4. Ensure each hostname has a proxied CNAME pointing to `{tunnel_id}.cfargotunnel.com`.

Cloudflare requires a catch-all ingress rule. Composia appends `http_status:404` by default.

## Controller configuration

Tunnel IDs and Cloudflare credentials belong in the controller config, not in service metadata:

```yaml
controller:
  cloudflare_tunnel:
    account_id: "REPLACE"
    api_token_file: /run/secrets/cloudflare-api-token
    tunnels:
      edge:
        tunnel_id: "c1744f8b-faa1-48a4-9e5c-02ac921467fa"
        fallback_service: http_status:404
    nodes:
      main:
        tunnel: edge
```

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `account_id` | `string` | Yes | Cloudflare account ID. |
| `api_token` / `api_token_file` | `string` | Yes | API token with Cloudflare Tunnel config and DNS write permissions. |
| `tunnels` | `map` | Yes | Tunnel aliases mapped to Cloudflare tunnel IDs. |
| `nodes` | `map` | No | Default node-to-tunnel mapping used when a service does not specify `tunnel`. |

The `tunnel_id` is not the connector secret, but it is still controller-level infrastructure metadata. Cloudflared connector tokens or credentials should stay in node/agent secrets used by the `cloudflared` service.

## Service declaration

Declare tunnel ingress in the service's `composia-meta.yaml`:

```yaml
network:
  cloudflare_tunnel:
    hostname: app.example.com
    service: http://app:8080
    origin_request:
      no_tls_verify: false
      http_host_header: app.internal
```

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `hostname` | `string` | Yes | Public hostname routed by Cloudflare Tunnel. |
| `service` | `string` | Yes | Origin URL used by `cloudflared`, for example `http://app:8080`. |
| `tunnel` | `string` | No | Tunnel alias. When omitted, Composia derives it from the target node mapping. |
| `path` | `string` | No | Optional path matcher for the ingress rule. |
| `origin_request` | `object` | No | Cloudflare origin parameters. Initial support includes `no_tls_verify`, `http_host_header`, `origin_server_name`, `connect_timeout`, and `tls_timeout`. |

## Tunnel selection

Composia resolves the tunnel for each service with these rules:

1. If `network.cloudflare_tunnel.tunnel` is set, that alias is used.
2. If the service targets one node, Composia uses `controller.cloudflare_tunnel.nodes.<node>.tunnel`.
3. If the service targets multiple nodes and all nodes map to the same tunnel, that tunnel is used.
4. If target nodes map to different tunnels, the service must set `network.cloudflare_tunnel.tunnel` explicitly.

## Stop behavior

When a stopped service declared `network.cloudflare_tunnel`, the follow-up tunnel sync excludes that service and deletes its CNAME. Later syncs only include services with running instances, so deploying the service again adds it back.

## Manual sync

Use the CLI to sync a service's tunnel configuration:

```bash
composia service my-app tunnel-sync
```

This syncs the full configured tunnel state, not only the selected service, because Cloudflare's remote tunnel configuration is updated as one document.
