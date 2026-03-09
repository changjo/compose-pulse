# AGENTS.md

This repository hosts the `compose-pulse` web app for self-hosted NAS operations.

## Mission

- Provide a secure, minimal web UI for:
  - manual selected container update (`docker compose pull && docker compose up -d`)
  - DIUN webhook-driven selective auto update
  - compose variable-aware image discovery and target multi-image mapping
  - dangling image prune (`docker image prune -f`)
- Never modify arbitrary NAS files.

## Hard Safety Rules

- Only allow compose directories under `/share/Container/<name>` (one level only).
- Never write persistent app data outside `/data` volume.
- Never execute shell-composed command strings.
- Allowed runtime commands only:
  - `docker compose -f <compose_file> pull`
  - `docker compose -f <compose_file> up -d`
  - `docker image prune -f`

## Security Model

- Edge protection: Caddy BasicAuth (outside app).
- App protection: login session cookie for dashboard APIs (`/api/auth/login`).
- Login hardening: IP+username rate limit (`429`, `Retry-After`).
- DIUN webhook protection: `X-DIUN-SECRET` check (auto-generated if env is empty).

## Architecture Snapshot

- Language: Go
- Runtime: single binary
- DB: SQLite (`/data/app.db`)
- Worker: in-process, global sequential queue (1 worker)
- Log streaming: SSE (`/api/jobs/{id}/stream`)
- Dashboard patch streaming: SSE (`/api/stream/dashboard`)

## API Ownership

- `GET /api/health`
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`
- `GET /api/containers/discover` (session)
- `GET /api/metrics` (session)
- `GET /api/targets` (session)
- `POST /api/targets` (session)
- `PATCH /api/targets/{id}` (session)
- `DELETE /api/targets/{id}` (session)
- `POST /api/jobs/update` (session)
- `POST /api/jobs/prune` (session)
- `POST /api/jobs/delete-all` (session)
- `GET /api/jobs` (`limit` + `cursor`) (session)
- `DELETE /api/jobs/{id}` (session)
- `GET /api/jobs/export.csv` (`limit`) (session)
- `GET /api/jobs/{id}/stream` (session)
- `GET /api/stream/dashboard` (session)
- `GET /api/audit/targets` (session)
- `GET /api/diun/events` (session)
- `GET /api/diun/receipts` (session)
- `GET /api/diun/webhook/config` (session)
- `GET /api/push/config` (session)
- `POST /api/push/subscriptions` (session)
- `DELETE /api/push/subscriptions` (session)
- `POST /api/push/test` (session)
- `POST /api/diun/webhook`

## Development Notes

- Local machine may not have `go` binary; use Dockerized Go toolchain for test/format when needed.
- Verified test command:
  - `docker run --rm -v "$PWD":/src -w /src golang:1.22-alpine sh -lc 'apk add --no-cache build-base >/dev/null && /usr/local/go/bin/gofmt -w main.go main_test.go integration_test.go diun_fixture_test.go && CGO_ENABLED=1 /usr/local/go/bin/go test ./...'`

## Definition of Done (for next changes)

- Tests pass.
- No safety rule regression.
- Documentation updated (`README.md`, `HANDOVER.md`).
