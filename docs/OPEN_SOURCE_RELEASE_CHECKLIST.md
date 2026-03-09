# Open Source Release Checklist

Last updated: 2026-03-09

## 1) Do not publish these files

- `.env`
- `.env.*` (except `.env.example`)
- local-only compose overrides such as `docker-compose.custom.yml`
- local runtime data under `.local/` or `data/`
- SQLite files (`*.db`, `*.db-shm`, `*.db-wal`)
- private keys/certs (`*.pem`, `*.key`, `*.p12`, `id_rsa`, `id_ed25519`)
- logs (`*.log`)

## 2) Required secret rotation (if already exposed)

If there is any chance secrets were committed or shared:

1. Rotate `ADMIN_PASSWORD`
2. Rotate `DIUN_WEBHOOK_SECRET`
3. Rotate `WEB_PUSH_VAPID_PRIVATE_KEY` (and regenerate public key)
4. Reconfigure DIUN and app env values

## 3) Pre-publish checks

Run from repository root:

```bash
# one-shot script
./scripts/prepublish_check.sh

# check if ignored secrets are still tracked
git ls-files .env '.env.*' '.local/*' 'data/*' '*.db' '*.pem' '*.key' '*.p12' id_rsa id_ed25519 docker-compose.custom.yml

# quick secret pattern scan
rg -n --hidden --glob '!.git' --glob '!.env' --glob '!.env.example' \
  '(ADMIN_PASSWORD=|DIUN_WEBHOOK_SECRET=|WEB_PUSH_VAPID_PRIVATE_KEY=|BEGIN PRIVATE KEY|BEGIN RSA PRIVATE KEY)'
```

Expected:

- `git ls-files` returns no sensitive files
- `rg` finds only placeholder values or helper scripts that generate/write placeholder env keys
- `LICENSE` file exists and matches the README license note

## 4) Safe defaults before publish

1. Keep `WEBHOOK_CONFIG_SHOW_SECRET` unset or `false`
2. Keep `WEB_PUSH_ENABLED=false` unless fully configured
3. Ensure README examples use placeholder values only
4. Ensure `.dockerignore` excludes local secret/runtime files used during image builds
5. Keep personal/local compose overrides out of the public tree

## 5) Runtime security reminders

1. Do not expose Docker socket publicly
2. Keep `/share/Container` mount read-only
3. Keep app writable path limited to `/data`
4. Keep DIUN webhook internal network call preferred (`http://composepulse:8087/api/diun/webhook`)
5. If using Caddy publicly, block external `POST /api/diun/webhook`
