#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TEST_DIR=$(mktemp -d)
trap 'rm -rf "$TEST_DIR"' EXIT
mkdir -p "$TEST_DIR/bin"

cat > "$TEST_DIR/bin/curl" <<'EOF'
#!/usr/bin/env sh
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then
    output=$2
    shift 2
  else
    shift
  fi
done
cp "$SOURCE_COMPOSE" "$output"
EOF

cat > "$TEST_DIR/bin/docker" <<'EOF'
#!/usr/bin/env sh
case "$*" in
  'compose version' | 'info') exit 0 ;;
  *) printf '%s\n' "$*" >> "$DOCKER_LOG" ;;
esac
EOF
chmod +x "$TEST_DIR/bin/curl" "$TEST_DIR/bin/docker"

INSTALL_DIR="$TEST_DIR/install"
printf '2\n%s\n\ny\n' "$INSTALL_DIR" | \
  PATH="$TEST_DIR/bin:$PATH" \
  SOURCE_COMPOSE="$ROOT_DIR/docker-compose.yaml" \
  DOCKER_LOG="$TEST_DIR/docker.log" \
  sh "$ROOT_DIR/install.sh" > "$TEST_DIR/output.log" 2> "$TEST_DIR/prompts.log"

[ -f "$INSTALL_DIR/docker-compose.yaml" ]
[ "$(stat -c '%a' "$INSTALL_DIR/.env")" = 600 ]
[ "$(stat -c '%a' "$INSTALL_DIR/config/config.yaml")" = 600 ]
grep -q '^controller:' "$INSTALL_DIR/config/config.yaml"
grep -q 'Execution plan' "$TEST_DIR/output.log"
grep -q 'docker compose --project-directory' "$TEST_DIR/output.log"
grep -q 'config --quiet' "$TEST_DIR/docker.log"
grep -q 'up -d controller' "$TEST_DIR/docker.log"

CANCEL_DIR="$TEST_DIR/cancelled"
printf '2\n%s\n\nn\n' "$CANCEL_DIR" | \
  PATH="$TEST_DIR/bin:$PATH" \
  SOURCE_COMPOSE="$ROOT_DIR/docker-compose.yaml" \
  DOCKER_LOG="$TEST_DIR/docker.log" \
  sh "$ROOT_DIR/install.sh" >/dev/null 2>&1
[ ! -e "$CANCEL_DIR" ]

ALL_DIR="$TEST_DIR/all"
if printf '4\n%s\ninvalid-controller-url\nadmin\npassword\npassword\nhttps://app.example.com\ny\n' "$ALL_DIR" | \
  PATH="$TEST_DIR/bin:$PATH" \
  SOURCE_COMPOSE="$ROOT_DIR/docker-compose.yaml" \
  DOCKER_LOG="$TEST_DIR/docker.log" \
  sh "$ROOT_DIR/install.sh" > "$TEST_DIR/all-output.log" 2> "$TEST_DIR/all-prompts.log"; then
  printf '%s\n' 'Error: invalid browser controller URL was accepted.' >&2
  exit 1
fi
grep -q 'Browser controller URL must be an http:// or https:// URL' "$TEST_DIR/all-prompts.log"
[ ! -e "$ALL_DIR" ]

# Compose escapes dollar signs so the container shell, not Compose, expands the path.
INIT_COMMAND=$(awk '
  /^  init-perms-agent:$/ { in_service = 1; next }
  in_service && /^    volumes:/ { exit }
  in_service && /^        / {
    sub(/^        /, "")
    gsub(/\$\$/, "$")
    print
  }
' "$ROOT_DIR/docker-compose.yaml")
[ -n "$INIT_COMMAND" ]
(
  COMPOSIA_AGENT_REPO_DIR="$TEST_DIR/agent repo;\$(exit 1)"
  mkdir() {
    [ "$#" -eq 2 ] && [ "$1" = -p ] && [ "$2" = /data/state-agent ]
  }
  chown() {
    [ "$#" -eq 4 ] && [ "$1" = -R ] && [ "$2" = 65532:65532 ] &&
      [ "$3" = "$COMPOSIA_AGENT_REPO_DIR" ] && [ "$4" = /data/state-agent ]
  }
  eval "$INIT_COMMAND"
)

printf '%s\n' 'install script check passed'
