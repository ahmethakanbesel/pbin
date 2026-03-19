---
phase: 01-foundation
plan: "03"
subsystem: domain
tags: [go, ddd, domain-entities, repository-pattern, health-endpoint, binary-entrypoint]

requires:
  - phase: 01-01
    provides: config.Parse, slug.New — used by main.go for startup wiring
  - phase: 01-02
    provides: storage.Open, filestore.NewLocal — wired in main.go

provides:
  - "file.File entity + file.Repository interface"
  - "bucket.Bucket + BucketFile entities + bucket.Repository interface"
  - "paste.Paste entity + paste.Repository interface"
  - "handler.Health — GET /health returns {status:ok}"
  - "cmd/pbin/main.go — single CGO-free binary wiring all components"

affects: [02-file-upload, 02-bucket-transfer, 02-paste, 03-cleanup, 04-frontend]

tech-stack:
  added: []
  patterns:
    - "Repository interface in domain package (not infrastructure) — file, bucket, paste each own their Repository"
    - "Constructor validation — New() returns typed error for invalid expiry preset or empty required field"
    - "ExpiryDuration helper — converts preset string to time.Duration; panics if called with unvalidated input"
    - "main.go as pure wiring — no business logic, only dependency injection and server lifecycle"
    - "Graceful shutdown — SIGTERM/SIGINT triggers srv.Shutdown with 10s context timeout"

key-files:
  created:
    - internal/domain/file/file.go
    - internal/domain/file/repository.go
    - internal/domain/file/file_test.go
    - internal/domain/bucket/bucket.go
    - internal/domain/bucket/repository.go
    - internal/domain/paste/paste.go
    - internal/domain/paste/repository.go
    - internal/domain/paste/paste_test.go
    - internal/handler/health.go
    - cmd/pbin/main.go
  modified: []

key-decisions:
  - "Repository interfaces live in domain packages (not infrastructure) — file.Repository not storage.FileRepository"
  - "Expiry preset validation in constructor — only 9 allowed values (10m, 1h, 6h, 1d, 7d, 30d, 90d, 1y, never)"
  - "ExpiryDuration panics on invalid input because callers must validate via New() before calling it"
  - "No bucket test file — structurally identical to file entity, no additional edge cases"

patterns-established:
  - "Pattern 1: Domain package owns its Repository interface — prevents import cycles and keeps infra details out of domain"
  - "Pattern 2: Constructor returns (Entity, error) — validates all invariants, never returns partially valid entity"
  - "Pattern 3: main.go is wire-only — config -> DB -> filestore -> handlers -> server, no logic"

requirements-completed: [INFRA-06]

duration: 2min
completed: 2026-03-19
---

# Phase 1 Plan 03: Domain Entities, Repository Interfaces, and Binary Entrypoint Summary

**Three domain entity packages (file, bucket, paste) with constructor validation and repository interfaces, health handler, and CGO-free binary entrypoint that wires config/DB/filestore and serves GET /health**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-19T17:15:46Z
- **Completed:** 2026-03-19T17:17:56Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments

- Domain entities with constructor validation: file.New, bucket.New, paste.New reject invalid expiry presets and empty required fields
- Repository interfaces defined in each domain package (not infrastructure) following DDD pattern
- Single CGO-free binary compiled via `CGO_ENABLED=0 go build ./cmd/pbin` — no external runtime dependencies
- Binary starts, applies goose migrations automatically, logs startup messages, serves GET /health returning 200 with `{"status":"ok"}`

## Task Commits

Each task was committed atomically:

1. **TDD RED: Failing tests for file + paste entities** - `409b918` (test)
2. **Task 1: Domain entities and repository interfaces** - `f1c1039` (feat)
3. **Task 2: Health endpoint and main.go binary entrypoint** - `2b3cf0f` (feat)

_Note: TDD tasks have RED (test) commit followed by GREEN (feat) commit_

## Files Created/Modified

- `internal/domain/file/file.go` - File entity, New constructor with expiry/slug validation, ExpiryDuration helper
- `internal/domain/file/repository.go` - Repository interface: Create, GetBySlug, MarkDownloaded, Delete, ListExpired
- `internal/domain/file/file_test.go` - Tests for New valid/invalid expiry, empty slug, ExpiryDuration presets
- `internal/domain/bucket/bucket.go` - Bucket + BucketFile entities, New constructor, ExpiryDuration helper
- `internal/domain/bucket/repository.go` - Repository interface: Create, AddFile, GetBySlug, MarkDownloaded, Delete, ListExpired
- `internal/domain/paste/paste.go` - Paste entity, New constructor with content/slug/expiry validation, default lang="text"
- `internal/domain/paste/repository.go` - Repository interface: Create, GetBySlug, MarkViewed, Delete, ListExpired
- `internal/domain/paste/paste_test.go` - Tests for New valid/invalid expiry, empty content, empty slug, default lang
- `internal/handler/health.go` - Health handler: sets Content-Type and writes {"status":"ok"}
- `cmd/pbin/main.go` - Binary entrypoint: flag parsing, slog setup, config load, dir creation, DB open, filestore init, ServeMux, graceful shutdown

## Decisions Made

- Repository interfaces placed in domain packages (not infrastructure) to prevent import cycles and keep domain self-contained
- Expiry validation uses a fixed preset map — only 9 valid values, making invalid input an explicit error rather than accepting arbitrary durations
- ExpiryDuration panics on invalid input because any caller must have already validated via New(); a panic here indicates a programming error
- Bucket entity has no separate test file — structurally identical validation logic to File, covered by the File tests pattern

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

Port 8080 was already in use during smoke test — used `PBIN_SERVER_PORT=18080` env override to verify binary on alternate port. Health endpoint returned `{"status":"ok"}` as expected.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 1 foundation is complete: binary compiles CGO-free, applies migrations, loads config with env overrides, serves health endpoint
- All Phase 2 domain feature work can proceed against the established repository interface pattern
- File upload handler imports `file.Repository` interface; paste handler imports `paste.Repository` interface
- No blockers for Phase 2

---
*Phase: 01-foundation*
*Completed: 2026-03-19*

## Self-Check: PASSED

- All 10 created files verified on disk
- All 3 task commits (409b918, f1c1039, 2b3cf0f) verified in git log
