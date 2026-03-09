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
git ls-files -- .env '.env.*' '.local/*' 'data/*' '*.db' '*.pem' '*.key' '*.p12' id_rsa id_ed25519 docker-compose.custom.yml | grep -v '^\.env\.example$'

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

## 5) GitHub release automation setup

Before pushing the first release tag, configure these GitHub repository settings:

1. Repository variable `DOCKERHUB_USERNAME` (required)
2. Repository secret `DOCKERHUB_TOKEN` (required)
3. Repository variable `DOCKERHUB_NAMESPACE` (optional, defaults to `DOCKERHUB_USERNAME`)
4. Repository variable `DOCKERHUB_IMAGE_NAME` (optional, defaults to the repository name)

Release workflow behavior:

1. `.github/workflows/ci.yml` verifies `main` and pull requests
2. `.github/workflows/release.yml` runs on tags that match `v*`
3. Stable tags publish multi-arch Docker Hub images for `linux/amd64` and `linux/arm64`
4. Stable tags also publish `latest`; prerelease tags do not
5. A GitHub Release is created automatically from the pushed tag

Suggested release command:

```bash
git tag v0.1.0
git push origin v0.1.0
```

After the workflow finishes:

1. Confirm the GitHub Release exists
2. Confirm the Docker Hub image exists for the exact tag
3. Confirm stable tags also published `v0.1` and `latest`

## 6) Runtime security reminders

1. Do not expose Docker socket publicly
2. Keep `/share/Container` mount read-only
3. Keep app writable path limited to `/data`
4. Keep DIUN webhook internal network call preferred (`http://composepulse:8087/api/diun/webhook`)
5. If using Caddy publicly, block external `POST /api/diun/webhook`
