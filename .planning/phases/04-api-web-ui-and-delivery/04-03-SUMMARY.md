---
phase: 04-api-web-ui-and-delivery
plan: "03"
subsystem: infra
tags: [go, http, middleware, basicauth, cleanup-worker, goroutine, graceful-shutdown]

# Dependency graph
requires:
  - phase: 04-01
    provides: middleware.BasicAuth and worker.NewCleanup packages
  - phase: 04-02
    provides: handler.NewUIHandler with Home, Paste, Bucket methods

provides:
  - "cmd/pbin/main.go wired with BasicAuth on all write and upload-form endpoints"
  - "Cleanup worker goroutine started at startup, stopped on SIGTERM/SIGINT"
  - "UI form routes registered: GET / (Home), GET /paste (Paste), GET /bucket (Bucket)"
  - "Complete, shippable pbin binary"

affects:
  - "04-api-web-ui-and-delivery (final wiring, phase complete)"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "mux.Handle (not mux.HandleFunc) for routes wrapped in middleware.BasicAuth"
    - "middleware.BasicAuth(...).ServeHTTP(w, r) for inline middleware in catch-all switch cases"
    - "cleanupWorker.Stop() called before srv.Shutdown in graceful shutdown sequence"

key-files:
  created: []
  modified:
    - cmd/pbin/main.go

key-decisions:
  - "Auth gates form pages in addition to API write endpoints — GET /, GET /paste, GET /bucket all wrapped in middleware.BasicAuth per CONTEXT.md spec"
  - "mux.Handle used instead of mux.HandleFunc for middleware-wrapped routes (BasicAuth returns http.Handler, not a func)"
  - "cleanupWorker.Stop() placed before srv.Shutdown to allow in-flight sweep completion before connection drain"

patterns-established:
  - "Inline catch-all middleware: use .ServeHTTP(w, r) pattern when wrapping a single case in a switch inside mux.HandleFunc"

requirements-completed:
  - INFRA-01
  - INFRA-03
  - INFRA-04
  - INFRA-05

# Metrics
duration: 2min
completed: 2026-03-20
---

# Phase 4 Plan 03: Final Wiring Summary

**Wired BasicAuth middleware on all write and form endpoints, cleanup worker with graceful stop, and UI form routes into cmd/pbin/main.go — pbin is now complete**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-20T00:02:13Z
- **Completed:** 2026-03-20T00:03:55Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Applied `middleware.BasicAuth` to POST /api/upload, POST /api/paste, GET /, GET /paste, GET /bucket (5 wrapping sites)
- Started `worker.NewCleanup(fileRepo, bucketRepo, pasteRepo, fs, 15*time.Minute)` at process startup
- Added `cleanupWorker.Stop()` to graceful shutdown sequence before `srv.Shutdown`
- Registered UIHandler form routes: Home at GET / catch-all root, Paste at GET /paste, Bucket at GET /bucket
- Binary builds cleanly; all smoke test endpoints return expected status codes

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire middleware, cleanup worker, and UI routes into main.go** - `71b02d5` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified

- `cmd/pbin/main.go` - Added middleware/worker imports, uiHandler+cleanupWorker construction, BasicAuth wrapping on 5 endpoints, cleanupWorker.Stop() in shutdown block

## Decisions Made

- Used `mux.Handle` (not `mux.HandleFunc`) for all middleware-wrapped routes since `middleware.BasicAuth` returns `http.Handler`
- In the GET / catch-all switch, used `.ServeHTTP(w, r)` inline pattern for the `n == 0` branch since only that case needs auth (the catch-all func signature is fixed)
- Placed `cleanupWorker.Stop()` before `srv.Shutdown` so the worker can finish any in-flight sweep before the server stops accepting connections

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Port 8080 was occupied by a prior binary during initial smoke test run; killed the old process and re-ran — all endpoints responded correctly on second test.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All four plans of Phase 4 complete; pbin v1.0 milestone is shippable.
- Binary starts, migrates DB, serves form UI pages, accepts file/bucket/paste uploads, serves content, and cleans up expired records automatically.
- No blockers.

---
*Phase: 04-api-web-ui-and-delivery*
*Completed: 2026-03-20*
