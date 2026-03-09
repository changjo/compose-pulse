# UI_DECISIONS.md

This log records weekly UI decisions made by the designer, product owner, and developer.

## Template

```
Date:
Owner:
Decision:
Reason:
Affected Screens:
Feasibility (Dev):
Follow-up:
```

## Entries

- Date: 2026-03-03
  Owner: Designer + Developer
  Decision: Keep the default dashboard focused on core actions only (registration, selected update, logs), and show operational/diagnostic information only when the `Advanced` toggle is enabled
  Reason: Reduce information density across both mobile and desktop, and lower the chance of mistakes for less experienced operators
  Affected Screens: `/` dashboard top bar, metrics/audit/DIUN/jobs/receipts sections
  Feasibility (Dev): Implemented, including localStorage-backed state persistence and pausing advanced polling while Advanced is off
  Follow-up: Further tune card density and readability for mobile when Advanced is on

- Date: 2026-03-03
  Owner: Designer + Developer
  Decision: Make SSE patch updates the default dashboard refresh mechanism instead of polling, while keeping pull-to-refresh as a mobile fallback
  Reason: Reduce idle network traffic while preserving an immediate manual refresh path for mobile users
  Affected Screens: `/` dashboard live updates, top pull indicator on mobile, Advanced Push panel
  Feasibility (Dev): Implemented, including stream reconnect backoff and 30-second fallback polling
  Follow-up: Measure real-world stream interruption frequency and fallback entry rate in production
