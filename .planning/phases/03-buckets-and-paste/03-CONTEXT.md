# Phase 3: Buckets and Paste - Context

**Gathered:** 2026-03-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Two sub-domains: (1) Multi-file transfer buckets — upload multiple files in a single request, shareable bucket URL, individual file downloads + ZIP bundle, password protection, one-use, expiry. (2) Pastebin — create text pastes with optional title/language, syntax highlighting, raw endpoint, one-use, expiry. Both reuse the expiry preset system and security patterns from Phase 2. Covers BUCK-01 through BUCK-05 and PASTE-01 through PASTE-05.

</domain>

<decisions>
## Implementation Decisions

### Bucket upload flow
- Single multipart request: `POST /api/upload?type=bucket` with multiple file fields — atomic, same base endpoint as file upload
- Response includes: bucket URL, delete URL, expiry, file count, and list of individual files (name, size, download URL)
- Individual files in a bucket are downloadable separately AND as a ZIP bundle
- Bucket page shows file list with individual download links + "Download all as ZIP" button
- Reuse Phase 2 patterns: slug-based delete secret, password via `X-Password` header / form page, bcrypt, atomic one-use

### Syntax highlighting
- Client-side via highlight.js loaded from CDN (cdnjs or unpkg)
- Auto-detect language from content + user can override via language selector
- Theme: auto light/dark respecting `prefers-color-scheme` media query (GitHub light + Atom One Dark)
- CSP must allow the CDN script-src for paste pages (different from file pages which have strict no-external CSP)

### Paste view page
- Line numbers alongside the code
- Copy button (one-click clipboard copy via JS)
- Raw link to `/raw/{slug}` for plain text access
- Metadata bar above code: title, language, created date, expiry info
- Paste creation endpoint: `POST /api/paste` — JSON body with content, title, lang, expiry, password, one_use fields

### Paste API & URLs
- Paste view: `/{slug}` — HTML page with syntax highlighting (same URL pattern as files, route by entity type)
- Raw paste: `/raw/{slug}` — plain text content, `Content-Type: text/plain`
- Paste delete: `/delete/{slug}/{secret}` — same pattern as files

### Carried from Phase 2 (locked)
- Security headers on all responses: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`
- Password: form page for browsers, `X-Password` header for API, bcrypt hash in DB
- One-use: atomic `UPDATE WHERE downloaded_at/viewed_at IS NULL` + RowsAffected
- Expiry: fixed presets (10min, 1h, 6h, 1d, 7d, 30d, 90d, 1y, never), read-time enforcement
- Deletion: slug-based secret URL

### Claude's Discretion
- Bucket page HTML/CSS design
- Paste view page HTML/CSS design
- ZIP streaming implementation details (archive/zip writing directly to ResponseWriter)
- How to differentiate files vs pastes vs buckets on `GET /{slug}` (e.g., lookup order, entity type field, or prefix-based routing)
- highlight.js CDN URL and version
- Language list for the paste form dropdown
- Whether bucket files get their own slugs or use bucket_slug + index
- Paste deletion secret storage (new column or reuse existing pastes table design)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing codebase (Phase 1 + 2 output)
- `internal/domain/file/service.go` — FileService pattern (Upload/Get/GetMeta/Delete) to replicate for Bucket and Paste services
- `internal/domain/file/repository.go` — Repository interface pattern (consumer-side, in domain package)
- `internal/handler/file.go` — FileHandler pattern (service interface, securityHeaders, writeError, password form, embed page)
- `internal/storage/file_repo.go` — SQLite repository implementation pattern (two-pool, atomic MarkDownloaded)
- `internal/storage/migrations/001_init.sql` — Schema for buckets, bucket_files, and pastes tables (already created)
- `internal/domain/bucket/bucket.go` — Bucket entity (exists from Phase 1, needs extension)
- `internal/domain/paste/paste.go` — Paste entity (exists from Phase 1, needs extension)
- `internal/domain/bucket/repository.go` — Bucket repository interface (exists, may need extension)
- `internal/domain/paste/repository.go` — Paste repository interface (exists, may need extension)
- `cmd/pbin/main.go` — Entrypoint wiring pattern

### Research
- `.planning/research/PITFALLS.md` — ZIP streaming without temp files, one-use TOCTOU race
- `.planning/research/ARCHITECTURE.md` — Domain boundary patterns, handler → service → repository flow

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `file.Service` pattern — Upload/Get/GetMeta/Delete with sentinel errors, bcrypt, slug generation; replicate for bucket and paste
- `handler.FileHandler` pattern — service interface, securityHeaders(), writeError(), servePasswordForm()
- `storage.NewFileRepo` pattern — SQLite repo with two-pool, goose migrations
- `slug.New(n)` — crypto-random slug generation
- `filestore.Backend` — disk storage for bucket files (same Write/Read/Delete interface)
- Schema already exists: `buckets`, `bucket_files`, `pastes` tables created in 001_init.sql

### Established Patterns
- Domain entity constructors validate invariants
- Repository interfaces in domain package, implementations in storage layer
- Handlers in `internal/handler/`, accept service interfaces for testability
- Minimal JSON responses for API, HTML pages for browser views
- `http.MaxBytesReader` before multipart parsing + `defer RemoveAll()`

### Integration Points
- `cmd/pbin/main.go` — needs bucket and paste handlers wired into mux
- `GET /{slug}` — currently serves files; needs routing to differentiate files vs pastes vs buckets
- `buckets` and `pastes` tables already exist in schema — may need `delete_secret` columns (new migration)

</code_context>

<specifics>
## Specific Ideas

- Bucket page should feel like PsiTransfer — clean file list with sizes, individual download links, and a prominent "Download all as ZIP" button
- Paste page should feel like Hastebin/GitHub Gist — code block with line numbers, clean metadata bar

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 03-buckets-and-paste*
*Context gathered: 2026-03-19*
