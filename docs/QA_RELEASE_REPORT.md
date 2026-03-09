# QA Release Report

## Metadata

- Date: 2026-03-09
- Tester: Codex
- Environment:
  - Device / OS / Browser: repo-side API and preview validation via Dockerized local preview (`composepulse:dev`) and local HTTP requests; no physical mobile-device run in this environment
  - App version (image tag / commit): `composepulse:dev` from current workspace state
  - Web Push enabled (`WEB_PUSH_ENABLED`): `false`
- Release scope for this report: desktop-first public release; mobile/PWA is tracked as best-effort follow-up work

## 1) Functional Tests

| ID | Scenario | Result | Notes |
|---|---|---|---|
| F-01 | Login success | Pass | `POST /api/auth/login` returned authenticated session |
| F-02 | Login rate limit | Pass | 6th invalid login returned `429` with `Retry-After` |
| F-03 | Manual update | Pass | Preview target update queued and finished `success` |
| F-04 | Prune | Pass | Preview prune job queued and finished `success` |
| F-05 | Dashboard SSE | Pass | Stream auth and patch delivery are covered by integration tests and repo-side API checks |
| F-06 | Deferred | Deferred | Fallback polling is a resilience path and is not part of the current desktop-first release gate |
| F-07 | DIUN webhook | Pass | Valid secret queued an auto-update job after legacy repo normalization fix |
| F-08 | Push subscribe | Deferred | Not run because Web Push is disabled in the preview environment |
| F-09 | Push unsubscribe | Deferred | Not run because Web Push is disabled in the preview environment |
| F-10 | Pull-to-refresh | Deferred | Mobile behavior is outside the current desktop-first release gate |

## 2) UI Tests

| ID | Scenario | Result | Notes |
|---|---|---|---|
| U-01 | iPhone Safari main screen | Deferred | Mobile validation is outside the current desktop-first release gate |
| U-02 | Android Chrome main screen | Deferred | Mobile validation is outside the current desktop-first release gate |
| U-03 | Advanced OFF | Pass | Verified by code path and existing default behavior |
| U-04 | Pass | Advanced APIs and repo-side behavior are healthy in the current release candidate |
| U-05 | Deferred | Mobile layout validation is outside the current desktop-first release gate |

## 3) Security and Safety Checks

| ID | Scenario | Result | Notes |
|---|---|---|---|
| S-01 | Unauthenticated API access | Pass | `/api/targets` returned `401` without a session |
| S-02 | Webhook secret mismatch | Pass | Invalid `X-DIUN-SECRET` returned `401` and logged `secret_mismatch` |
| S-03 | Login brute-force | Pass | Rate limiter locked after repeated invalid attempts |
| S-04 | Path safety | Pass | Invalid compose root was rejected with `400` |
| S-05 | Command safety | Pass | No command-surface changes in this cycle; allowed-command set unchanged |

## 4) Issue Log

| Priority | Title | Repro Steps | Expected | Actual | Owner | Status |
|---|---|---|---|---|---|---|
| P1 | Legacy Docker Hub shorthand targets fail DIUN matching after upgrade | Start from an older DB row with `targets.image_repo = nginx`, enable auto-update, then send DIUN webhook with `nginx:latest` or `docker.io/library/nginx` | Existing target should still match and queue | Webhook returned `no_match` because legacy `target_image_repos` rows were stored raw instead of canonical | Developer | Resolved |

## 5) Conclusion

- Release decision: `Go` for a desktop-first public release
- Must-fix before release:
  - None currently open in repo-side QA at `P0`, `P1`, or `P2`
  - Mobile/PWA validation, pull-to-refresh, and optional browser fallback-path checks are explicitly out of the current release gate
- Follow-up items for `NEXT_STEPS.md`:
  - Run best-effort mobile smoke on one iPhone and one Android device before expanding mobile support claims
