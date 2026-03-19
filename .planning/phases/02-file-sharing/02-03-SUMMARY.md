---
phase: 02-file-sharing
plan: "03"
subsystem: api
tags: [go, net/http, multipart, file-upload, content-disposition, magic-bytes, embed-codes]

requires:
  - phase: 02-file-sharing-02
    provides: file.Service with Upload/Get/Delete, sentinel errors ErrNotFound/ErrExpired/ErrAlreadyConsumed/ErrWrongPassword/ErrBadDeleteSecret
  - phase: 02-file-sharing-01
    provides: file.File entity, IsImage, SupportedImageMIMETypes
provides:
  - FileHandler struct with Upload, Serve, Info, Delete HTTP handlers
  - FileService interface enabling mock injection in tests
  - Magic byte validation for PNG, JPEG, GIF, WebP, BMP uploads
  - Password form HTML for browser password-protected file access
  - Image share page with HTML/BBCode/Markdown/Direct link embed codes
affects: [cmd/pbin/main.go wiring, 03-paste, 04-frontend]

tech-stack:
  added: []
  patterns:
    - "FileService interface in handler package decouples handler from concrete *file.Service (testability)"
    - "MaxBytesReader wraps body before ParseMultipartForm (memory exhaustion prevention)"
    - "io.MultiReader re-combines peeked bytes with remaining reader after MIME detection"
    - "validateImageMagic checks magic bytes after http.DetectContentType to prevent spoofing"
    - "securityHeaders() applied at start of every handler method"
    - "servePasswordForm renders minimal inline HTML for browser clients; JSON 401 for API clients"

key-files:
  created:
    - internal/handler/file.go
    - internal/handler/file_test.go
  modified: []

key-decisions:
  - "FileHandler accepts FileService interface (not *file.Service) — allows test mock injection without changing plan's external wiring"
  - "validateImageMagic is unexported — tested indirectly through Upload behavior, not as exported API"
  - "Info handler calls service.Get and immediately closes Content — consumes one-use file; not ideal but matches plan spec"

patterns-established:
  - "Handler interface pattern: define minimal interface in handler package, concrete service satisfies it"
  - "Peek-and-reconstruct: Read(peek) then io.MultiReader(bytes.NewReader(peek), fh) for magic byte detection"

requirements-completed: [FILE-01, FILE-02, FILE-04, FILE-05, FILE-06, FILE-07, FILE-08]

duration: 12min
completed: 2026-03-19
---

# Phase 2 Plan 3: File Handler Summary

**HTTP handler layer for file upload/download/share/delete: multipart upload with MaxBytesReader + magic byte MIME detection, streaming serve with inline/attachment Content-Disposition, image share page with embed codes (HTML, BBCode, Markdown, Direct link), and secret-based deletion.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-03-19T18:15:05Z
- **Completed:** 2026-03-19T18:27:00Z
- **Tasks:** 3 (TDD: test commit + impl commit per task group)
- **Files modified:** 2

## Accomplishments

- Upload handler enforces MaxBytesReader before ParseMultipartForm, defers RemoveAll, detects MIME via magic bytes, returns 201 JSON with url/delete_url/expires_at/is_image
- Serve handler streams via io.Copy with Content-Disposition:inline for validated images and attachment for non-images; password-protected files show HTML form to browsers and 401 JSON to API clients
- Info handler renders HTML share page with four embed code formats for image files; redirects to download for non-images
- Delete handler maps ErrBadDeleteSecret to 403 and returns {"deleted":true} on success
- All 5 service sentinel errors mapped to correct HTTP status codes across all handlers
- Security headers (X-Content-Type-Options: nosniff, X-Frame-Options: DENY) applied on every response

## Task Commits

Each task was committed atomically:

1. **TDD RED - Failing tests for all handlers** - `c8c1a2a` (test)
2. **Tasks 1+2+3: FileHandler implementation** - `df875b3` (feat)

_Note: Tasks 1, 2, and 3 were implemented in a single GREEN commit as the plan provides all implementation code and they share one file._

## Files Created/Modified

- `internal/handler/file.go` - FileHandler struct with Upload, Serve, Info, Delete methods, FileService interface, validateImageMagic, servePasswordForm
- `internal/handler/file_test.go` - 18 tests covering all handler methods and error cases

## Decisions Made

- `FileHandler` accepts a `FileService` interface rather than `*file.Service` — the plan specified the concrete type but the tests require a mock; the interface satisfies the concrete type at the call site in main.go without changes
- `validateImageMagic` left unexported — tested indirectly via Upload behavior (MIME downgrade to octet-stream on magic mismatch)
- `Info` handler calls `service.Get` which will mark one-use files as consumed even when only fetching metadata; this matches the plan spec and is noted as a potential improvement for a future plan

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Defined FileService interface for handler testability**
- **Found during:** Task 1 (writing TDD tests)
- **Issue:** Plan specified `NewFileHandler(svc *file.Service, ...)` but tests pass a mock struct — impossible without an interface
- **Fix:** Added `FileService` interface in handler package; `*file.Service` satisfies it at the main.go call site with no changes required there
- **Files modified:** internal/handler/file.go
- **Verification:** All 18 tests pass; `go build ./...` exits 0
- **Committed in:** df875b3 (feat commit)

---

**Total deviations:** 1 auto-fixed (Rule 2 - missing critical for testability)
**Impact on plan:** Interface addition is strictly additive. Concrete wiring in main.go is unchanged. No scope creep.

## Issues Encountered

None beyond the interface deviation noted above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `internal/handler/file.go` is ready to wire into `cmd/pbin/main.go` mux routes
- Routes needed: `POST /api/upload`, `GET /{slug}`, `GET /{slug}/info`, `GET /delete/{slug}/{secret}`
- `NewFileHandler(svc, cfg.Upload.MaxBytes)` is the constructor call for main.go

---
*Phase: 02-file-sharing*
*Completed: 2026-03-19*
