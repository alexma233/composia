# Networking

This document explains how to configure DNS and Caddy reverse proxy in Composia.

## DNS Configuration

Composia supports automatic DNS record management. Currently only Cloudflare is supported.

### Controller Configuration

```yaml
controller:
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"
```

Create the API Token file:

```bash
echo "your-cloudflare-api-token" > ./cloudflare-token.txt
```

**Cloudflare Token Permissions Required:**
- Zone:Read
- DNS:Edit

### Service DNS Configuration

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

### Automatic IP Derivation

If `value` is not specified, Composia attempts to automatically derive it from node configuration:

```yaml
controller:
  nodes:
    - id: "main"
      public_ipv4: "203.0.113.10"    # Used for A records
      public_ipv6: "2001:db8::1"      # Used for AAAA records
```

**Note:** Automatic derivation is only suitable for single-node services. For multi-node services, explicitly provide `value`.

### Trigger DNS Update

DNS updates are available in the following cases:
- Migrating a service to a new node
- Manually executing `dns_update`

Manual trigger uses the ConnectRPC method `composia.controller.v1.ServiceCommandService/RunServiceAction` with the `SERVICE_ACTION_DNS_UPDATE` action.

### DNS Configuration Examples

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

## Caddy Reverse Proxy

Composia supports automatic generation and synchronization of Caddy configuration fragments.

### Architecture

```
Service (composia-meta.yaml)
    │ network.caddy.enabled: true
    ▼
Controller (generates config fragment)
    │
    ▼
Agent (distributes to nodes)
    │ writes to generated_dir
    ▼
Caddy (loads config and reloads)
```

### 1. Deploy Caddy Infrastructure Service

Create a Caddy infrastructure service:

```yaml
# infra-caddy/composia-meta.yaml
name: infra-caddy
nodes:
  - main
enabled: true

infra:
  caddy:
    compose_service: caddy      # Compose service name
    config_dir: /etc/caddy      # Caddy configuration directory
```

```yaml
# infra-caddy/docker-compose.yaml
services:
  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config
      - /srv/caddy/generated:/etc/caddy/conf.d  # Generated config directory
    command: caddy run --config /etc/caddy/Caddyfile --adapter caddyfile

volumes:
  caddy_data:
  caddy_config:
```

```caddy
# infra-caddy/Caddyfile
# Import generated configurations
import /etc/caddy/conf.d/*.conf

# Optional: default response
:80 {
    respond "Caddy is running"
}
```

### 2. Configure Agent

```yaml
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-agent-token"
  caddy:
    generated_dir: "/srv/caddy/generated"  # Must match Caddy container mount path
```

### 3. Configure Business Service

Add configuration to services that need Caddy proxy:

```yaml
# my-app/composia-meta.yaml
name: my-app
nodes:
  - main

network:
  caddy:
    enabled: true
    source: ./Caddyfile.fragment
```

Create the Caddy configuration fragment:

```caddy
# my-app/Caddyfile.fragment
app.example.com {
    reverse_proxy localhost:8080
    
    # Security headers
    header {
        X-Frame-Options "SAMEORIGIN"
        X-Content-Type-Options "nosniff"
        X-XSS-Protection "1; mode=block"
        Referrer-Policy "strict-origin-when-cross-origin"
    }
    
    # Gzip compression
    encode gzip
    
    # Logging
    log {
        output file /var/log/caddy/app.log
        format json
    }
    
    # TLS (automatic Let's Encrypt)
    tls {
        protocols tls1.2 tls1.3
    }
}
```

### 4. Automated Behavior

Caddy configuration is automatically synchronized in the following cases:

| Operation | Automated Behavior |
|-----------|-------------------|
| `deploy` | Triggers `caddy_sync` + `caddy_reload` after success |
| `update` | Triggers `caddy_sync` + `caddy_reload` after success |
| `stop` | Removes generated fragment and triggers `caddy_reload` |
| `migrate` | Removes config from source node, adds to target node |

### Caddy Configuration Fragment Templates

**Basic Reverse Proxy:**

```caddy
app.example.com {
    reverse_proxy localhost:3000
}
```

**With Load Balancing (Multiple Instances):**

```caddy
app.example.com {
    reverse_proxy localhost:3000 localhost:3001 localhost:3002 {
        lb_policy round_robin
        health_uri /health
        health_interval 10s
    }
}
```

**With Basic Authentication:**

```caddy
app.example.com {
    basicauth {
        admin $2a$14$Zkx19XLiW6VYouLHR5NmfOFU0z2GTNmpkT/5qqR7hx4IjWJPDhjvG
    }
    reverse_proxy localhost:3000
}
```

**WebSocket Support:**

```caddy
app.example.com {
    reverse_proxy localhost:3000 {
        header_up Upgrade {>Upgrade}
        header_up Connection {>Connection}
    }
}
```

**Rate Limiting:**

```caddy
app.example.com {
    rate_limit {
        zone static_example {
            key static
            events 100
            window 1m
        }
    }
    reverse_proxy localhost:3000
}
```

## Complete Example

### Deploy a Complete Web Application

**Directory Structure:**

```
my-webapp/
├── composia-meta.yaml
├── docker-compose.yaml
└── Caddyfile.fragment
```

**composia-meta.yaml:**

```yaml
name: my-webapp
nodes:
  - main

network:
  caddy:
    enabled: true
    source: ./Caddyfile.fragment
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    proxied: true

data_protect:
  data:
    - name: uploads
      backup:
        strategy: files.copy
        include:
          - ./data/uploads
      restore:
        strategy: files.copy
        include:
          - ./data/uploads

backup:
  data:
    - name: uploads
      provider: rustic
```

**docker-compose.yaml:**

```yaml
services:
  app:
    image: myapp:1.0.0
    ports:
      - "127.0.0.1:8080:8080"  # Local only, exposed via Caddy
    volumes:
      - ./data/uploads:/app/uploads
    environment:
      - NODE_ENV=production
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

**Caddyfile.fragment:**

```caddy
app.example.com {
    reverse_proxy localhost:8080
    
    header {
        X-Frame-Options "SAMEORIGIN"
        X-Content-Type-Options "nosniff"
    }
    
    encode gzip
    
    log {
        output file /var/log/caddy/my-webapp.log
    }
}
```

**Deployment Steps:**

1. Ensure Caddy infrastructure service is deployed
2. Commit `my-webapp` directory to Git repository
3. Find `my-webapp` service in Web UI
4. Click **Deploy**
5. If needed, run `dns_update` after deployment; Caddy file sync happens through the corresponding node maintenance steps
6. Visit `https://app.example.com`

## Troubleshooting

### DNS Not Updated

Check:
1. Is Controller configured with `dns.cloudflare`?
2. Is Cloudflare API Token valid?
3. Is domain Zone correct?

### Caddy Configuration Not Applied

Check:
1. Is Caddy infrastructure service running?
2. Is Agent's `caddy.generated_dir` correct?
3. Is Caddy container correctly mounting the generated directory?
4. Inspect the Caddy container logs using your own container runtime tooling

### HTTPS Certificate Issues

- Ensure certificate directory is persisted (`caddy_data` volume)
- Check if domain DNS correctly points to server
- View Caddy logs for certificate request status

## Related Documentation

- [Service Definition](./service-definition) — Complete service configuration reference
- [Deployment](./deployment) — Service deployment flow
- [Caddy Official Documentation](https://caddyserver.com/docs/) — Caddy configuration reference
