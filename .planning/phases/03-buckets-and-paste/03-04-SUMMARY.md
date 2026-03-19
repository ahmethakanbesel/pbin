---
phase: 03-buckets-and-paste
plan: 04
subsystem: api
tags: [go, http, multipart, zip, bucket, handler]

requires:
  - phase: 03-02
    provides: BucketService with Create, GetMeta, StreamZIP, Delete and domain types

provides:
  - BucketHandler with Upload, View, DownloadFile, DownloadZIP, DeleteBucket HTTP methods
  - GetFile method added to bucket.Service and BucketService interface
  - Per-file download links in bucket view page (/{slug}/file/{storageKey})

affects:
  - main.go wiring (BucketHandler must be registered in ServeMux)
  - integration tests for bucket endpoints

tech-stack:
  added: []
  patterns:
    - "BucketHandler follows same thin-handler pattern as FileHandler — interface injection, securityHeaders first, delegate to service"
    - "Multi-file upload iterates r.MultipartForm.File[\"file\"] slice (not single r.FormFile)"
    - "DownloadZIP delegates entirely to svc.StreamZIP — no zip.Writer in handler layer"
    - "humanSize and htmlEscape are package-private helpers defined in bucket.go"

key-files:
  created:
    - internal/handler/bucket.go
  modified:
    - internal/domain/bucket/service.go

key-decisions:
  - "GetFile added to Service (not just interface) — method finds file by StorageKey in bucket.Files slice then calls store.Read"
  - "DownloadFile returns 401 JSON for wrong password (no password form — individual file downloads are API-like)"
  - "View and DownloadZIP serve password form for browser requests; 401 JSON for Accept:application/json or X-Password header"
  - "htmlEscape and humanSize are unexported helpers in bucket.go to avoid importing html/template for minimal page rendering"
  - "Password query param is forwarded to per-file links and ZIP link in View HTML so password is preserved across navigation"

patterns-established:
  - "BucketService interface in handler package mirrors FileService — allows mock injection"
  - "securityHeaders(w) called as first statement in every handler method (enforces X-Content-Type-Options: nosniff and X-Frame-Options: DENY)"

requirements-completed: [BUCK-01, BUCK-02, BUCK-03, BUCK-04, BUCK-05]

duration: 2min
completed: 2026-03-19
---

# Phase 03 Plan 04: BucketHandler Summary

**HTTP handler for multi-file bucket operations — Upload (multipart), View (HTML file list), DownloadFile, DownloadZIP (streaming), and Delete — with GetFile added to bucket.Service**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-19T20:21:24Z
- **Completed:** 2026-03-19T20:23:35Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments

- Added `GetFile` to `bucket.Service` — auth/expiry checks then `store.Read` by `StorageKey`
- Created `internal/handler/bucket.go` with `BucketHandler` struct and `BucketService` interface
- Upload accepts multiple `file` fields from multipart form; detects MIME per file with peek/MultiReader pattern
- View renders PsiTransfer-style HTML with per-file download links (`/{slug}/file/{storageKey}`) and ZIP button
- DownloadZIP delegates entirely to `svc.StreamZIP` — handler writes no bytes before the service call
- All five handler methods call `securityHeaders(w)` as first statement

## Task Commits

1. **Task 1: BucketHandler Upload, View, DownloadFile, DownloadZIP, and Delete** - `0459716` (feat)

## Files Created/Modified

- `internal/handler/bucket.go` - BucketHandler with all five HTTP methods and BucketService interface
- `internal/domain/bucket/service.go` - Added GetFile method

## Decisions Made

- `GetFile` method finds the matching file by iterating `b.Files` by `StorageKey` — simple linear scan appropriate for expected bucket sizes
- `DownloadFile` returns 401 JSON on wrong password (no HTML form) — individual file downloads are programmatic/API-like
- Password is forwarded via query param in per-file links and ZIP link in View page so browsing between pages doesn't lose auth
- `htmlEscape` and `humanSize` are unexported helpers in `bucket.go` — avoids importing `html/template` for simple page rendering

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added GetFile to bucket.Service**

- **Found during:** Task 1 (BucketHandler implementation)
- **Issue:** `GetFile` method was listed in the plan's BucketService interface but not yet present in `internal/domain/bucket/service.go`
- **Fix:** Implemented `GetFile` on `*Service` — performs same auth/expiry checks as `GetMeta`, finds file by `StorageKey`, calls `store.Read`
- **Files modified:** `internal/domain/bucket/service.go`
- **Verification:** `go build ./...` passes; `GetFile` callable in handler
- **Committed in:** `0459716` (part of task commit)

---

**Total deviations:** 1 auto-fixed (missing critical method)
**Impact on plan:** Required for correctness — handler interface required GetFile but service implementation was missing it. No scope creep.

## Issues Encountered

None — project compiled cleanly after adding GetFile and creating the handler.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- BucketHandler is complete and compiles
- Ready to be wired into main.go ServeMux alongside FileHandler and PasteHandler
- Route registration needed: POST /api/upload?type=bucket, GET /b/{slug}, GET /b/{slug}/file/{storageKey}, GET /b/{slug}/zip, GET /b/delete/{slug}/{secret}

---
*Phase: 03-buckets-and-paste*
*Completed: 2026-03-19*
