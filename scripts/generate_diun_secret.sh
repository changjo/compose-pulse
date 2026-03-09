#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
ENV_FILE="${1:-$ROOT_DIR/.env}"

if [ ! -f "$ENV_FILE" ]; then
  echo "env file not found: $ENV_FILE" >&2
  exit 1
fi

generate_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 48 | tr -d '\n'
    return 0
  fi

  # Fallback without openssl.
  LC_ALL=C tr -dc 'A-Za-z0-9._~-' </dev/urandom | head -c 64
}

SECRET=$(generate_secret)
if [ -z "$SECRET" ]; then
  echo "failed to generate DIUN_WEBHOOK_SECRET" >&2
  exit 1
fi

TMP_FILE="${ENV_FILE}.tmp.$$"
trap 'rm -f "$TMP_FILE"' EXIT

awk -v secret="$SECRET" '
BEGIN { updated = 0 }
/^[[:space:]]*DIUN_WEBHOOK_SECRET=/ {
  print "DIUN_WEBHOOK_SECRET=" secret
  updated = 1
  next
}
{ print }
END {
  if (updated == 0) {
    print "DIUN_WEBHOOK_SECRET=" secret
  }
}
' "$ENV_FILE" >"$TMP_FILE"

mv "$TMP_FILE" "$ENV_FILE"
trap - EXIT

echo "updated DIUN_WEBHOOK_SECRET in $ENV_FILE"
