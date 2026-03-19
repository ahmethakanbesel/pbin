---
phase: 04-api-web-ui-and-delivery
plan: "01"
subsystem: infra
tags: [middleware, auth, basic-auth, cleanup, worker, background, expiry]

requires:
  - phase: 01-foundation
    provides: config.AuthConfig struct fields (Enabled, Username, Password)
  - phase: 02-file-sharing
    provides: file.Repository.ListExpired, filestore.Backend.Delete
  - phase: 03-buckets-and-paste
    provides: bucket.Repository.ListExpired, paste.Repository.ListExpired

provides:
  - middleware.BasicAuth — timing-safe HTTP Basic Auth middleware gating write endpoints
  - worker.NewCleanup — background sweep worker deleting expired files, buckets, and pastes every 15 minutes

affects:
  - 04-03-wiring (main.go will import and wire both components)
  - 04-02 (handlers can proceed without auth logic concerns)

tech-stack:
  added: []
  patterns:
    - Zero-overhead pass-through when auth disabled (return next unchanged)
    - crypto/subtle.ConstantTimeCompare for timing-safe credential comparison
    - Local repository interfaces in worker package to avoid import cycles
    - Stop channel + done channel for clean goroutine shutdown
    - Panic recovery with 5s sleep and recursive runLoop restart

key-files:
  created:
    - internal/middleware/auth.go
    - internal/worker/cleanup.go
  modified: []

key-decisions:
  - "file.File uses Slug as on-disk storage key (no StorageKey field) — cleanup uses f.Slug for store.Delete"
  - "Worker defines its own repository interfaces locally to avoid coupling and import cycles"
  - "Stop() closes stop channel and blocks on done channel for synchronous shutdown guarantee"

patterns-established:
  - "Middleware pattern: disabled check returns next unchanged, enabled wraps in http.HandlerFunc"
  - "Cleanup worker: sweep-per-domain helper functions with per-item error logging and count return"

requirements-completed: [INFRA-03, INFRA-04]

duration: 2min
completed: 2026-03-19
---

# Phase 4 Plan 01: Basic Auth Middleware and Cleanup Worker Summary

**Timing-safe HTTP Basic Auth middleware and background expiry cleanup worker covering files, buckets, and pastes with panic recovery and clean shutdown**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-19T20:57:24Z
- **Completed:** 2026-03-19T20:58:57Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- `internal/middleware/auth.go`: BasicAuth middleware with zero-overhead disabled path, `crypto/subtle.ConstantTimeCompare` for both username and password, WWW-Authenticate header, and 401 JSON response
- `internal/worker/cleanup.go`: Cleanup worker with 15-minute sweep, three-domain ListExpired+Delete loop, on-disk storage key deletion, slog summary per sweep, panic recovery with restart, and Stop() channel handshake

## Task Commits

Each task was committed atomically:

1. **Task 1: Basic Auth middleware** - `abf01da` (feat)
2. **Task 2: Expiry cleanup worker** - `05bcf6b` (feat)

## Files Created/Modified

- `internal/middleware/auth.go` - BasicAuth middleware wrapping http.Handler with timing-safe credential comparison
- `internal/worker/cleanup.go` - Background cleanup worker with Start/Stop lifecycle and three-domain sweep

## Decisions Made

- `file.File` has no `StorageKey` field — the `Slug` is used directly as the on-disk key (confirmed from `file.Service.Delete` which passes `shareSlug` to `store.Delete`). Cleanup uses `f.Slug` for `store.Delete`.
- Worker defines local repository interfaces (`FileRepository`, `BucketRepository`, `PasteRepository`) to avoid coupling to domain packages beyond entity types and to keep the worker independently testable.
- `Stop()` closes the stop channel then blocks on a separate `done` channel that `runLoop` closes on return — provides a synchronous shutdown guarantee.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed incorrect StorageKey field reference on file.File**
- **Found during:** Task 2 (Expiry cleanup worker)
- **Issue:** Plan specified `f.StorageKey` in the sweep logic but `file.File` has no `StorageKey` field — it uses `Slug` as the disk key
- **Fix:** Changed `store.Delete(ctx, f.StorageKey)` to `store.Delete(ctx, f.Slug)` after confirming the pattern in `file.Service.Delete`
- **Files modified:** internal/worker/cleanup.go
- **Verification:** `go build ./internal/worker/...` exits 0
- **Committed in:** 05bcf6b (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug)
**Impact on plan:** Auto-fix required for correctness — file storage keys are slugs, not a separate field. No scope creep.

## Issues Encountered

- `file.File` entity does not have a `StorageKey` field (unlike `BucketFile.StorageKey`). The plan's interface specification mentioned it as a possibility ("verify exact field names"). Confirmed by reading `file.go` and `file.Service.Delete` — `Slug` is the storage key for single files.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `middleware.BasicAuth` is ready to wrap write handlers in `main.go` (plan 04-03)
- `worker.NewCleanup` is ready to be constructed and started in `main.go` (plan 04-03)
- Both packages compile cleanly in isolation; `go build ./...` exits 0

---
*Phase: 04-api-web-ui-and-delivery*
*Completed: 2026-03-19*

## Self-Check: PASSED

- internal/middleware/auth.go: FOUND
- internal/worker/cleanup.go: FOUND
- 04-01-SUMMARY.md: FOUND
- Commit abf01da: FOUND
- Commit 05bcf6b: FOUND
