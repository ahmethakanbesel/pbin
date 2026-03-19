# Phase 2: File Sharing - Context

**Gathered:** 2026-03-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can upload files, receive shareable links with configurable expiry, download files via direct URL, protect shares with passwords, mark files as one-time download, and get embed codes for images. This covers FILE-01 through FILE-08.

</domain>

<decisions>
## Implementation Decisions

### Upload response
- Minimal JSON response: URL, delete URL, and expiry only
- Deletion via slug-based secret URL: `/delete/{slug}/{secret}` — no separate token field, secret is a second random slug stored in DB
- Shareable URL format: `/{slug}` — simple, no namespace prefix
- Upload endpoint: `POST /api/upload` only (standard multipart form upload) — no PUT support in v1

### Image embed behavior
- Image validation: extension AND magic byte check (PNG signature, JPEG SOI marker, GIF89a/GIF87a, WebP RIFF, BMP header)
- Supported embed formats: PNG, JPEG, GIF, WebP, BMP — no SVG (XSS risk)
- Embed codes appear on the web share page only, not in API response
- Validated images served inline with correct Content-Type (`image/png`, `image/jpeg`, etc.) — enables `<img src>` embedding
- Non-image files always served as attachment
- Embed code formats: HTML (`<img>`), BBCode (`[img]`), Markdown (`![]()`), Direct link

### Password UX
- Browser: visiting `/{slug}` for password-protected files shows a password input form; correct password reveals the download
- API: password submitted via `X-Password` header
- Password stored as bcrypt hash in DB (already in schema)

### Download security
- All non-image files: `Content-Disposition: attachment` + `X-Content-Type-Options: nosniff`
- Validated images: `Content-Disposition: inline` with correct `Content-Type`
- Strict Content Security Policy on share pages — no inline scripts, no external resources
- Security headers on all responses: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`

### Claude's Discretion
- Exact password form page HTML/CSS design
- bcrypt cost factor
- Delete secret slug length
- `Content-Security-Policy` header exact value
- Error response format for expired/deleted/invalid files
- Whether to add `delete_secret` column to files table via a new migration or reuse existing schema

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing codebase (Phase 1 output)
- `internal/domain/file/file.go` — File entity with constructor validation, expiry presets, fields
- `internal/domain/file/repository.go` — Repository interface (Create, GetBySlug, MarkDownloaded, Delete, ListExpired)
- `internal/storage/migrations/001_init.sql` — Schema with files table (slug, filename, size, mime_type, password, one_use, downloaded_at, expires_at)
- `internal/filestore/store.go` — StorageBackend interface (Put, Get, Delete, Exists)
- `internal/filestore/local.go` — LocalFS implementation with path traversal protection
- `internal/slug/slug.go` — Crypto slug generator (New, MustNew)
- `internal/config/config.go` — Config struct with Upload.MaxBytes
- `cmd/pbin/main.go` — Entrypoint wiring config, DB, filestore, mux

### Research
- `.planning/research/PITFALLS.md` — Multipart temp file leaks, one-use TOCTOU race, stored XSS via uploads
- `.planning/research/ARCHITECTURE.md` — Handler → Service → Repository flow, three-layer separation

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `file.New()` — Constructor with expiry validation, already handles all expiry presets
- `file.Repository` — Interface already has `MarkDownloaded()` for atomic one-use semantics
- `slug.New(n)` — Crypto-random slug generator, reuse for both share slugs and delete secrets
- `filestore.Backend` — Put/Get/Delete/Exists interface, LocalFS implements with slug-keyed 2-char prefix dirs
- `storage.Open()` — SQLite two-pool with WAL, goose migrations

### Established Patterns
- Domain entity constructors validate invariants (file.New checks expiry preset, slug non-empty)
- Repository interfaces in domain package, implementations in storage layer
- Handler in `internal/handler/` — separate HTTP transport layer
- Slug-based keys for disk storage (never user-supplied filenames)

### Integration Points
- `cmd/pbin/main.go` — needs file upload/download/delete handlers wired into mux
- `internal/handler/` — add file handlers alongside health.go
- Files table schema already exists — may need `delete_secret` column (new migration)
- `internal/domain/file/file.go` — may need `DeleteSecret` field added to entity

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches following the patterns established in Phase 1.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-file-sharing*
*Context gathered: 2026-03-19*
