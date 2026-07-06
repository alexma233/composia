#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
DEV_DIR="$ROOT_DIR/dev"

CONTROLLER_TOKEN_PLACEHOLDER="replace-with-dev-controller-token"
MAIN_AGENT_TOKEN_PLACEHOLDER="replace-with-dev-main-agent-token"
DEV_AGE_SECRET="AGE-SECRET-KEY-1KTAWUUQUUMF6FTXQZ8R2EAHHS2PCMMMTNHZFE20UULV27XFURL3S9D40AT"
DEV_AGE_RECIPIENT="age1wdc28dv5vcjz8yewrzemr0xmyhz6jvgan4s2mrzvd0f9sh6rm9rqhgkkkf"

random_token() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
    return
  fi

  if [ -r /dev/urandom ] && command -v od >/dev/null 2>&1; then
    od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
    printf '\n'
    return
  fi

  printf 'dev-token-%s\n' "$(date +%s)"
}

ensure_token_file() {
  file=$1
  if [ ! -s "$file" ]; then
    random_token > "$file"
    chmod 600 "$file"
  fi
}

read_first_line() {
  file=$1
  IFS= read -r line < "$file"
  printf '%s\n' "$line"
}

replace_placeholder() {
  file=$1
  placeholder=$2
  value=$3

  if [ ! -f "$file" ] || ! grep -q "$placeholder" "$file"; then
    return
  fi

  tmp=$(mktemp)
  sed "s|$placeholder|$value|g" "$file" > "$tmp"
  mv "$tmp" "$file"
}

copy_if_missing() {
  src=$1
  dest=$2
  if [ ! -e "$dest" ]; then
    cp "$src" "$dest"
  fi
}

migrate_container_config() {
  file=$1
  if [ ! -f "$file" ]; then
    return
  fi

  tmp=$(mktemp)
  sed -E \
    -e 's|repo_dir: ".*/dev/repo-controller"|repo_dir: "/data/repo-controller"|' \
    -e 's|repo_dir: ".*/dev/repo-agent"|repo_dir: "/data/repo-agent"|' \
    "$file" > "$tmp"
  mv "$tmp" "$file"
}

ensure_age_files() {
  identity_file="$DEV_DIR/age-identity.key"
  recipient_file="$DEV_DIR/age-recipients.txt"

  if [ ! -s "$identity_file" ] || ! grep -q '^AGE-SECRET-KEY-' "$identity_file"; then
    if command -v age-keygen >/dev/null 2>&1; then
      tmp=$(mktemp)
      age-keygen -o "$tmp" >/dev/null
      mv "$tmp" "$identity_file"
    else
      {
        printf '# public key: %s\n' "$DEV_AGE_RECIPIENT"
        printf '%s\n' "$DEV_AGE_SECRET"
      } > "$identity_file"
    fi
    chmod 600 "$identity_file"
  fi

  if [ ! -s "$recipient_file" ] || ! grep -q '^age1' "$recipient_file"; then
    public_key=$(awk '/^# public key: / { print $4; exit }' "$identity_file")
    if [ -z "$public_key" ]; then
      public_key="$DEV_AGE_RECIPIENT"
    fi
    printf '%s\n' "$public_key" > "$recipient_file"
    chmod 644 "$recipient_file"
  fi
}

mkdir -p \
  "$DEV_DIR/logs" \
  "$DEV_DIR/repo-agent" \
  "$DEV_DIR/repo-agent-node-2" \
  "$DEV_DIR/repo-controller" \
  "$DEV_DIR/state-agent" \
  "$DEV_DIR/state-agent-node-2" \
  "$DEV_DIR/state-controller"

ensure_token_file "$DEV_DIR/controller-access-token.txt"
ensure_token_file "$DEV_DIR/main-agent-token.txt"
ensure_token_file "$DEV_DIR/node-2-agent-token.txt"

controller_token=$(read_first_line "$DEV_DIR/controller-access-token.txt")
main_agent_token=$(read_first_line "$DEV_DIR/main-agent-token.txt")

copy_if_missing "$DEV_DIR/.env.example" "$DEV_DIR/.env"
replace_placeholder "$DEV_DIR/.env" "$CONTROLLER_TOKEN_PLACEHOLDER" "$controller_token"

copy_if_missing "$DEV_DIR/config.controller.container.yaml.example" "$DEV_DIR/config.controller.container.yaml"
migrate_container_config "$DEV_DIR/config.controller.container.yaml"
replace_placeholder "$DEV_DIR/config.controller.container.yaml" "$CONTROLLER_TOKEN_PLACEHOLDER" "$controller_token"
replace_placeholder "$DEV_DIR/config.controller.container.yaml" "$MAIN_AGENT_TOKEN_PLACEHOLDER" "$main_agent_token"

copy_if_missing "$DEV_DIR/config.controller.yaml.example" "$DEV_DIR/config.controller.yaml"
copy_if_missing "$DEV_DIR/config.agent.yaml.example" "$DEV_DIR/config.agent.yaml"

ensure_age_files

if [ ! -d "$DEV_DIR/repo-controller/.git" ] && command -v git >/dev/null 2>&1; then
  git init -q "$DEV_DIR/repo-controller"
fi

printf '%s\n' 'Development files are ready.'
