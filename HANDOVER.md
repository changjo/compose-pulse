# HANDOVER.md

Last updated: 2026-03-09

## 1) Current Status

Implemented MVP for the planned ComposePulse web app:

- Go web server with embedded dashboard UI
- SQLite schema/migrations for targets/jobs/logs/diun events/webhook receipts/app settings
- Session-based login (`/api/auth/*`) for dashboard access
- Login-first dashboard UX (hide all operational panels until authenticated)
- Remember-me login option with extended session TTL
- Fixed webhook secret via env for stable DIUN integration
- Manual target registration and toggle updates
- Manual update job enqueue and prune job enqueue
- In-process sequential worker for job execution
- SSE log stream per job
- DIUN webhook intake + image repo matching + cooldown filtering + auto enqueue
- 24h metrics endpoint for failed jobs / avg duration / webhook failure rate
- Docker deployment files and README
- Unit tests for key validation/parsing logic
- Integration tests for session auth, job lifecycle, and webhook secret handling
- API error response normalization (`{error, code}`) across API handlers
- SSE guardrail: max connection cap + active/rejected monitoring fields + open/close logs
- Target deletion endpoint with active-job safety check
- Jobs cursor pagination (`GET /api/jobs?limit=&cursor=`)
- DIUN payload fixture-based parser tests (`testdata/diun_payloads`)
- Auto-update maintenance window option (`AUTO_UPDATE_WINDOW_START_HOUR`, `AUTO_UPDATE_WINDOW_END_HOUR`)
- Job history CSV export endpoint (`GET /api/jobs/export.csv`)
- Job history delete endpoint (`DELETE /api/jobs/{id}` with queued/running guard)
- Job history bulk delete endpoint (`POST /api/jobs/delete-all`, blocked when queued/running exists)
- Target audit summary endpoint (`GET /api/audit/targets`)
- Dashboard language toggle (ko/en)
- Mobile responsive UI improvements (table scroll wrappers, touch-friendly controls)
- PWA support (`manifest.webmanifest`, `sw.js`, install prompt/help for Android/iOS)
- Separate login page flow (`/login`) and dashboard routing by session
- Container discovery registration flow (`GET /api/containers/discover`)
- Compose image parsing with variable resolution (`.env` + process env)
- Multi-image target mapping (`image_repos`) and webhook matching by mapping table
- Secret helper script: `scripts/generate_diun_secret.sh`
- Local UI preview helper script: `scripts/dev_local_ui.sh`
- Docker build architecture hardcoding removed (dropped `GOARCH=amd64`) for Apple Silicon and local portability
- Hot-reload local UI mode: `scripts/dev_local_ui.sh start-hot` (`WEB_DIR` runtime static serving)
- Added `.env.example` for compose-required variables (admin, webhook secret, data bind path)
- Dashboard visual refresh: Dieter Rams-inspired minimal UI (reduced decoration, neutral palette)
- Button style refinement for Rams-like hierarchy (neutral default, restrained danger, focused primary)
- Mobile table UX refresh: stacked card rows with field labels (`data-label`) to avoid horizontal overflow
- Auth UX hardening: unauth `/` now redirects to `/login`; client-side redirect guard to reduce login refresh loops
- Table readability tune: checkbox size normalized and aggressive word-break removed for headers/buttons/cell text
- Pull resiliency: `docker compose pull` now retries with configurable backoff (`PULL_RETRY_MAX_ATTEMPTS`, `PULL_RETRY_DELAY_SECONDS`)
- Mobile UX reliability: target selection now persists across 10s auto-refresh to prevent accidental deselection before "Run Selected Update"
- Mobile usability: targets section now supports collapse/expand toggle (default collapsed on narrow viewport)
- Mobile usability: audit summary section now supports collapse/expand toggle (default collapsed on narrow viewport)
- Auth hardening: login rate limit (`IP+username`, `429` + `Retry-After`) with configurable envs
- Metrics expansion: login failures/rate-limited counts + webhook failed count in 24h
- DIUN observability: webhook receipts endpoint (`GET /api/diun/receipts`) with `status_code/reason_code/queued_job_id`
- Webhook reason code standardization (`secret_mismatch`, `payload_invalid`, `no_match`, `queued`, `queue_full`, `internal_error`, `auto_disabled`, `cooldown_blocked`, `outside_maintenance_window`)
- Image repo canonicalization extended for Docker Hub shorthand/alias matching (`nginx` <-> `docker.io/library/nginx`, `index.docker.io` -> `docker.io`)
- Pull retry optimization: `toomanyrequests` `retry-after` parsing with clamp (2s~120s) and retry log visibility
- Advanced UX: dashboard advanced sections hidden by default, toggle state persisted via localStorage, advanced polling paused when OFF
- Dashboard realtime refresh reworked: default 10s polling removed, `/api/stream/dashboard` SSE patch stream added with reconnect backoff + fallback polling(30s)
- Mobile pull-to-refresh added (top pull gesture threshold + single-flight refresh)
- Optional Web Push foundation added: `/api/push/config`, `/api/push/subscriptions`, `/api/push/test`, `push_subscriptions`/`push_delivery_logs` tables
- Push device-state accuracy fix: `/api/push/config?endpoint=` now supports per-device subscribed status (not just global any-subscription)
- Push disable hardening: calling `DELETE /api/push/subscriptions` without an endpoint now disables all active subscriptions
- Push diagnostics: `/api/push/test` response includes `last_error` when failures occur
- Web Push subject normalization: plain email/`mailto: admin@...` values normalized to valid `mailto:` subject
- Service worker push handlers added (`push`, `notificationclick`) and Advanced push control panel added in UI
- iOS PWA badge handling added: set a badge on push receipt and clear it when the app opens or the notification is clicked (supported browsers only)
- iOS badge reliability fix: removed the path that cleared the badge immediately on `push-event`; the service worker now forwards `badge_count` to the app for more consistent behavior
- Frontend resilience hardening: service worker readiness now times out/falls back, and Advanced loaders use isolated batches so one failing API/push lookup does not stall the whole dashboard
- Mobile density compact mode: Registered Containers, Audit, DIUN Events, Webhook Receipts, and Job History now render as one-line compact rows on mobile
- Mobile compact detail accordion: all five sections support expand/collapse by row tap or detail button (one open item per section), and Job History separates the log button on mobile
- Mobile metrics density tweak: the Operational Metrics panel now uses a 2-column grid on common phone widths and falls back to 1 column only below 360px
- Pull-to-refresh rework: touch devices now use a touch-first refresh gesture path with stronger top-of-page/horizontal-drag guards, the mobile indicator is now an iPhone-style circular progress bubble that fills while pulling and spins during refresh, and the service worker static cache key was bumped so updated UI assets replace older PWA-cached copies
- Release hardening fix: startup migration now normalizes legacy `targets.image_repo` and `target_image_repos` values to canonical repository names so older Docker Hub shorthand targets continue matching DIUN webhook payloads after upgrade
- DIUN webhook hardening: payload extraction now also accepts `entry.image` string payloads, and webhook receipt failures now store explicit `queue_full` / `internal_error` reason codes so failed auto-update attempts no longer appear as a generic unknown reason
- Dashboard metrics hardening: `/api/metrics` now falls back to zero when optional telemetry tables are missing in older DBs instead of failing the entire metrics panel, and the frontend metrics renderer now skips absent DOM nodes instead of aborting the whole card update
- Frontend asset cache hardening: `/` and `/login` now serve versioned JS/CSS/manifest URLs plus `Cache-Control: no-store`, and `app.js` registers `sw.js` with the same asset version so stale browser/PWA caches no longer keep an older metrics renderer paired with newer HTML
- Webhook config hardening: `/api/diun/webhook/config` now returns only a masked secret by default (raw value only when `WEBHOOK_CONFIG_SHOW_SECRET=true`)
- Frontend behavior change: the Advanced view no longer auto-calls `/api/diun/webhook/config`; it shows only the path, header name, and a hidden-secret notice
- Open-source prep: strengthened local runtime/secret patterns in `.gitignore` and added `docs/OPEN_SOURCE_RELEASE_CHECKLIST.md`
- Open-source prep helper script: `scripts/prepublish_check.sh` for scanning sensitive files, patterns, and tracked-file state
- README refresh: reorganized the docs around app overview, quick start, day-to-day usage order, and DIUN+Caddy operational guidance
- README tuning: reduced environment-specific guidance and centered reverse-proxy examples on Caddy and Traefik
- README clarity pass: repositioned the project as a safe self-hosted Docker Compose updater, simplified first-run steps, and added public screenshots under `docs/screenshots/login.png` and `docs/screenshots/dashboard.png`
- README requirement pass: clarified that DIUN is required for automatic updates, while manual-only operation can start without DIUN; linked the official DIUN docs in prerequisites and setup steps
- README portability pass: generalized the public wording away from `/share/Container`-specific messaging and explained that non-QNAP hosts should override `CONTAINER_ROOT` plus the matching bind mount in a local compose override
- README branding polish: added a rounded-corner README-specific app icon near the top of the README for clearer project identity on GitHub
- UI branding polish: added the app icon to the login card header and the dashboard topbar so the app identity is visible in the top-left of both primary screens
- Local-only compose override policy added: environment-specific override files stay outside the public repo
- Key generation helper added: `scripts/generate_vapid_keys.sh` (Dockerized Go helper that generates VAPID public/private keys and writes them into `.env`)
- README key guide added: documented the purpose, generation flow, rotation timing, and operating notes for `DIUN_WEBHOOK_SECRET`, `WEB_PUSH_VAPID_*`, and `WEB_PUSH_SUBJECT`
- MIT License file added for public release
- Branding update: standardized the user-facing name as `ComposePulse` across the UI title, login page, PWA manifest, push default title, and log lines
- Identifier rename complete (pre-deploy): changed default service/image/container/module/cookie/localStorage/cache keys to `composepulse`
- PWA icon pack added: generated `web/icons/app-icon-{32,64,180,192,512}.png` and wired them into the manifest, index, and service worker
- PWA icon redesign applied: replaced the icon with a simpler ComposePulse concept (`C` + pulse line + stack bars) and bumped the service worker cache key to `v8`
- PWA icon simplified again: reduced the icon to a Compose `C` + single pulse symbol and bumped the cache key to `v9`
- PWA icon polish: kept the `C + pulse` symbol while rebalancing the background gradient and stroke thickness, and bumped the cache key to `v10`
- Metrics extended with dashboard stream/push delivery counters (`dashboard_stream_active`, `dashboard_stream_rejected_total`, `push_sent_24h`, `push_failed_24h`)
- Integration tests expanded for dashboard stream auth/patch delivery, push subscription APIs, metrics push counters
- Branding neutralization for open source: removed `QNAP` terminology from README, AGENTS, HANDOVER, and comments in `main.go`
- PWA icon concept exploration: added A/B/C comparison proposals (512px/64px) under `web/icons/proposals/composepulse-{a,b,c}-{512,64}.png`
- PWA icon redesign (Liquid Glass style): applied an app-tile background, glass highlight layer, and glass-treated symbol, then updated the `-v3` icon set, manifest, and service worker cache key (`v19`) for iOS
- Release QA baseline (repo-side): Dockerized `go test ./...`, prepublish check, preview boot, login/rate-limit, manual update, prune, auth denial, webhook secret mismatch, DIUN queue/match, CSV export, and path-safety checks passed; current public release scope is desktop-first, while mobile/PWA validation is deferred to follow-up and should not be advertised as release-gated support yet
- GitHub Actions release automation added: CI now runs prepublish checks plus Dockerized Go tests on `main`/PRs, and tag pushes matching `v*` now publish multi-arch Docker Hub images (`linux/amd64`, `linux/arm64`) and create GitHub Releases automatically; Docker Hub config is documented in the README and open-source release checklist
- Prepublish release guard fix: `scripts/prepublish_check.sh` no longer flags `.env.example` as a tracked sensitive file, so the new GitHub Actions CI/release workflow can pass with the intended public sample env file committed
- Compose file naming cleanup: the public default is now `docker-compose.yml` for Docker Hub image deployments, while local source builds moved to `docker-compose.build.yml`; the README and sample env now reflect the new default path
- In-app version visibility added: the login screen and dashboard header now show the embedded app version, release builds inject the Git tag through Docker `APP_VERSION`, and local source builds can override the default `dev` label with `COMPOSEPULSE_APP_VERSION`

