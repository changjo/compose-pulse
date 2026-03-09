# QA_REPORT_TEMPLATE.md

## Metadata

- Date:
- Tester:
- Environment:
  - Device / OS / Browser:
  - App version (image tag / commit):
  - Web Push enabled (`WEB_PUSH_ENABLED`):

## 1) Functional Tests (QA Engineer)

| ID | Scenario | Steps | Expected | Result (Pass/Fail) | Notes |
|---|---|---|---|---|---|
| F-01 | Login success | Enter the correct password on the login page | `/` opens and the dashboard is visible |  |  |
| F-02 | Login rate limit | Enter an invalid password repeatedly | `429` with `Retry-After` is returned |  |  |
| F-03 | Manual update | Select a target and run `Run Selected Update` | job is queued/running/successful and logs are visible |  |  |
| F-04 | Prune | Run `Dangling Image Prune` | prune job succeeds or fails with visible logs |  |  |
| F-05 | Dashboard SSE | Change a target or job in another tab | the current tab refreshes automatically |  |  |
| F-06 | Fallback polling | Block or disconnect SSE intentionally | the screen still refreshes via 30-second fallback polling |  |  |
| F-07 | DIUN webhook | Send a webhook with a valid secret | receipts/events are recorded, and a job is queued when auto-update conditions are met |  |  |
| F-08 | Push subscribe | Enable Push in Advanced > Push | `subscribed=true` and a test notification is received |  |  |
| F-09 | Push unsubscribe | Disable Push | `subscribed=false` and test notifications stop arriving |  |  |
| F-10 | Pull-to-refresh | Pull down from the top on mobile | exactly one refresh runs and duplicate refreshes are prevented |  |  |

## 2) UI Tests (QA Engineer)

| ID | Scenario | Expected | Result | Notes |
|---|---|---|---|---|
| U-01 | iPhone Safari main screen | Core actions fit within one screen and no horizontal overflow occurs |  |  |
| U-02 | Android Chrome main screen | Checkbox and button tap targets are comfortable and do not cause accidental taps |  |  |
| U-03 | Advanced OFF | Advanced panels are hidden and the main workflow remains simple |  |  |
| U-04 | Advanced ON | Metrics, Audit, DIUN, Receipts, Jobs, and Push panels are visible |  |  |
| U-05 | Jobs / Targets tables | Mobile card-row labels render correctly without severe truncation or awkward wrapping |  |  |

## 3) Usability Tests (Real User)

| ID | Scenario | Expected | Result | Notes |
|---|---|---|---|---|
| UX-01 | One-minute task completion | Login -> selected update -> log review can be completed within one minute |  |  |
| UX-02 | Mobile maintenance flow | Collapse/expand plus pull-to-refresh makes status re-checking practical on mobile |  |  |
| UX-03 | Error clarity | Failure messages are understandable enough to identify the likely cause |  |  |

## 4) Security Tests (Real User + QA)

| ID | Scenario | Expected | Result | Notes |
|---|---|---|---|---|
| S-01 | Unauthenticated API access | Protected `/api/*` session routes return `401` without a session |  |  |
| S-02 | Webhook secret mismatch | `/api/diun/webhook` returns `401` and records a `secret_mismatch` receipt |  |  |
| S-03 | Login brute-force | Rate limiting triggers and normal login works again after the lock window expires |  |  |
| S-04 | Path safety | Registration outside `/share/Container/<name>` is rejected |  |  |
| S-05 | Command safety | Execution remains limited to the approved command set |  |  |

## 5) Issue Log

| Priority | Title | Repro Steps | Expected | Actual | Owner | Status |
|---|---|---|---|---|---|---|
| P0/P1/P2 |  |  |  |  |  |  |

## 6) Conclusion

- Release decision: `Go` / `No-Go`
- Must-fix before release:
- Follow-up items for `NEXT_STEPS.md`:
