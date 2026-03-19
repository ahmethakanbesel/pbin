---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: unknown
stopped_at: Completed 03-04-PLAN.md
last_updated: "2026-03-19T20:24:42.020Z"
progress:
  total_phases: 4
  completed_phases: 2
  total_plans: 13
  completed_plans: 12
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-19)

**Core value:** Users can share files, transfer file bundles, and paste text through a single self-hosted service that runs from one binary with zero external dependencies.
**Current focus:** Phase 03 — buckets-and-paste

## Current Position

Phase: 03 (buckets-and-paste) — EXECUTING
Plan: 1 of 6

## Performance Metrics

**Velocity:**

- Total plans completed: 0
- Average duration: -
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**

- Last 5 plans: none yet
- Trend: -

*Updated after each plan completion*
| Phase 01-foundation P01 | 12 | 3 tasks | 6 files |
| Phase 02-file-sharing P01 | 3 | 3 tasks | 3 files |
| Phase 02-file-sharing P03 | 12 | 3 tasks | 2 files |
| Phase 02-file-sharing P04 | 13min | 2 tasks | 2 files |
| Phase 03-buckets-and-paste P01 | 1min | 1 tasks | 1 files |
| Phase 03-buckets-and-paste P03 | 103s | 3 tasks | 3 files |
| Phase 03-buckets-and-paste P02 | 2 | 3 tasks | 3 files |
| Phase 03-buckets-and-paste P05 | 2min | 1 tasks | 1 files |
| Phase 03-buckets-and-paste P04 | 2min | 1 tasks | 2 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Roadmap]: SQLite via `modernc.org/sqlite` (CGO-free, WAL mode, two-pool setup required from first commit)
- [Roadmap]: `pressly/goose` for embedded schema migrations — must be in place before any handler code
- [Roadmap]: `knadh/koanf/v2` for config (TOML + env var overrides)
- [Roadmap]: Go 1.22+ `net/http.ServeMux` only — no web framework
- [Roadmap]: `Content-Disposition: attachment` forced on all served files (XSS prevention — cannot be retrofitted)
- [Phase 01-foundation]: Config defaults via struct literal; koanf layered on top with Exists() guards (no Static provider exists)
- [Phase 01-foundation]: Slug charset excludes ambiguous chars (0/O/1/l/I) for human readability
- [Phase 01-foundation]: Env var transform: PBIN_SERVER_PORT -> server.port (strip prefix, lowercase, _ -> .)
- [Phase 01-foundation]: Two-pool SQLite: WriteDB.SetMaxOpenConns(1) + ReadDB unbounded — prevents SQLITE_BUSY
- [Phase 01-foundation]: goose.SetBaseFS(embed.FS) + goose.Up in Open() — migrations run automatically at startup
- [Phase 01-foundation]: LocalFS keys must be ^[a-zA-Z0-9]+$ — path traversal prevention at key validation layer
- [Phase 01-foundation]: Atomic file write via os.CreateTemp + os.Rename — prevents partial writes visible to readers
- [Phase 01-foundation]: Repository interfaces in domain packages (file.Repository not storage.FileRepository) — prevents import cycles
- [Phase 01-foundation]: Expiry preset validation in New() constructor — only 9 allowed values, invalid is a typed error
- [Phase 01-foundation]: ExpiryDuration panics on invalid input — callers must validate via New() first; panic = programming error
- [Phase 01-foundation]: main.go is wire-only — config, DB, filestore, handlers, server; no business logic in entrypoint
- [Phase 02-file-sharing]: Bytes-first upload order: filestore.Write before repo.Create; best-effort store.Delete rollback on DB failure
- [Phase 02-file-sharing]: ExpiresAt is a read-time field on File entity set only by the repository, not by New() constructor
- [Phase 02-file-sharing]: Read-time expiry enforced in Service.Get() immediately after GetBySlug, before password check
- [Phase 02-file-sharing]: MarkDownloaded uses atomic UPDATE WHERE downloaded_at IS NULL + RowsAffected — prevents TOCTOU race
- [Phase 02-file-sharing]: No SVG in SupportedImageMIMETypes — XSS risk (SVG can contain script tags)
- [Phase 02-file-sharing]: FileHandler accepts FileService interface (not *file.Service) for mock injection in tests; concrete *file.Service satisfies it at main.go wiring site
- [Phase 02-file-sharing]: GET /{slug}/info registered before GET /{slug} in mux — Go 1.22 stdlib routing requires more-specific patterns first
- [Phase 02-file-sharing]: securityHeaders() must be called in every handler including health — health handler was missing it (auto-fixed in 02-04)
- [Phase 03-buckets-and-paste]: Each ALTER TABLE statement uses its own goose StatementBegin/End block — goose requires one statement per block
- [Phase 03-buckets-and-paste]: Down migration uses SELECT 1 placeholder — intentionally irreversible, mirrors 002 pattern for SQLite compatibility
- [Phase 03-buckets-and-paste]: Paste.ExpiresAt is read-time field on Paste entity populated only by the repository (mirrors File pattern)
- [Phase 03-buckets-and-paste]: DeleteSecret set by service after New() constructor — constructor signature unchanged
- [Phase 03-buckets-and-paste]: Bucket URLs use /b/ prefix (e.g. baseURL/b/slug) distinct from single-file URLs
- [Phase 03-buckets-and-paste]: Each bucket file gets its own storageKey from slug.New (not bucket slug) — independent on-disk keys
- [Phase 03-buckets-and-paste]: StreamZIP writes zip.NewWriter(w) directly to http.ResponseWriter — no bytes.Buffer, no Content-Length
- [Phase 03-buckets-and-paste]: Per-request crypto/rand nonce applied to CSP header and inline script/style nonce attributes in paste view — isolates paste page CSP from file pages
- [Phase 03-buckets-and-paste]: GetFile added to bucket.Service — finds file by StorageKey in bucket.Files then calls store.Read
- [Phase 03-buckets-and-paste]: BucketHandler DownloadFile returns 401 JSON for wrong password (no password form — individual file downloads are API-like)
- [Phase 03-buckets-and-paste]: Password forwarded via query param in per-file and ZIP links in View HTML so browsing preserves auth

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 3]: Verify `modernc.org/sqlite` v1.47.0 supports `DELETE ... RETURNING *` syntax (SQLite 3.35+ required; bundled is 3.51.3 — spike before one-use implementation)
- [Phase 4]: Syntax highlighting strategy decision needed before frontend work: Chroma (server-side, Go dep) vs highlight.js (client-only, no Go dep). Recommendation: highlight.js for v1.

## Session Continuity

Last session: 2026-03-19T20:24:42.017Z
Stopped at: Completed 03-04-PLAN.md
Resume file: None
