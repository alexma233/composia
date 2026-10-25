#!/usr/bin/env sh
set -eu
umask 077

COMPOSE_URL="https://forgejo.alexma.top/alexma233/composia/raw/branch/main/docker-compose.yaml"
WEB_IMAGE="forgejo.alexma.top/alexma233/composia-web:latest"

prompt() {
  label=$1
  default=${2:-}
  if [ -n "$default" ]; then
    printf '%s [%s]: ' "$label" "$default" >&2
  else
    printf '%s: ' "$label" >&2
  fi
  IFS= read -r value
  printf '%s\n' "${value:-$default}"
}

prompt_secret() {
  label=$1
  printf '%s: ' "$label" >&2
  if [ -t 0 ]; then
    stty -echo
    trap 'stty echo' EXIT HUP INT TERM
    IFS= read -r value
    stty echo
    trap - EXIT HUP INT TERM
    printf '\n' >&2
  else
    IFS= read -r value
  fi
  printf '%s\n' "$value"
}

require_value() {
  name=$1
  value=$2
  if [ -z "$value" ]; then
    printf 'Error: %s cannot be empty.\n' "$name" >&2
    exit 1
  fi
  case "$value" in
    *"'"* | *"
"*)
      printf "Error: %s cannot contain a single quote or newline.\n" "$name" >&2
      exit 1
      ;;
  esac
}

require_identifier() {
  name=$1
  value=$2
  require_value "$name" "$value"
  case "$value" in
    *[!A-Za-z0-9._-]*)
      printf 'Error: %s may contain only letters, numbers, dot, underscore, and hyphen.\n' "$name" >&2
      exit 1
      ;;
  esac
}

require_url() {
  name=$1
  value=$2
  require_value "$name" "$value"
  case "$value" in
    http://?* | https://?*) ;;
    *) printf 'Error: %s must be an http:// or https:// URL.\n' "$name" >&2; exit 1 ;;
  esac
}

random_token() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  elif [ -r /dev/urandom ] && command -v od >/dev/null 2>&1; then
    od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
    printf '\n'
  else
    printf 'Error: openssl or od with /dev/urandom is required.\n' >&2
    exit 1
  fi
}

socket_gid() {
  if stat -c '%g' /var/run/docker.sock >/dev/null 2>&1; then
    stat -c '%g' /var/run/docker.sock
  else
    stat -f '%g' /var/run/docker.sock
  fi
}

hash_password() {
  printf '%s' "$1" | docker run --rm -i --network none --entrypoint deno "$WEB_IMAGE" eval '
const password = await new Response(Deno.stdin.readable).text();
const salt = crypto.getRandomValues(new Uint8Array(16));
const key = await crypto.subtle.importKey(
  "raw-secret",
  new TextEncoder().encode(password),
  "Argon2id",
  false,
  ["deriveBits"],
);
const digest = new Uint8Array(await crypto.subtle.deriveBits({
  name: "Argon2id",
  memory: 65536,
  passes: 3,
  parallelism: 1,
  nonce: salt,
}, key, 256));
const base64 = (value) => Uint8Array.from(value).toBase64().replace(/=+$/g, "");
console.log(`$argon2id$v=19$m=65536,t=3,p=1$${base64(salt)}$${base64(digest)}`);
'
}

create_directories() {
  case "$ROLE" in
    web)
      mkdir -p "$INSTALL_DIR"
      ;;
    controller)
      mkdir -p \
        "$INSTALL_DIR/config" \
        "$INSTALL_DIR/data/repo-controller" \
        "$INSTALL_DIR/data/state-controller" \
        "$INSTALL_DIR/data/logs"
      ;;
    agent)
      mkdir -p \
        "$INSTALL_DIR/config" \
        "$INSTALL_DIR/data/repo-agent" \
        "$INSTALL_DIR/data/state-agent"
      ;;
    all)
      mkdir -p \
        "$INSTALL_DIR/config" \
        "$INSTALL_DIR/data/repo-controller" \
        "$INSTALL_DIR/data/state-controller" \
        "$INSTALL_DIR/data/logs" \
        "$INSTALL_DIR/data/repo-agent" \
        "$INSTALL_DIR/data/state-agent"
      ;;
  esac
}

