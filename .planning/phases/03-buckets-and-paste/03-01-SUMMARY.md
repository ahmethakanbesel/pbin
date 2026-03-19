---
phase: 03-buckets-and-paste
plan: 01
subsystem: database
tags: [sqlite, goose, migrations, schema]

requires:
  - phase: 01-foundation
    provides: goose embedded migration runner and SQLite schema bootstrapped with 001_init.sql and 002_add_delete_secret.sql
provides:
  - Migration 003 that adds delete_secret TEXT column to buckets table
  - Migration 003 that adds delete_secret TEXT column to pastes table
affects:
  - 03-02-buckets-domain
  - 03-03-paste-domain
  - 03-04-buckets-handler
  - 03-05-paste-handler

tech-stack:
  added: []
  patterns:
    - "Goose per-statement StatementBegin/End blocks — each SQL statement wrapped separately"
    - "Intentionally irreversible SQLite down migration using SELECT 1 placeholder"

key-files:
  created:
    - internal/storage/migrations/003_add_delete_secrets.sql
  modified: []

key-decisions:
  - "Each ALTER TABLE statement wrapped in its own StatementBegin/End block (goose requires one statement per block)"
  - "Down migration is intentionally irreversible (SELECT 1) — mirrors pattern from 002"

patterns-established:
  - "Migration file naming: NNN_verb_noun.sql with sequential numbering"
  - "Nullable TEXT column for delete_secret — no NOT NULL constraint, matches files table pattern"

requirements-completed: [BUCK-01, BUCK-04, BUCK-05, PASTE-01, PASTE-04, PASTE-05]

duration: 4min
completed: 2026-03-19
---

# Phase 3 Plan 1: Migration 003 — delete_secret columns for buckets and pastes

**Goose migration 003 adding nullable TEXT delete_secret column to both buckets and pastes tables, enabling BucketService and PasteService to persist user-facing delete tokens.**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-19T20:14:24Z
- **Completed:** 2026-03-19T20:18:00Z
- **Tasks:** 1 of 1
- **Files modified:** 1

## Accomplishments

- Created `internal/storage/migrations/003_add_delete_secrets.sql` following exact goose pattern from migration 002
- Both `ALTER TABLE buckets ADD COLUMN delete_secret TEXT` and `ALTER TABLE pastes ADD COLUMN delete_secret TEXT` in separate StatementBegin/End blocks
- Project compiles without errors (`go build ./...` exits 0)

## Task Commits

Each task was committed atomically:

1. **Task 1: Migration 003 — add delete_secret to buckets and pastes** - `7f2e5dd` (feat)

**Plan metadata:** (see final docs commit)

## Files Created/Modified

- `internal/storage/migrations/003_add_delete_secrets.sql` - Goose migration adding delete_secret TEXT column to buckets and pastes tables; auto-runs via embedded FS at startup to advance goose_db_version to 3

## Decisions Made

- Each ALTER TABLE statement uses its own `-- +goose StatementBegin` / `-- +goose StatementEnd` wrapper. Goose requires a single SQL statement per block; combining both into one block would fail parsing.
- Down migration uses `SELECT 1` placeholder matching the irreversibility pattern from migration 002 (SQLite older versions do not support DROP COLUMN).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Migration 003 is embedded via the existing `embed.FS` directive in `internal/storage/db.go` and will auto-run at startup
- `buckets.delete_secret` and `pastes.delete_secret` columns are ready for use by BucketRepository and PasteRepository `Create()` methods in plans 03-02 and 03-03
- No blockers for the next plan

---
*Phase: 03-buckets-and-paste*
*Completed: 2026-03-19*
