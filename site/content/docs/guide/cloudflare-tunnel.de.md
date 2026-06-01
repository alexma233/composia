---
title: "Cloudflare Tunnel"
date: '2026-05-31T00:00:00+08:00'
weight: 25
---

Composia kann remote konfigurierte Cloudflare-Tunnel-Ingress-Regeln für Dienste verwalten, die `network.cloudflare_tunnel` deklarieren. Die Tunnel-Synchronisierung wird als Controller-seitige Aufgabe ausgeführt, da die Cloudflare-Remote-Tunnelkonfiguration ein globaler Zustand ist.

## Funktionsweise

Wenn ein Dienst bereitgestellt, aktualisiert, gestoppt oder manuell synchronisiert wird, erstellt der Controller eine `cloudflare_tunnel_sync`-Aufgabe. Der Controller-Worker führt sie aus:

1. Liest alle Dienstmetadaten in der Aufgaben-Repo-Revision.
2. Erstellt Tunnel-Ingress-Regeln aus Diensten, die `network.cloudflare_tunnel` deklarieren.
3. Sendet die vollständige Ingress-Liste an Cloudflare mit `PUT /accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations`.
4. Stellt sicher, dass jeder Hostname einen Proxied-CNAME hat, der auf `{tunnel_id}.cfargotunnel.com` zeigt.

Cloudflare erfordert eine Catch-all-Ingress-Regel. Composia fügt standardmäßig `http_status:404` an.

## Controller-Konfiguration

Tunnel-IDs und Cloudflare-Zugangsdaten gehören in die Controller-Konfiguration, nicht in die Dienstmetadaten:

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

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `account_id` | `string` | Ja | Cloudflare-Konto-ID. |
| `api_token` / `api_token_file` | `string` | Ja | API-Token mit Cloudflare-Tunnel-Konfigurations- und DNS-Schreibberechtigungen. |
| `tunnels` | `map` | Ja | Tunnel-Aliase, die Cloudflare-Tunnel-IDs zugeordnet sind. |
| `nodes` | `map` | Nein | Standard-Zuordnung von Knoten zu Tunneln, die verwendet wird, wenn ein Dienst keinen `tunnel` angibt. |

Die `tunnel_id` ist nicht das Connector-Geheimnis, aber dennoch Controller-level Infrastruktur-Metadaten. Cloudflared-Connector-Tokens oder -Zugangsdaten sollten in Knoten-/Agent-Geheimnissen verbleiben, die vom `cloudflared`-Dienst verwendet werden.

## Dienstdeklaration

Deklarieren Sie den Tunnel-Ingress in der `composia-meta.yaml` des Dienstes:

```yaml
network:
  cloudflare_tunnel:
    hostname: app.example.com
    service: http://app:8080
    origin_request:
      no_tls_verify: false
      http_host_header: app.internal
```

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `hostname` | `string` | Ja | Öffentlicher Hostname, der vom Cloudflare-Tunnel geroutet wird. |
| `service` | `string` | Ja | Origin-URL, die von `cloudflared` verwendet wird, z.B. `http://app:8080`. |
| `tunnel` | `string` | Nein | Tunnel-Alias. Wenn nicht angegeben, leitet Composia ihn aus der Zielknotenzuordnung ab. |
| `path` | `string` | Nein | Optionaler Pfad-Matcher für die Ingress-Regel. |
| `origin_request` | `object` | Nein | Cloudflare-Origin-Parameter. Erste Unterstützung umfasst `no_tls_verify`, `http_host_header`, `origin_server_name`, `connect_timeout` und `tls_timeout`. |

## Tunnelauswahl

Composia löst den Tunnel für jeden Dienst mit diesen Regeln auf:

1. Wenn `network.cloudflare_tunnel.tunnel` gesetzt ist, wird dieser Alias verwendet.
2. Wenn der Dienst auf einen Knoten abzielt, verwendet Composia `controller.cloudflare_tunnel.nodes.<node>.tunnel`.
3. Wenn der Dienst auf mehrere Knoten abzielt und alle Knoten auf denselben Tunnel verweisen, wird dieser Tunnel verwendet.
4. Wenn Zielknoten auf verschiedene Tunnel verweisen, muss der Dienst `network.cloudflare_tunnel.tunnel` explizit setzen.

## Stopp-Verhalten

Wenn ein gestoppter Dienst `network.cloudflare_tunnel` deklariert hat, schließt die nachfolgende Tunnel-Synchronisierung diesen Dienst aus und löscht seinen CNAME. Spätere Synchronisierungen umfassen nur Dienste mit laufenden Instanzen, sodass ein erneutes Bereitstellen des Dienstes ihn wieder hinzufügt.

## Manuelle Synchronisierung

Verwenden Sie die CLI, um die Tunnelkonfiguration eines Dienstes zu synchronisieren:

```bash
composia service my-app tunnel-sync
```

Dies synchronisiert den vollständigen konfigurierten Tunnelzustand, nicht nur den ausgewählten Dienst, da die Cloudflare-Remote-Tunnelkonfiguration als ein Dokument aktualisiert wird.