write_env() {
  cat > "$INSTALL_DIR/.env" <<EOF
COMPOSIA_CONFIG_DIR='$INSTALL_DIR/config'
COMPOSIA_CONTROLLER_REPO_DIR='$INSTALL_DIR/data/repo-controller'
COMPOSIA_CONTROLLER_STATE_DIR='$INSTALL_DIR/data/state-controller'
COMPOSIA_CONTROLLER_LOG_DIR='$INSTALL_DIR/data/logs'
COMPOSIA_AGENT_REPO_DIR='$INSTALL_DIR/data/repo-agent'
COMPOSIA_AGENT_STATE_DIR='$INSTALL_DIR/data/state-agent'
DOCKER_SOCK_GID=$DOCKER_SOCK_GID
WEB_CONTROLLER_ADDR='$WEB_CONTROLLER_ADDR'
WEB_BROWSER_CONTROLLER_ADDR='$WEB_BROWSER_CONTROLLER_ADDR'
WEB_CONTROLLER_ACCESS_TOKEN='$WEB_ACCESS_TOKEN'
WEB_LOGIN_USERNAME='$WEB_USERNAME'
WEB_LOGIN_PASSWORD_HASH='$WEB_PASSWORD_HASH'
WEB_SESSION_SECRET='$WEB_SESSION_SECRET'
ORIGIN='$WEB_ORIGIN'
HOST=0.0.0.0
PORT=3000
EOF
  chmod 600 "$INSTALL_DIR/.env"
}

write_config() {
  case "$ROLE" in
    controller)
      cat > "$INSTALL_DIR/config/config.yaml" <<EOF
controller:
  listen_addr: ":7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  access_tokens:
    - name: "web"
      token: '$WEB_ACCESS_TOKEN'
      enabled: true
  nodes:
    - id: '$NODE_ID'
      display_name: '$NODE_ID'
      enabled: true
      token: '$AGENT_TOKEN'
EOF
      ;;
    agent)
      cat > "$INSTALL_DIR/config/config.yaml" <<EOF
agent:
  controller_addr: '$AGENT_CONTROLLER_ADDR'
  node_id: '$NODE_ID'
  token: '$AGENT_TOKEN'
  repo_dir: '$INSTALL_DIR/data/repo-agent'
  state_dir: "/data/state-agent"
EOF
      ;;
    all)
      cat > "$INSTALL_DIR/config/config.yaml" <<EOF
controller:
  listen_addr: ":7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  access_tokens:
    - name: "web"
      token: '$WEB_ACCESS_TOKEN'
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: '$AGENT_TOKEN'
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: '$AGENT_TOKEN'
  repo_dir: '$INSTALL_DIR/data/repo-agent'
  state_dir: "/data/state-agent"
EOF
      ;;
  esac
  chmod 600 "$INSTALL_DIR/config/config.yaml"
}

compose() {
  docker compose \
    --project-directory "$INSTALL_DIR" \
    --env-file "$INSTALL_DIR/.env" \
    -f "$INSTALL_DIR/docker-compose.yaml" \
    "$@"
}

print_compose_command() {
  printf "docker compose --project-directory '%s' --env-file '%s/.env' -f '%s/docker-compose.yaml'" \
    "$INSTALL_DIR" "$INSTALL_DIR" "$INSTALL_DIR"
  for argument in "$@"; do
    printf " '%s'" "$argument"
  done
  printf '\n'
}

printf '%s\n' \
  'Composia Docker Compose installer' \
  '' \
  'Choose one installation role:' \
  '  1) Web UI only' \
  '  2) Controller only' \
  '  3) Agent only' \
  '  4) Web UI + Controller + Agent'
ROLE_CHOICE=$(prompt 'Role' '4')
case "$ROLE_CHOICE" in
  1 | web) ROLE=web ;;
  2 | controller) ROLE=controller ;;
  3 | agent) ROLE=agent ;;
  4 | all) ROLE=all ;;
  *) printf 'Error: invalid role.\n' >&2; exit 1 ;;
esac

