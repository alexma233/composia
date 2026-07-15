---
title: "Docker Compose"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Der Docker-Compose-Stack führt den Controller, einen lokalen Agenten und die Web-UI aus der kanonischen [`docker-compose.yaml`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/docker-compose.yaml) aus.

## Dateien herunterladen

Du musst nicht das gesamte Repository für eine Docker-Compose-Installation klonen. Lade die Compose-Datei und die Umgebungsvorlage herunter:

```bash
curl -LO https://forgejo.alexma.top/alexma233/composia/raw/branch/main/docker-compose.yaml
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/.env.example -o .env
```

Bearbeite `.env` vor dem Start des Stacks. Die Vorlage ist nach Rolle gruppiert; für den All-in-One-Stack behalte alle Gruppen. Siehe [Konfiguration](../configuration/) für die Bedeutung jeder Variable.

Ermittle die Docker-Socket-Gruppen-ID auf dem Host:

```bash
stat -c '%g' /var/run/docker.sock
```

Setze `DOCKER_SOCK_GID` auf diesen Wert.

## Agent-Repository-Pfad

`COMPOSIA_AGENT_REPO_DIR` wird eingebunden als:

```yaml
- ${COMPOSIA_AGENT_REPO_DIR}:${COMPOSIA_AGENT_REPO_DIR}
```

Der Host-Pfad und der Container-Pfad müssen identisch sein. Der Agent ruft den Host-Docker-Daemon auf, und der Host-Docker-Daemon löst Bind-Mounts aus dem Host-Dateisystem auf. Wenn das Dienst-Repository unter einem anderen Pfad im Agent-Container eingebunden ist, kann Docker Compose Host-Pfade generieren, die nicht existieren.

Verwende denselben absoluten Pfad auf beiden Seiten, zum Beispiel:

```bash
COMPOSIA_AGENT_REPO_DIR=/data/repo-agent
```

Set `agent.repo_dir` in `config.yaml` to the same absolute path.

## Grundlegende `config.yaml`

Erstelle `config.yaml` innerhalb von `COMPOSIA_CONFIG_DIR`. Die Docker-Compose-Datei bindet dieses Verzeichnis unter `/app/configs` ein.

```yaml {filename="config.yaml"}
controller:
  listen_addr: ":7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  access_tokens:
    - name: "web"
      token: "REPLACE_WITH_WEB_ACCESS_TOKEN"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
```

Setze `WEB_CONTROLLER_ACCESS_TOKEN` in `.env` auf denselben Wert wie `controller.access_tokens[0].token`.

## Web-Passwort

`WEB_LOGIN_PASSWORD_HASH` must be an Argon2id PHC hash. Generate it from a hidden prompt so the plaintext password is not written to shell history:

```bash
read -r -s -p 'Web password: ' COMPOSIA_WEB_PASSWORD; echo
printf '%s' "$COMPOSIA_WEB_PASSWORD" | docker run --rm -i -e NODE_NO_WARNINGS=1 node:24-alpine node -e 'const {randomBytes}=require("node:crypto");let p="";process.stdin.setEncoding("utf8");process.stdin.on("data",c=>p+=c);process.stdin.on("end",async()=>{const salt=randomBytes(16);const key=await crypto.subtle.importKey("raw-secret",Buffer.from(p),"Argon2id",false,["deriveBits"]);const bits=await crypto.subtle.deriveBits({name:"Argon2id",memory:65536,passes:3,parallelism:1,nonce:salt},key,256);const b64=b=>Buffer.from(b).toString("base64").replace(/=+$/g,"");console.log(`$argon2id$v=19$m=65536,t=3,p=1$${b64(salt)}$${b64(bits)}`);})'
unset COMPOSIA_WEB_PASSWORD
```

Paste the full `$argon2id$...` output into `.env`. The command uses Docker to run Node.js 24, so it does not require a local Node.js install.

Generiere `WEB_SESSION_SECRET` mit einem beliebigen kryptografisch sicheren Zufallsgenerator, zum Beispiel:

```bash
openssl rand -hex 32
```

## Start

```bash
docker compose up -d
docker compose ps
```

Öffne die Web-UI unter `http://localhost:3000`.

## Rollenaufteilung

Die Compose-Datei ist nach Rolle gegliedert:

- **Controller-Stack**: `init-repo-controller`, `init-perms-controller`, `controller`.
- **Web-UI**: `web`.
- **Gemeinsame Initialisierung**: `init-config-perms`.
- **Agent-Stack**: `init-perms-agent`, `agent`.

Für alles, was über die All-in-One-Bereitstellung hinausgeht, trenne diese Abschnitte explizit für deine Topologie. Controller und Web können zusammen oder getrennt laufen. Jeder Agent-Node behält den Agent-Stack und seinen eigenen Docker-Socket-Zugriff.

## Images

Release-Images werden auf Forgejo, GHCR und Docker Hub veröffentlicht:

| Komponente | Forgejo | GHCR | Docker Hub |
|-----------|---------|------|------------|
| CLI | `forgejo.alexma.top/alexma233/composia-cli` | `ghcr.io/alexma233/composia-cli` | `alexma233/composia-cli` |
| Controller | `forgejo.alexma.top/alexma233/composia-controller` | `ghcr.io/alexma233/composia-controller` | `alexma233/composia-controller` |
| Agent | `forgejo.alexma.top/alexma233/composia-agent` | `ghcr.io/alexma233/composia-agent` | `alexma233/composia-agent` |
| Web | `forgejo.alexma.top/alexma233/composia-web` | `ghcr.io/alexma233/composia-web` | `alexma233/composia-web` |

Canary-Images werden nur auf Forgejo und GHCR veröffentlicht.

## Häufige Prüfungen

- Controller kann nicht starten: Überprüfe, ob `config.yaml` unter `COMPOSIA_CONFIG_DIR` existiert und dass erforderliche Controller-Pfade vorhanden sind oder erstellt werden können.
- Agent kann Docker nicht verwenden: Überprüfe, ob `DOCKER_SOCK_GID` mit `/var/run/docker.sock` auf dem Host übereinstimmt.
- Web kann Controller nicht erreichen: `WEB_CONTROLLER_ADDR` ist für den Webserver-Container, während `WEB_BROWSER_CONTROLLER_ADDR` für den Browser ist.
