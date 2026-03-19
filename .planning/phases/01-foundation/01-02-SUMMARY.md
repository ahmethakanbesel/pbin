---
phase: 01-foundation
plan: 02
subsystem: database
tags: [go, sqlite, modernc, goose, migrations, filestore, wal, two-pool]

# Dependency graph
requires:
  - phase: 01-01
    provides: "go.mod with modernc.org/sqlite and pressly/goose pinned; Config struct with DBPath and StoragePath"
provides:
  - "storage.Open() returning DBPair with WAL + two-pool + embedded goose migrations"
  - "internal/storage/migrations/001_init.sql: files, buckets, bucket_files, pastes schema"
  - "filestore.Backend interface: Write, Read, Delete, Exists"
  - "filestore.NewLocal() — LocalFS with slug-keyed atomic writes and path traversal prevention"
affects: [03-core-api, 04-frontend]

# Tech tracking
tech-stack:
  added:
    - "modernc.org/sqlite v1.47.0 promoted to direct dependency"
    - "pressly/goose/v3 v3.27.0 promoted to direct dependency"
  patterns:
    - "Two-pool SQLite: WriteDB.SetMaxOpenConns(1), ReadDB unbounded"
    - "DSN pragma syntax for modernc: _pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
    - "goose.SetBaseFS(embed.FS) + goose.Up at startup before HTTP server binds"
    - "Atomic file write: os.CreateTemp in target dir + os.Rename"
    - "Key validation: validKey = regexp.MustCompile(`^[a-zA-Z0-9]+$`)"
    - "Two-char subdirectory prefix for inode distribution: {root}/{key[:2]}/{key}"
    - "TDD: failing test commit, then implementation commit per task"

key-files:
  created:
    - "internal/storage/db.go"
    - "internal/storage/migrations.go"
    - "internal/storage/migrations/001_init.sql"
    - "internal/storage/db_test.go"
    - "internal/filestore/store.go"
    - "internal/filestore/local.go"
    - "internal/filestore/local_test.go"
  modified:
    - "go.mod"
    - "go.sum"

key-decisions:
  - "Two-pool architecture (WriteDB capped at 1, ReadDB uncapped) to prevent SQLITE_BUSY under concurrent load"
  - "modernc.org/sqlite DSN pragma syntax differs from mattn: use _pragma=key(value) not ?_journal_mode=WAL"
  - "goose.SetBaseFS must be called before goose.Up; migrationFS embedded in migrations.go (separate file from db.go)"
  - "LocalFS keys must match ^[a-zA-Z0-9]+$ — rejects path separators, null bytes, spaces, all traversal attempts"
  - "Atomic write via os.CreateTemp + os.Rename prevents partial writes being visible to readers"

patterns-established:
  - "DBPair.Close(): close WriteDB first, return first non-nil error"
  - "storage.Open: empty path returns error immediately (no panic)"
  - "LocalFS.Delete: returns nil for non-existent keys (idempotent)"
  - "LocalFS.Read: wraps os.ErrNotExist directly for errors.Is compatibility"

requirements-completed:
  - INFRA-06

# Metrics
duration: 8min
completed: 2026-03-19
---

# Phase 1 Plan 2: SQLite and Storage Infrastructure Summary

**SQLite with WAL mode + two-pool connection management + embedded goose migrations, and slug-keyed LocalFS backend with atomic writes and path-traversal prevention**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-03-19T17:10:19Z
- **Completed:** 2026-03-19T17:18:30Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments

- SQLite Open() with WAL journal mode, 5s busy timeout, foreign keys, and NORMAL synchronous — all via modernc.org/sqlite DSN pragmas
- Two-pool architecture: WriteDB capped at 1 connection, ReadDB unbounded — prevents SQLITE_BUSY under concurrent writes
- Embedded goose migrations run automatically at startup; 001_init.sql creates four domain tables with expires_at indexes
- LocalFS Backend with two-char subdirectory prefix layout, atomic writes via temp+rename, and alphanumeric-only key validation blocking path traversal

## Task Commits

Each task was committed atomically:

1. **Task 1: SQLite database - test (RED)** - `56f6994` (test)
2. **Task 1: SQLite database - implementation (GREEN)** - `32015c1` (feat)
3. **Task 2: LocalFS - test (RED)** - `31135e0` (test)
4. **Task 2: LocalFS - implementation (GREEN)** - `7eb77b4` (feat)

_Note: TDD tasks have separate test (RED) and implementation (GREEN) commits per task._

## Files Created/Modified

- `internal/storage/db.go` - DBPair struct, Open() with WAL + two-pool + goose migrations
- `internal/storage/migrations.go` - embed.FS declaration for migration SQL files
- `internal/storage/migrations/001_init.sql` - files, buckets, bucket_files, pastes tables + expires_at indexes
- `internal/storage/db_test.go` - 6 tests: empty path, non-nil pair, WAL mode, max conns, goose table, all 4 tables
- `internal/filestore/store.go` - Backend interface: Write, Read, Delete, Exists
- `internal/filestore/local.go` - LocalFS: NewLocal, keyPath, Write (atomic), Read, Delete, Exists
- `internal/filestore/local_test.go` - 7 tests: root creation, round-trip, not-exist, delete, idempotent delete, exists lifecycle, path traversal
- `go.mod` - modernc.org/sqlite and pressly/goose promoted from indirect to direct
- `go.sum` - updated checksums

## Decisions Made

- Two-pool architecture chosen over single pool: prevents the SQLITE_BUSY problem documented in PITFALLS.md Pitfall 1 — WriteDB serializes all writes while ReadDB supports concurrent reads in WAL mode
- modernc.org/sqlite DSN pragma format is `_pragma=key(value)` not `?_journal_mode=WAL` — verified with TestOpen_WALJournalMode asserting actual PRAGMA query result equals "wal"
- goose.SetBaseFS must be set globally before goose.Up — placed in Open() so each test run is self-contained
- LocalFS key regex `^[a-zA-Z0-9]+$` is intentionally strict: rejects `../`, `/`, null bytes, spaces — matches exactly the output of internal/slug

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/storage` package is ready for use by any handler that needs DB access
- `internal/filestore` package is ready for use in upload handlers
- Both packages are independently testable with t.TempDir() — no external DB or filesystem setup needed
- CGO_ENABLED=0 go build ./... exits 0 — single-binary cross-compile promise maintained

---
*Phase: 01-foundation*
*Completed: 2026-03-19*