INSTALL_DIR=$(prompt 'Absolute installation directory' "$PWD/composia")
require_value 'Installation directory' "$INSTALL_DIR"
case "$INSTALL_DIR" in
  /*) ;;
  *) printf 'Error: installation directory must be absolute.\n' >&2; exit 1 ;;
esac

command -v curl >/dev/null 2>&1 || { printf 'Error: curl is required.\n' >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { printf 'Error: Docker is required.\n' >&2; exit 1; }
docker compose version >/dev/null 2>&1 || { printf 'Error: Docker Compose v2 is required.\n' >&2; exit 1; }
docker info >/dev/null 2>&1 || { printf 'Error: the Docker daemon is not available.\n' >&2; exit 1; }

if [ -e "$INSTALL_DIR/.env" ] || [ -e "$INSTALL_DIR/config/config.yaml" ] || [ -e "$INSTALL_DIR/docker-compose.yaml" ]; then
  printf 'Error: installation files already exist under %s. Nothing was changed.\n' "$INSTALL_DIR" >&2
  exit 1
fi

WEB_CONTROLLER_ADDR=http://controller:7001
WEB_BROWSER_CONTROLLER_ADDR=http://localhost:7001
WEB_ACCESS_TOKEN=disabled
WEB_USERNAME=disabled
WEB_PASSWORD_HASH=disabled
WEB_SESSION_SECRET=disabled
WEB_ORIGIN=http://localhost:3000
AGENT_CONTROLLER_ADDR=http://controller:7001
AGENT_TOKEN=disabled
NODE_ID=main
DOCKER_SOCK_GID=0
WEB_PASSWORD=

case "$ROLE" in
  web)
    WEB_CONTROLLER_ADDR=$(prompt 'Controller URL reachable from the Web container')
    WEB_BROWSER_CONTROLLER_ADDR=$(prompt 'Controller URL reachable from browsers' "$WEB_CONTROLLER_ADDR")
    WEB_ACCESS_TOKEN=$(prompt_secret 'Controller access token')
    ;;
  controller)
    WEB_ACCESS_TOKEN=$(random_token)
    AGENT_TOKEN=$(random_token)
    NODE_ID=$(prompt 'Agent node ID to authorize' 'main')
    require_identifier 'Agent node ID' "$NODE_ID"
    ;;
  agent)
    AGENT_CONTROLLER_ADDR=$(prompt 'Controller URL reachable from the Agent container')
    NODE_ID=$(prompt 'Agent node ID' 'main')
    AGENT_TOKEN=$(prompt_secret 'Agent token')
    require_identifier 'Agent node ID' "$NODE_ID"
    ;;
  all)
    WEB_ACCESS_TOKEN=$(random_token)
    AGENT_TOKEN=$(random_token)
    WEB_BROWSER_CONTROLLER_ADDR=$(prompt 'Controller URL reachable from browsers' "$WEB_BROWSER_CONTROLLER_ADDR")
    ;;
esac

if [ "$ROLE" = web ] || [ "$ROLE" = all ]; then
  WEB_USERNAME=$(prompt 'Web login username' 'admin')
  WEB_PASSWORD=$(prompt_secret 'Web login password')
  WEB_PASSWORD_CONFIRMATION=$(prompt_secret 'Confirm web login password')
  [ "$WEB_PASSWORD" = "$WEB_PASSWORD_CONFIRMATION" ] || { printf 'Error: passwords do not match.\n' >&2; exit 1; }
  require_identifier 'Web login username' "$WEB_USERNAME"
  [ -n "$WEB_PASSWORD" ] || { printf 'Error: Web login password cannot be empty.\n' >&2; exit 1; }
  WEB_SESSION_SECRET=$(random_token)
  WEB_ORIGIN=$(prompt 'Public Web UI origin' 'http://localhost:3000')
fi

require_url 'Web controller URL' "$WEB_CONTROLLER_ADDR"
require_url 'Browser controller URL' "$WEB_BROWSER_CONTROLLER_ADDR"
require_url 'Agent controller URL' "$AGENT_CONTROLLER_ADDR"
require_url 'Web origin' "$WEB_ORIGIN"
require_value 'Controller access token' "$WEB_ACCESS_TOKEN"
require_value 'Agent token' "$AGENT_TOKEN"

if [ "$ROLE" = agent ] || [ "$ROLE" = all ]; then
  [ -S /var/run/docker.sock ] || { printf 'Error: /var/run/docker.sock is required for the Agent.\n' >&2; exit 1; }
  DOCKER_SOCK_GID=$(socket_gid)
fi

case "$ROLE" in
  web) SERVICES=web ;;
  controller) SERVICES=controller ;;
  agent) SERVICES=agent ;;
  all) SERVICES='controller web agent' ;;
esac

printf '\n%s\n' 'Execution plan (secrets are intentionally redacted):'
printf '  Role:             %s\n' "$ROLE"
printf '  Install directory: %s\n' "$INSTALL_DIR"
printf '  Services:         %s\n\n' "$SERVICES"
printf '%s\n' '  1. Create only the directories required by the selected role:'
case "$ROLE" in
  web) printf "     mkdir -p '%s'\n" "$INSTALL_DIR" ;;
  controller) printf "     mkdir -p '%s/config' '%s/data/repo-controller' '%s/data/state-controller' '%s/data/logs'\n" "$INSTALL_DIR" "$INSTALL_DIR" "$INSTALL_DIR" "$INSTALL_DIR" ;;
  agent) printf "     mkdir -p '%s/config' '%s/data/repo-agent' '%s/data/state-agent'\n" "$INSTALL_DIR" "$INSTALL_DIR" "$INSTALL_DIR" ;;
  all) printf "     mkdir -p '%s/config' '%s/data/repo-controller' '%s/data/state-controller' '%s/data/logs' '%s/data/repo-agent' '%s/data/state-agent'\n" "$INSTALL_DIR" "$INSTALL_DIR" "$INSTALL_DIR" "$INSTALL_DIR" "$INSTALL_DIR" "$INSTALL_DIR" ;;
esac
printf '%s\n' \
  "  2. curl -fL '$COMPOSE_URL' -o '$INSTALL_DIR/docker-compose.yaml.tmp', then rename it" \
  "  3. Hash the Web password locally when required (otherwise skip)" \
  "  4. Generate '$INSTALL_DIR/.env' and config/config.yaml when required (mode 600; secrets redacted)"
if [ "$ROLE" = web ] || [ "$ROLE" = all ]; then
  printf "     Password command: docker run --rm -i --network none --entrypoint deno '%s' eval <Argon2id code>\n" "$WEB_IMAGE"
fi
printf '  5. '; print_compose_command config --quiet
# The unquoted expansion intentionally passes the selected service names as separate arguments.
# shellcheck disable=SC2086
printf '  6. '; print_compose_command pull --include-deps $SERVICES
# shellcheck disable=SC2086
printf '  7. '; print_compose_command up -d $SERVICES
# shellcheck disable=SC2086
printf '  8. '; print_compose_command ps $SERVICES
printf '%s\n' '  9. Print completion paths; never print secrets'
printf '\nNo package manager, firewall, Docker daemon, or system service settings will be changed.\n'

CONFIRM=$(prompt 'Execute this plan? [y/N]' 'N')
case "$CONFIRM" in
  y | Y | yes | YES) ;;
  *) printf 'Cancelled. Nothing was changed.\n'; exit 0 ;;
esac

printf '\n[1/9] Creating installation directories\n'
printf '+ create only the directories listed in step 1\n'
create_directories

printf '\n[2/9] Downloading the public Compose file\n'
printf "+ curl -fL '%s' -o '%s/docker-compose.yaml.tmp'\n" "$COMPOSE_URL" "$INSTALL_DIR"
curl -fL "$COMPOSE_URL" -o "$INSTALL_DIR/docker-compose.yaml.tmp"
mv "$INSTALL_DIR/docker-compose.yaml.tmp" "$INSTALL_DIR/docker-compose.yaml"
chmod 644 "$INSTALL_DIR/docker-compose.yaml"

if [ "$ROLE" = web ] || [ "$ROLE" = all ]; then
  printf '\n[3/9] Hashing the Web password locally (password is passed on stdin)\n'
  printf "+ docker run --rm -i --network none --entrypoint deno '%s' eval '<Argon2id code>'\n" "$WEB_IMAGE"
  WEB_PASSWORD_HASH=$(hash_password "$WEB_PASSWORD")
  WEB_PASSWORD=
  WEB_PASSWORD_CONFIRMATION=
else
  printf '\n[3/9] Web password hash not required for this role; skipped\n'
fi

printf '\n[4/9] Writing generated configuration\n'
printf "+ write '%s/.env' with mode 600 (secrets redacted)\n" "$INSTALL_DIR"
write_env
if [ "$ROLE" != web ]; then
  printf "+ write '%s/config/config.yaml' with mode 600 (secrets redacted)\n" "$INSTALL_DIR"
  write_config
else
  printf '+ config/config.yaml is not required for Web-only installation; skipped\n'
fi

printf '\n[5/9] Validating the rendered Compose configuration\n'
printf '+ '; print_compose_command config --quiet
compose config --quiet

printf '\n[6/9] Pulling selected images\n'
# shellcheck disable=SC2086
printf '+ '; print_compose_command pull --include-deps $SERVICES
# shellcheck disable=SC2086
compose pull --include-deps $SERVICES

printf '\n[7/9] Starting selected services\n'
# shellcheck disable=SC2086
printf '+ '; print_compose_command up -d $SERVICES
# shellcheck disable=SC2086
compose up -d $SERVICES

printf '\n[8/9] Showing service status\n'
# shellcheck disable=SC2086
printf '+ '; print_compose_command ps $SERVICES
# shellcheck disable=SC2086
compose ps $SERVICES

printf '\n[9/9] Installation complete\n'
printf 'Configuration: %s/.env\n' "$INSTALL_DIR"
if [ "$ROLE" != web ]; then
  printf 'Runtime config: %s/config/config.yaml\n' "$INSTALL_DIR"
fi
printf 'Secrets were not printed. Inspect the mode-600 configuration files (with sudo after container initialization) when transferring credentials to another node.\n'
