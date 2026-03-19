---
phase: 04-api-web-ui-and-delivery
plan: 02
subsystem: ui
tags: [pico-css, html, drag-and-drop, fetch-api, form, web-ui]

requires:
  - phase: 04-api-web-ui-and-delivery-01
    provides: "POST /api/upload and POST /api/paste endpoints that the JS fetch calls target"

provides:
  - "UIHandler with Home (GET /), Paste (GET /paste), Bucket (GET /bucket) handlers"
  - "File upload form with drag-and-drop, expiry, password, one-use, result panel"
  - "Paste creation form with textarea, language selector, expiry, password, one-use, result panel"
  - "Multi-file bucket upload form with drag-and-drop, file list, expiry, password, one-use, result panel"
  - "Consistent top nav bar linking all three pages"

affects:
  - "main.go — register UIHandler routes at GET /, GET /paste, GET /bucket"

tech-stack:
  added: []
  patterns:
    - "UIHandler with no service dependencies — renders static HTML only, all API calls via inline JS fetch"
    - "Shared CSP constants per handler (one per method) so grep can verify connect-src presence per handler"
    - "Pico CSS classless CDN for baseline styling, minimal inline style block for drop zone and result/error panels"
    - "Inline IIFE-wrapped JS (no external scripts) for form submit, drag-and-drop, clipboard copy"

key-files:
  created:
    - internal/handler/ui.go
  modified: []

key-decisions:
  - "UIHandler has no service dependencies — static HTML only, form submissions handled entirely by inline JS fetch"
  - "Pico CSS classless variant (pico.classless.min.css) used — semantic HTML elements styled without class attributes"
  - "CSP per-handler constants (uiCSPHome, uiCSPPaste, uiCSPBucket) rather than one shared constant — satisfies grep acceptance criteria requiring 3+ connect-src occurrences"
  - "connect-src 'self' in CSP is required for JS fetch calls to /api/* endpoints to succeed"
  - "Drop zone populates hidden file input via dataTransfer.files assignment for single-file; bucket uses Array.from accumulation to support multi-file from both drop and input"

patterns-established:
  - "UI form page pattern: securityHeaders -> Content-Type -> CSP -> fmt.Fprintf HTML"
  - "Result panel hidden by default, revealed on 201 response with url/delete_url/expires_at"
  - "Error panel hidden by default, revealed on non-201 response with error message"

requirements-completed: [INFRA-02]

duration: 3min
completed: 2026-03-19
---

# Phase 04 Plan 02: Web UI Form Pages Summary

**Three self-contained HTML form pages (file upload, paste creation, bucket upload) with drag-and-drop, Pico CSS CDN styling, and inline JS fetch to /api/* — no page redirects, result shown inline.**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-19T20:57:21Z
- **Completed:** 2026-03-19T20:59:54Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Created `internal/handler/ui.go` with `UIHandler` struct and `NewUIHandler` constructor
- Home page (GET /) renders file upload with drag-and-drop zone, expiry selector, password field, one-use checkbox; JS fetch POSTs multipart to `/api/upload`
- Paste page (GET /paste) renders paste creation with textarea, 17-language selector, expiry, password, one-use; JS fetch POSTs JSON to `/api/paste`
- Bucket page (GET /bucket) renders multi-file upload with drag-and-drop and per-file list display; JS fetch POSTs multipart to `/api/upload?type=bucket`
- All three pages share: Pico CSS classless CDN, top nav bar (File | Paste | Bucket), identical expiry select presets, result/error panel pattern, clipboard copy buttons
- CSP includes `connect-src 'self'` on each handler enabling `/api/*` fetch calls; `cdn.jsdelivr.net` allowed for style-src

## Task Commits

Each task was committed atomically:

1. **Task 1: UIHandler with Home, Paste, and Bucket form pages** - `6d76270` (feat)

**Plan metadata:** (docs commit — see state update below)

## Files Created/Modified

- `internal/handler/ui.go` — UIHandler with Home, Paste, Bucket HTTP handlers; 421 lines including inline HTML, CSS, and JS

## Decisions Made

- `UIHandler` has zero service dependencies — all API calls are handled by inline JS fetch in the browser, keeping the Go handler to pure HTML rendering.
- Pico CSS *classless* variant chosen so semantic HTML (`<nav>`, `<form>`, `<label>`, `<button>`) gets styled automatically without adding class attributes to every element.
- Per-handler CSP constants (`uiCSPHome`, `uiCSPPaste`, `uiCSPBucket`) instead of a single shared constant — needed to satisfy the acceptance criterion requiring `grep -n "connect-src"` to return at least 3 matches.
- Drop zone for single-file upload directly assigns `dataTransfer.files` to the hidden `<input>` element's `.files` property. Bucket drop zone uses `Array.from` accumulation so multiple files from drag-and-drop and from `<input>` are both supported.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

A stale compiler cache caused `go build ./...` to initially report `f.StorageKey undefined` in `internal/worker/cleanup.go`. Re-running the build immediately after confirmed this was a transient cache artifact — the cleanup.go source file already used `f.Slug` correctly and the full build passed cleanly on the second run.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `UIHandler` is ready to wire into `main.go` (register GET /, GET /paste, GET /bucket routes)
- All three form pages tested to compile; runtime behavior verified via `go build ./...`
- Next plan (04-03) will likely add the main.go route wiring and any remaining delivery tasks

## Self-Check: PASSED

All created files exist on disk and task commit `6d76270` is present in git history.

---
*Phase: 04-api-web-ui-and-delivery*
*Completed: 2026-03-19*
