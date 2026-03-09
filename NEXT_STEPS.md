# NEXT_STEPS.md

## Immediate (P0)

- [x] Add API integration tests (session login failure/success, job state transitions, webhook secret validation)
- [x] Standardize API error responses as `{error: "...", code: "..."}`
- [x] Improve SSE connection stability (connection cap, active/rejected metrics, disconnect cleanup logs)
- [x] Expand README production setup guidance for Caddy

## Short Term (P1)

- [x] Implement `DELETE /api/targets/{id}`
- [x] Add cursor-based pagination to `/api/jobs`
- [x] Expand DIUN payload parsing tests with real fixture samples
- [x] Add maintenance window controls for auto-update

## Nice to Have (P2)

- [x] Add dashboard language toggle (ko/en)
- [x] Add CSV export for job history (`GET /api/jobs/export.csv`)
- [x] Add audit summary panel by target (`GET /api/audit/targets`)
- [x] Hide dashboard before login and support automatic login (`remember me`)
- [x] Improve mobile UI and add iOS/Android PWA install support
- [x] Replace manual target entry with discovery-based registration and split out the login page
- [x] Add compose-variable-based image parsing and per-target multi-image mapping (`image_repos`)

## Stability (P4)

- [x] Remove 10-second dashboard polling and switch to `/api/stream/dashboard` SSE patch updates
- [x] Add mobile pull-to-refresh
- [x] Add optional Web Push APIs/storage/service worker integration (`/api/push/*`)
- [x] Expand operational metrics (`dashboard_stream_*`, `push_*_24h`)
- [x] Add integration tests for dashboard stream auth/patch delivery, push APIs, and metrics expansion
- [ ] Run a one-week Web Push canary with one admin device, then re-evaluate default policy

## Ops

- [ ] Run best-effort mobile smoke on one iPhone (Safari/PWA) and one Android device (Chrome/PWA) before expanding mobile support claims
- [ ] Enable auto-update for one canary container only
- [ ] Decide whether to expand scope after one week of observation
- [ ] Periodically classify failed jobs by cause (network / image / compose error)

## Collaboration Loop (Weekly)

- [ ] Monday: update priorities and scope in `NEXT_STEPS.md` (product owner)
- [ ] Wednesday: update UI decision records (designer owner, with one-line dev feasibility note)
- [ ] Friday: refresh `HANDOVER.md` with implementation status, risks, and the next-session prompt (developer owner)
- [ ] At release time: update only user-impacting changes in `README.md` (API / operations / behavior changes)
