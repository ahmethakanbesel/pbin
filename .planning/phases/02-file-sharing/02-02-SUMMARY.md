---
phase: 02-file-sharing
plan: "02"
subsystem: database
tags: [sqlite, modernc, repository-pattern, tdd, domain-entity]

requires:
  - phase: 02-01
    provides: "File domain entity, Repository interface, FileService skeleton"
  - phase: 01-foundation
    provides: "DBPair (WriteDB/ReadDB), goose migrations, storage package scaffold"

provides:
  - "SQLite-backed fileRepo implementing all 5 methods of file.Repository"
  - "ExpiresAt *time.Time on File entity — populated by repository on reads"
  - "Read-time expiry enforcement in Service.Get() via ErrExpired"
  - "Compile-time interface check via var _ file.Repository = (*fileRepo)(nil)"

affects:
  - 02-file-sharing
  - 03-cleanup-worker
  - 04-handlers

tech-stack:
  added: []
  patterns:
    - "Two-pool SQLite (WriteDB for INSERT/UPDATE/DELETE, ReadDB for SELECT)"
    - "sql.NullInt64 for nullable INTEGER columns; unixepoch <-> time.Time conversion"
    - "Compile-time interface satisfaction check in concrete implementation file"
    - "Atomic MarkDownloaded via UPDATE WHERE downloaded_at IS NULL + RowsAffected (TOCTOU-safe)"
    - "nullableString helper converts empty string to nil for SQL NULL storage"
    - "TDD: failing tests committed before implementation (RED → GREEN)"

key-files:
  created:
    - internal/storage/file_repo.go
    - internal/storage/file_repo_test.go
  modified:
    - internal/domain/file/file.go
    - internal/domain/file/service.go

key-decisions:
  - "ExpiresAt is a display/service field set only by repository reads — not part of New() constructor invariants"
  - "Read-time expiry enforcement added to Service.Get() immediately after GetBySlug, before password check"
  - "ExpiresAt stored as unix epoch INTEGER in DB; converted to *time.Time in UTC on read"
  - "MarkDownloaded uses atomic UPDATE WHERE downloaded_at IS NULL to prevent TOCTOU race on one-use files"

patterns-established:
  - "Repository pattern: SQL queries isolated in storage package; domain package has zero knowledge of SQLite"
  - "Error mapping: sql.ErrNoRows always mapped to domain-layer ErrNotFound at repo boundary"
  - "Nullable columns: empty string <-> NULL via nullableString; bool <-> 0/1 via boolToInt"

requirements-completed: [FILE-01, FILE-02, FILE-03]

duration: 2min
completed: "2026-03-19"
---

# Phase 02 Plan 02: SQLite File Repository Summary

**SQLite fileRepo implementing all 5 methods of file.Repository with atomic one-use MarkDownloaded, read-time expiry enforcement, and ExpiresAt field on the File entity**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-19T21:09:58Z
- **Completed:** 2026-03-19T21:11:58Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Added `ExpiresAt *time.Time` to `file.File` struct and populated it from `expires_at` INTEGER column on reads
- Implemented `fileRepo` satisfying `file.Repository` with compile-time interface check; all 5 methods covered
- Added read-time expiry enforcement in `Service.Get()` — `ErrExpired` returned before password/one-use checks
- Followed TDD: failing tests committed first (RED), then implementation (GREEN); 15 tests all pass

## Task Commits

Each task was committed atomically:

1. **Task 1: Add ExpiresAt field to File entity** - `65695b7` (feat)
2. **Task 2: TDD RED — failing tests for SQLite file repository** - `0e9dfd4` (test)
3. **Task 2: TDD GREEN — implement SQLite file repository** - `eb8d8e7` (feat)

_Note: TDD task has two commits (test → feat)_

**Plan metadata:** _(pending docs commit)_

## Files Created/Modified

- `internal/storage/file_repo.go` — SQLite fileRepo with Create, GetBySlug, MarkDownloaded, Delete, ListExpired
- `internal/storage/file_repo_test.go` — 9 integration tests covering all repository methods
- `internal/domain/file/file.go` — Added `ExpiresAt *time.Time` field to File struct
- `internal/domain/file/service.go` — Added read-time expiry check in Get(); removed stale deferral comment

## Decisions Made

- `ExpiresAt` is NOT part of the `New()` constructor — it is a read-time field populated only by the repository. This keeps the domain constructor focused on write-time invariants.
- Read-time expiry check added before the password check in `Service.Get()` to fail fast on expired files regardless of whether a password is provided.
- `ExpiresAt` stored and retrieved in UTC to avoid timezone inconsistencies.
- `MarkDownloaded` uses `UPDATE WHERE downloaded_at IS NULL` with `RowsAffected` check — the atomic SQLite write serialization prevents TOCTOU races without explicit locking.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `fileRepo` is fully wired and ready for handlers (Phase 02-03 and beyond) to instantiate via `storage.NewFileRepo(db)`
- `Service.Get()` now enforces read-time expiry; cleanup worker (Phase 3) can delete expired rows without handlers serving stale files
- All 5 repository methods covered by integration tests — regression-safe for future schema changes

---
*Phase: 02-file-sharing*
*Completed: 2026-03-19*
