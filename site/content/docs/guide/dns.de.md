---
title: "DNS"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Composia verwaltet DNS-Records für Dienste, die `network.dns` deklarieren. DNS-Updates laufen als controller-seitige Aufgaben.

## Wie es funktioniert

Wenn ein Dienst deployed wird oder ein DNS-Update manuell ausgelöst wird, erstellt der Controller eine `dns_update`-Aufgabe. Der Controller-Worker führt sie aus:

1. Liest die Dienst-Meta beim in der Aufgabe aufgezeichneten Repo-Revision.
2. Erstellt die gewünschten DNS-Records aus `network.dns`.
3. Synchronisiert die Records mit dem DNS-Provider.

## Provider-Konfiguration

Konfiguriere mindestens einen DNS-Provider in der Controller-Konfiguration. Die Provider-Anmeldeinformationen und Zonenliste sind global:

```yaml
controller:
  dns:
    cloudflare:
      api_token: "REPLACE"
      zones:
        - "example.com"
        - "other.com"
```

Fünf Provider werden unterstützt. Jeder hat seine eigenen Anmeldeschlüssel und alle teilen ein `zones`-Feld, das die verwalteten Domain-Zonen auflistet:

| Provider | Schlüsselpräfix | Anmeldeschlüssel |
|----------|-----------|-----------------|
| `cloudflare` | `dns.cloudflare` | `api_token`, `api_token_file` |
| `alidns` | `dns.alidns` | `access_key_id`, `access_key_secret`, `region_id`, optional `security_token` |
| `dnspod` | `dns.dnspod` | `secret_id`, `secret_key`, `region`, optional `session_token` |
| `route53` | `dns.route53` | `access_key_id`, `secret_access_key`, `region`, optional `session_token`, `profile`, `hosted_zone_id` |
| `huaweicloud` | `dns.huaweicloud` | `access_key_id`, `secret_access_key`, `region_id` |

Jedes Anmeldefeld hat eine entsprechende `_file`-Variante zum Lesen aus einer Datei (zum Beispiel `api_token_file`).

## Dienst-DNS-Deklaration

Deklariere DNS-Einstellungen in der `composia-meta.yaml` des Dienstes:

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    value: 203.0.113.10
    proxied: true
    ttl: 120
    comment: "Verwaltet von Composia"
```

| Schlüssel | Typ | Erforderlich | Beschreibung |
|-----|------|----------|-------------|
| `provider` | `string` | Ja | `cloudflare`, `alidns`, `dnspod`, `route53` oder `huaweicloud`. |
| `hostname` | `string` | Ja | DNS-Hostname. Die Zone wird aus der konfigurierten Zonenliste abgeglichen. |
| `record_type` | `string` | Nein | `A`, `AAAA` oder `CNAME`. Wenn leer, wird der Record-Typ aus dem Wert oder den Node-Adressen abgeleitet. |
| `value` | `string` | Nein | Expliziter DNS-Record-Wert. Wenn leer, leitet Composia den Wert vom Ziel-Node ab. |
| `proxied` | `bool` | Nein | Aktiviert den Cloudflare-Proxy. Wird nur von Cloudflare unterstützt. |
| `ttl` | `uint32` | Nein | DNS-TTL in Sekunden. |
| `comment` | `string` | Nein | DNS-Record-Kommentar. Wird nur von Cloudflare unterstützt. |

## Record-Auflösung

### Mit einem expliziten Wert

Wenn `value` gesetzt ist, verwendet Composia ihn direkt. Wenn es eine IP-Adresse ist, wird der Record-Typ abgeleitet: IPv4 wird zu `A`, IPv6 wird zu `AAAA`. Wenn es ein Hostname ist, muss der Record-Typ `CNAME` sein (oder leer, was ebenfalls zu `CNAME` aufgelöst wird).

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    value: 203.0.113.10
```

### Von Node-Adressen

Wenn `value` leer ist, verwendet Composia die `public_ipv4` und `public_ipv6` des Ziel-Nodes aus der Controller-Konfiguration:

```yaml
controller:
  nodes:
    - id: "main"
      public_ipv4: "203.0.113.10"
      public_ipv6: "2001:db8::10"
```

Bei leerem `record_type` werden sowohl A- als auch AAAA-Records erstellt, wenn der Node beide Adressen hat. Wenn `record_type` `A` ist, wird nur die IPv4-Adresse verwendet. Wenn `record_type` `AAAA` ist, wird nur die IPv6-Adresse verwendet.

Dienste, die mehr als einen Node anvisieren, müssen `value` explizit setzen. Ein leeres `value` mit mehreren Ziel-Nodes erzeugt einen Fehler.

## DNS-Updates auslösen

DNS-Records werden während des Deploy-Aufgabenablaufs erstellt oder aktualisiert. Du kannst auch ein eigenständiges DNS-Update über die Web-UI oder CLI auslösen:

```bash
composia service dns-update my-app
```

Dies erstellt eine `dns_update`-Aufgabe. Das Aufgabenprotokoll zeigt die Zonenauflösung, Record-Operationen und das Endergebnis.

## Cloudflare-Optionen

Wenn der Provider `cloudflare` ist, werden `proxied` und `comment` nach der Record-Erstellung angewendet. Composia ruft die Cloudflare-API auf, um jeden DNS-Record mit dem angeforderten Proxy-Status und Kommentar zu patchen.

Nicht-Cloudflare-Provider unterstützen diese Optionen nicht. Das Setzen von `proxied` oder `comment` mit einem anderen Provider führt zum Fehlschlag des DNS-Updates.

## Zonenabgleich

Composia gleicht den Dienst-Hostnamen mit den konfigurierten Zonen ab. Zonen werden von der längsten zur kürzesten Übereinstimmung durchprobiert. Zum Beispiel, mit `zones: ["example.com.", "sub.example.com."]`, passt der Hostname `app.sub.example.com` zuerst zu `sub.example.com.`.

Wenn keine Zone zum Hostnamen passt, schlägt das DNS-Update fehl.

## Bereinigung veralteter Records

Die DNS-Synchronisation verwaltet genau drei Record-Typen pro Hostname: A, AAAA und CNAME. Jeder konfigurierte Record-Typ, der nicht im Sollzustand vorhanden ist, wird gelöscht, bevor neue Records gesetzt werden. Wenn zum Beispiel ein Dienst zuvor `record_type: A` hatte und zu `record_type: CNAME` wechselt, wird der alte A-Record entfernt und ein neuer CNAME-Record erstellt.

Das Ändern des Hostnamens eines Dienstes bereinigt keine Records für den alten Hostnamen. Wenn du `app.example.com` in `api.example.com` umbenennst, bleiben die Records für `app.example.com` im DNS-Provider, bis du sie manuell entfernst.