Key files:

- `main.go`
- `main_test.go`
- `web/index.html`
- `web/app.js`
- `web/styles.css`
- `docker-compose.yml`
- `docker-compose.build.yml`
- `README.md`
- `UI_DECISIONS.md`

## 2) Non-Negotiable Constraints Enforced

- Login session cookie required for dashboard API endpoints.
- `X-DIUN-SECRET` required for webhook (fixed via `DIUN_WEBHOOK_SECRET` env in compose).
- `compose_dir` validation: only `/share/Container/<name>` accepted.
- `compose_file` must be filename only (no path traversal).
- `DB_PATH` restricted to `/data` subtree.
- Allowed operational commands are fixed and minimal.
- Prune command is only `docker image prune -f` (no `-a` mode).

## 3) Verified

- Format + tests passed using Dockerized Go toolchain:
  - `docker run --rm -v "$PWD":/src -w /src golang:1.22-alpine sh -lc 'apk add --no-cache build-base >/dev/null && /usr/local/go/bin/gofmt -w main.go integration_test.go main_test.go diun_fixture_test.go && CGO_ENABLED=1 /usr/local/go/bin/go test ./...'`
  - result: `ok composepulse`

## 4) Remaining Work (Prioritized)

P0 (before production):

Completed:

1. Integration tests for auth/lifecycle/webhook
2. Caddy production setup docs
3. API error response normalization
4. SSE connection cap + monitoring

