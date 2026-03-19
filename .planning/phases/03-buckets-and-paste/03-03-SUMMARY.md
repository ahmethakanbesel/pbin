---
phase: 03-buckets-and-paste
plan: "03"
subsystem: paste-domain
tags: [paste, service, repository, sqlite, bcrypt, one-use, delete-secret]
dependency_graph:
  requires: [03-01]
  provides: [paste.Service, paste.Repository, pasteRepo]
  affects: [03-05]
tech_stack:
  added: []
  patterns: [domain-service-pattern, two-pool-sqlite, nullable-string-helper, compile-time-interface-assertion]
key_files:
  created:
    - internal/domain/paste/service.go
    - internal/storage/paste_repo.go
  modified:
    - internal/domain/paste/paste.go
decisions:
  - "Paste.ExpiresAt is read-time field on Paste entity populated only by the repository (mirrors File pattern)"
  - "DeleteSecret set by service after New() constructor — constructor signature unchanged"
  - "ExpiryDuration called only after New() validates expiry preset — no double-validation needed"
metrics:
  duration: "103s"
  completed_date: "2026-03-19"
  tasks_completed: 3
  files_modified: 3
---

# Phase 03 Plan 03: Paste Domain Service and Repository Summary

PasteService with Create/Get/Delete business logic plus pasteRepo SQLite implementation storing content entirely in the database with bcrypt password hashing, atomic one-use MarkViewed, and constant-time delete secret comparison.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Extend Paste entity with DeleteSecret and ExpiresAt | 7feb19b | internal/domain/paste/paste.go |
| 2 | Implement PasteService | 582faaf | internal/domain/paste/service.go |
| 3 | Implement pasteRepo (SQLite) | 486306a | internal/storage/paste_repo.go |

## What Was Built

### Paste Entity Extension (paste.go)

Added two new fields to `Paste` struct:
- `DeleteSecret string` — secret token for authorised deletion
- `ExpiresAt *time.Time` — read-time field populated by the repository from unix epoch

Added 5 new sentinel errors alongside the existing 3:
- `ErrNotFound`, `ErrExpired`, `ErrAlreadyConsumed`, `ErrWrongPassword`, `ErrBadDeleteSecret`

### PasteService (service.go)

Business logic with three methods:
- `Create()` — generates slug + delete secret via `slug.New()`, bcrypts optional password (cost 12), constructs entity, sets `DeleteSecret` after construction, persists via repository
- `Get()` — enforces expiry before password check, enforces password, atomically marks one-use pastes via `MarkViewed()` + `RowsAffected` check
- `Delete()` — uses `subtle.ConstantTimeCompare` for timing-safe secret comparison

### pasteRepo (paste_repo.go)

SQLite implementation of `paste.Repository`:
- Compile-time assertion: `var _ paste.Repository = (*pasteRepo)(nil)`
- `Create` — uses `nullableString()` for password and delete_secret, `boolToInt()` for one_use, nullable int64 for expires_at
- `GetBySlug` — scans all fields including `ExpiresAt` from unix epoch (`sql.NullInt64` to `*time.Time`), returns `paste.ErrNotFound` on `sql.ErrNoRows`
- `MarkViewed` — atomic `UPDATE ... WHERE viewed_at IS NULL`, checks `RowsAffected` to detect already-consumed pastes
- `Delete` — simple `DELETE FROM pastes WHERE slug = ?`
- `ListExpired` — selects all rows with `expires_at < unixepoch()`, closes rows before returning

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

### Files Exist
- internal/domain/paste/paste.go — FOUND (modified)
- internal/domain/paste/service.go — FOUND (created)
- internal/storage/paste_repo.go — FOUND (created)

### Commits Exist
- 7feb19b — FOUND
- 582faaf — FOUND
- 486306a — FOUND

## Self-Check: PASSED
