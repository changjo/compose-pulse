#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

echo "[1/5] Sensitive file existence scan"
SENSITIVE_FILES="$(find . \
  \( -name ".env" -o -name ".env.*" -o -name "*.db" -o -name "*.db-shm" -o -name "*.db-wal" -o -name "*.sqlite" -o -name "*.sqlite3" -o -name "*.pem" -o -name "*.key" -o -name "*.crt" -o -name "*.p12" -o -name "id_rsa" -o -name "id_ed25519" -o -name "docker-compose.custom.yml" -o -path "./.local" -o -path "./.local/*" -o -path "./data" -o -path "./data/*" \) \
  -not -path "./.git/*" \
  -not -path "./.env.example" \
  -print)"
if [ -n "$SENSITIVE_FILES" ]; then
  echo "Found sensitive files (review before publish):"
  echo "$SENSITIVE_FILES"
else
  echo "OK: no sensitive file patterns found."
fi

echo
echo "[2/5] Secret pattern scan"
echo "Review hits manually; docs/examples/helper scripts may contain placeholder names but should not include real secrets."
if command -v rg >/dev/null 2>&1; then
  rg -n --hidden --glob '!.git' --glob '!.env' --glob '!.env.example' \
    '(ADMIN_PASSWORD=|DIUN_WEBHOOK_SECRET=|WEB_PUSH_VAPID_PRIVATE_KEY=|BEGIN PRIVATE KEY|BEGIN RSA PRIVATE KEY)' \
    || true
else
  echo "rg not found; skipped pattern scan."
fi

echo
echo "[3/5] Git tracked sensitive files scan"
if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  TRACKED="$(git ls-files .env '.env.*' '.local/*' 'data/*' '*.db' '*.db-shm' '*.db-wal' '*.sqlite' '*.sqlite3' '*.pem' '*.key' '*.crt' '*.p12' id_rsa id_ed25519 docker-compose.custom.yml || true)"
  if [ -n "$TRACKED" ]; then
    echo "Tracked sensitive files detected:"
    echo "$TRACKED"
    exit 1
  fi
  echo "OK: no tracked sensitive files."
else
  echo "Git repo not detected in current environment; skipped tracked-file scan."
fi

echo
echo "[4/5] License file check"
if [ -f LICENSE ]; then
  echo "OK: LICENSE file exists."
else
  echo "Missing LICENSE file."
  exit 1
fi

echo
echo "[5/5] Docker build context guard check"
if [ ! -f .dockerignore ]; then
  echo "Missing .dockerignore."
  exit 1
fi

DOCKERIGNORE_OK=1
for pattern in ".env" ".local" "data" "*.db" "docker-compose.custom.yml"; do
  if ! grep -qxF "$pattern" .dockerignore; then
    echo "Missing .dockerignore pattern: $pattern"
    DOCKERIGNORE_OK=0
  fi
done
if [ "$DOCKERIGNORE_OK" -eq 1 ]; then
  echo "OK: .dockerignore contains core local secret/runtime patterns."
else
  exit 1
fi

echo
echo "Prepublish check finished."