P1 (recommended next):

Completed:

1. `DELETE /api/targets/{id}` with active job safety check
2. `/api/jobs` cursor pagination
3. maintenance window auto-update option
4. DIUN fixture compatibility tests

P2 (nice to have):

Completed:

1. Internationalization toggle (Korean/English labels).
2. CSV export for job history.
3. Lightweight audit summary panel by target.

## 5) Production Rollout Checklist

1. Create `.env` with strong values:
   - `ADMIN_USERNAME` (optional, default `admin`)
   - `ADMIN_PASSWORD` (recommended)
   - `DIUN_WEBHOOK_SECRET` (required)
   - `APP_DATA_BIND_DIR` (recommended)
   - `COOLDOWN_SECONDS`
2. Deploy app:
   - `docker compose up -d` for the published image path, or `docker compose -f docker-compose.build.yml up -d --build` for a local source build
3. Confirm mounts:
   - `/var/run/docker.sock`
   - `/share/Container:/share/Container:ro`
   - bind mount for `/data` (e.g. `/share/Container/composepulse/data:/data`)
4. Configure Caddy BasicAuth + reverse proxy.
5. Log in once and confirm `/api/diun/webhook/config` secret.
6. Configure DIUN webhook URL and `X-DIUN-SECRET`.
7. Register one low-risk target first and enable auto-update for canary run.
8. Observe metrics/jobs for at least 1 week before expanding auto-update scope.

## 6) Known Risks

- Docker socket mount grants high privilege to app container. Mitigation: strict app validation, minimal command surface, network isolation.
- Different DIUN payload variants may require additional field extraction rules.
- SQLite + single worker is intentional for safety; heavy scale is out of scope.

## 7) Suggested Next Session Start Prompt

"Read `HANDOVER.md` and execute P4 canary ops: enable Web Push for one admin device, collect 1-week stream/push metrics, and tune fallback/reconnect thresholds."
