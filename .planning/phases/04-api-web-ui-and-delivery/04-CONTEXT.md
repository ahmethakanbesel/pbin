# Phase 4: API, Web UI, and Delivery - Context

**Gathered:** 2026-03-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Polish the REST API surface for consistency, add embedded web UI with upload forms for non-technical users, add Basic Auth middleware on write endpoints, implement background expiry cleanup worker, and ensure max upload size enforcement. This is the final delivery phase — after this, pbin is shippable. Covers INFRA-01 through INFRA-05.

</domain>

<decisions>
## Implementation Decisions

### Web UI upload forms
- Separate pages: `/` (file upload homepage), `/paste` (paste creation), `/bucket` (multi-file bucket upload)
- CSS: Pico CSS loaded from CDN — classless framework, clean defaults
- Drag-and-drop file upload on file and bucket pages (requires JS drop zone)
- Simple top nav bar with links: File | Paste | Bucket
- Each page has its own form that POSTs to the respective API endpoint
- After successful upload, show the result (shareable URL, delete URL, etc.)

### Cleanup worker
- Background goroutine running every 15 minutes
- Sweeps all three domains: files, buckets, pastes
- Deletes expired DB rows AND their on-disk data (files + bucket files via filestore.Backend)
- Logging: summary only — "Cleaned up N files, N buckets, N pastes" per sweep
- Worker restarts on panic (defer/recover pattern)
- Starts at application startup, stops on graceful shutdown

### Basic Auth (locked from project setup)
- Gates write endpoints only: POST /api/upload, POST /api/paste, upload form pages
- Read endpoints remain public (GET /{slug}, download, raw, etc.)
- Enabled via config: auth.enabled=true, auth.username, auth.password
- Standard HTTP Basic Auth header check

### Claude's Discretion
- Exact HTML templates for upload forms
- Pico CSS CDN URL and version
- Drop zone JS implementation details
- API response consistency audit (ensure all endpoints follow same JSON error format)
- embed.FS setup for static assets (if any beyond CDN)
- Cleanup worker error handling and retry behavior
- Whether to use Go html/template or fmt.Fprintf for HTML pages
- Navigation bar HTML/CSS details

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing codebase
- `cmd/pbin/main.go` — Current entrypoint with all domain wiring and route dispatch
- `internal/handler/file.go` — FileHandler pattern, shared helpers (securityHeaders, writeJSON, writeError, servePasswordForm)
- `internal/handler/bucket.go` — BucketHandler with View page HTML (pattern for upload form HTML)
- `internal/handler/paste.go` — PasteHandler with View page HTML (pattern for paste form HTML)
- `internal/config/config.go` — Config struct with Auth.Enabled/Username/Password, Upload.MaxBytes
- `internal/domain/file/repository.go` — ListExpired method for cleanup
- `internal/domain/bucket/repository.go` — ListExpired method for cleanup
- `internal/domain/paste/repository.go` — ListExpired method for cleanup
- `internal/filestore/store.go` — Backend.Delete for cleanup

### Research
- `.planning/research/PITFALLS.md` — Background cleanup goroutine pattern with stop channel

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `securityHeaders()`, `writeJSON()`, `writeError()`, `servePasswordForm()` — shared handler helpers
- `ListExpired()` on all three repository interfaces — already defined, ready for cleanup worker
- `filestore.Backend.Delete()` — disk cleanup
- Config already has `Auth` section with Enabled, Username, Password fields
- HTML page patterns in bucket.go (View) and paste.go (View) — can replicate for upload forms

### Established Patterns
- HTML rendered via `fmt.Fprintf` with inline CSS (bucket view, paste view, file info page)
- Security headers on every handler
- JSON error responses with `writeError()`
- Pico CSS from CDN (new for this phase, but highlight.js CDN pattern exists in paste handler)

### Integration Points
- `cmd/pbin/main.go` — add Basic Auth middleware, start cleanup worker, register upload form routes
- Need to export `writeJSON` and `writeError` or make them accessible for middleware
- Cleanup worker needs access to all three repositories + filestore backend

</code_context>

<specifics>
## Specific Ideas

No specific requirements — follow the established patterns from Phases 2-3 for consistency.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-api-web-ui-and-delivery*
*Context gathered: 2026-03-19*
