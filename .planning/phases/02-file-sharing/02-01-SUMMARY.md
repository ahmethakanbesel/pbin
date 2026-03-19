---
phase: 02-file-sharing
plan: "01"
subsystem: domain
tags: [go, sqlite, bcrypt, filestore, slug, migration, goose]

requires:
  - phase: 01-foundation
    provides: "File entity skeleton, Repository interface, filestore.Backend interface, slug generator, goose migration infrastructure"

provides:
  - "Goose migration 002 adding delete_secret column to files table"
  - "Extended File entity with DeleteSecret field and IsImage/SupportedImageMIMETypes"
  - "FileService with Upload, Get (one-use atomicity), and Delete (timing-safe secret check)"
  - "Five exported sentinel errors for HTTP handler mapping"

affects:
  - 02-file-sharing
  - 03-transfer-sharing
  - 04-paste-sharing

tech-stack:
  added: []
  patterns:
    - "Service struct holds repo + filestore.Backend + baseURL — all business rules in domain, handlers are thin wrappers"
    - "Sentinel errors pattern: domain errors mapped to HTTP codes at handler boundary"
    - "Bytes-first upload: write to filestore, then DB insert; rollback store on DB failure"
    - "One-use atomicity via MarkDownloaded repository call — TOCTOU prevention at DB layer"
    - "Timing-safe delete secret verification via crypto/subtle.ConstantTimeCompare"

key-files:
  created:
    - internal/storage/migrations/002_add_delete_secret.sql
    - internal/domain/file/service.go
  modified:
    - internal/domain/file/file.go

key-decisions:
  - "Bytes-first upload order: write to filestore before DB insert; best-effort store.Delete on DB failure"
  - "Read-time expiry enforcement deferred to Plan 02 when SQLite repository populates ExpiresAt on File entity"
  - "No SVG in SupportedImageMIMETypes — XSS risk (SVG can contain script tags)"
  - "Delete order: store.Delete before repo.Delete — if store fails, DB row stays for operator recovery"

patterns-established:
  - "Service pattern: domain service holds repo + backend, no HTTP knowledge"
  - "Sentinel errors: all domain errors are exported vars, handlers do errors.Is() mapping"

requirements-completed: [FILE-01, FILE-03, FILE-04, FILE-05, FILE-06]

duration: 3min
completed: 2026-03-19
---

# Phase 2 Plan 01: File Domain Service Summary

**SQLite migration for delete_secret, File entity with image detection, and FileService with bcrypt passwords, one-use atomicity, and timing-safe delete secret verification**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-19T18:04:43Z
- **Completed:** 2026-03-19T18:06:32Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- Goose migration 002 adds `delete_secret TEXT` column to files table with intentional no-op Down (SQLite DDL limitation)
- File entity extended with `DeleteSecret` field, `IsImage()` function, and `SupportedImageMIMETypes` exported var (5 MIME types, no SVG)
- FileService implements Upload (bcrypt cost 12, slug + delete secret generation), Get (atomic one-use via MarkDownloaded), and Delete (timing-safe subtle.ConstantTimeCompare)

## Task Commits

Each task was committed atomically:

1. **Task 1: Goose migration 002 — add delete_secret column** - `4c8246f` (chore)
2. **Task 2: Extend File entity with DeleteSecret and image detection** - `cffc32a` (feat)
3. **Task 3: Implement FileService business logic** - `6b54419` (feat)

## Files Created/Modified

- `internal/storage/migrations/002_add_delete_secret.sql` - Goose migration adding delete_secret TEXT column
- `internal/domain/file/file.go` - Added DeleteSecret field, New() parameter, IsImage(), SupportedImageMIMETypes
- `internal/domain/file/service.go` - FileService with Upload/Get/Delete and 5 sentinel errors

## Decisions Made

- **Bytes-first upload order:** filestore.Write before repo.Create so we can delete the bytes on DB failure (best-effort cleanup)
- **Read-time expiry deferred:** File entity doesn't carry ExpiresAt yet — that field will be populated by the SQLite repo in Plan 02; expiry enforcement at read-time is deferred
- **No SVG support:** SupportedImageMIMETypes excludes SVG due to XSS risk (script tag injection)
- **Delete order:** store.Delete before repo.Delete — if store.Delete fails, DB row survives for operator inspection

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- FileService is complete and ready for Plan 02 (SQLite repository implementation)
- Repository interface remains unchanged — Plan 02 implements it in `internal/storage`
- `ErrNotFound` sentinel is defined in service.go; repo must wrap its not-found condition with this error for `errors.Is()` to work at the handler boundary

---
*Phase: 02-file-sharing*
*Completed: 2026-03-19*
