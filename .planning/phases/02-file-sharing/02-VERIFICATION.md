---
phase: 02-file-sharing
verified: 2026-03-19T00:00:00Z
status: gaps_found
score: 16/17 must-haves verified
gaps:
  - truth: "Info handler does not consume one-use files (metadata-only access)"
    status: failed
    reason: "Info calls service.Get which atomically calls MarkDownloaded for one-use files, consuming the download slot even though Info only needs metadata. Visiting GET /{slug}/info for a one-use image file will mark the file as consumed before the actual download."
    artifacts:
      - path: "internal/handler/file.go"
        issue: "Info() calls h.svc.Get() at line 317; service.Get() calls s.repo.MarkDownloaded() for any OneUse file (service.go line 154-162). Content is immediately closed but the download is already consumed."
      - path: "internal/domain/file/service.go"
        issue: "Get() has no parameter to skip one-use consumption. A separate metadata-only method (e.g., GetMeta) is needed, or Get() needs a skipConsume bool/option."
    missing:
      - "A metadata-only path in the service (e.g., service.GetMeta or a skipConsume option on Get) that returns file metadata without marking one-use files as consumed"
      - "Info handler must use this metadata path so visiting /{slug}/info does not consume a one-use file's single download"
human_verification:
  - test: "End-to-end curl: upload one-use image, visit /{slug}/info, then download /{slug}"
    expected: "Info page renders embed codes; subsequent GET /{slug} still delivers the file (200), not 410 Gone"
    why_human: "Programmatic code review confirms the bug — human test would confirm the real-world impact and verify the fix"
  - test: "Password-protected file: visit /{slug} without a password in a browser"
    expected: "HTML password form renders correctly; submitting correct password delivers the file"
    why_human: "Browser rendering of the inline HTML form cannot be verified programmatically"
  - test: "Image upload: verify Content-Disposition is inline and Content-Type matches image MIME"
    expected: "curl -v shows Content-Disposition: inline; filename=... and Content-Type: image/png (or jpeg/gif/webp/bmp)"
    why_human: "Confirms correct streaming behavior and MIME type passthrough"
---

# Phase 2: File Sharing Verification Report

**Phase Goal:** Users can upload files, receive shareable links with configurable expiry, download files via direct URL, protect shares with passwords, mark files as one-time download, and get embed codes for images.
**Verified:** 2026-03-19
**Status:** gaps_found
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Schema migration adds delete_secret column to files table | VERIFIED | `002_add_delete_secret.sql` contains `ALTER TABLE files ADD COLUMN delete_secret TEXT` with correct goose Up/Down markers |
| 2 | File entity carries DeleteSecret field and IsImage() works for 5 MIME types | VERIFIED | `file.go`: DeleteSecret field present; SupportedImageMIMETypes = ["image/png","image/jpeg","image/gif","image/webp","image/bmp"]; IsImage() uses O(1) map lookup |
| 3 | Service can create a file: generates slug + delete_secret, hashes password, persists bytes + metadata | VERIFIED | service.go Upload(): slug.New(12) + slug.New(24), bcrypt cost 12, store.Write then repo.Create with rollback on failure |
| 4 | Service enforces one-use atomically via MarkDownloaded returning (false, nil) for already-consumed files | VERIFIED | service.go Get(): `if f.OneUse { consumed, err := s.repo.MarkDownloaded(...); if !consumed { return ErrAlreadyConsumed } }` |
| 5 | Service deletes both DB row and disk bytes together, verified by matching delete_secret | VERIFIED | service.go Delete(): subtle.ConstantTimeCompare on delete secret; store.Delete then repo.Delete |
| 6 | SQLite repository implements all 5 methods of file.Repository | VERIFIED | file_repo.go: `var _ file.Repository = (*fileRepo)(nil)` compile-time check; all 5 methods present (Create, GetBySlug, MarkDownloaded, Delete, ListExpired) |
| 7 | Create INSERT includes delete_secret and expires_at columns | VERIFIED | file_repo.go Create(): INSERT INTO files (slug, filename, size, mime_type, password, one_use, expires_at, delete_secret) |
| 8 | GetBySlug returns ErrNotFound and populates ExpiresAt | VERIFIED | file_repo.go GetBySlug(): `errors.Is(err, sql.ErrNoRows) -> file.ErrNotFound`; `expiresAtRaw.Valid -> f.ExpiresAt = &t` |
| 9 | MarkDownloaded uses atomic UPDATE WHERE downloaded_at IS NULL + RowsAffected | VERIFIED | file_repo.go: `UPDATE files SET downloaded_at = unixepoch() WHERE slug = ? AND downloaded_at IS NULL`; `n == 1` check |
| 10 | POST /api/upload enforces size limit, returns JSON with url/delete_url/expires_at/is_image | VERIFIED | handler/file.go: MaxBytesReader before ParseMultipartForm; defer RemoveAll; uploadResponse struct with correct fields; 201 JSON on success; 413 on MaxBytesError |
| 11 | GET /{slug} serves file with correct Content-Disposition (attachment/inline) | VERIFIED | handler/file.go Serve(): images get `inline; filename=...` with image Content-Type; non-images get `attachment; filename=...` with application/octet-stream; streamed via io.Copy |
| 12 | GET /{slug} password protection: form HTML for browsers, 401 JSON for X-Password header | VERIFIED | handler/file.go Serve(): ErrWrongPassword -> servePasswordForm(w, slug) for browsers; 401 JSON when X-Password header present or Accept: application/json |
| 13 | GET /{slug} for expired file returns 410; one-use second request returns 410 | VERIFIED | handler/file.go Serve(): ErrExpired -> 410; ErrAlreadyConsumed -> 410; service.go Get() enforces ExpiresAt read-time check |
| 14 | GET /delete/{slug}/{secret} returns 200 JSON; bad secret returns 403 | VERIFIED | handler/file.go Delete(): service.Delete -> ErrBadDeleteSecret -> 403; success -> `{"deleted":true}` |
| 15 | Security headers on all responses | VERIFIED | securityHeaders(w) called as first line in all 4 handlers (Upload, Serve, Info, Delete) and in Health handler |
| 16 | GET /{slug}/info renders HTML share page with HTML/BBCode/Markdown/Direct link embed codes for images; redirects for non-images | VERIFIED | handler/file.go Info(): non-image -> 302 redirect to /{slug}; image -> HTML page with 4 labeled code blocks (HTML img tag, BBCode [img], Markdown ![](), Direct link); strict CSP `img-src 'self'` |
| 17 | Info handler does not consume one-use files (metadata-only access) | FAILED | Info() calls h.svc.Get() which calls s.repo.MarkDownloaded() atomically for OneUse files — the file is marked consumed before Info can close the content |

**Score:** 16/17 truths verified

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/storage/migrations/002_add_delete_secret.sql` | Goose migration adding delete_secret TEXT column | VERIFIED | Contains `ALTER TABLE files ADD COLUMN delete_secret TEXT` with correct Up/Down markers |
| `internal/domain/file/file.go` | File entity with DeleteSecret, ExpiresAt, IsImage, SupportedImageMIMETypes | VERIFIED | All fields and functions present; ExpiresAt added in plan 02 |
| `internal/domain/file/service.go` | FileService with Upload, Get, Delete methods and sentinel errors | VERIFIED | All 5 sentinel errors; all 3 methods; bcrypt cost 12; subtle.ConstantTimeCompare |
| `internal/domain/file/repository.go` | Repository interface (5 methods) | VERIFIED | Complete interface with all 5 methods |
| `internal/storage/file_repo.go` | SQLite implementation of file.Repository | VERIFIED | Compile-time interface check; all 5 methods; atomic MarkDownloaded; RowsAffected check |
| `internal/handler/file.go` | FileHandler with Upload, Serve, Info, Delete | VERIFIED | All 4 handler methods; magic byte validation; password form HTML; security headers |
| `cmd/pbin/main.go` | Wired FileHandler with correct routes | VERIFIED | NewFileRepo -> file.NewService -> handler.NewFileHandler; 4 routes registered; /{slug}/info before /{slug} |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/domain/file/service.go` | `internal/domain/file/repository.go` | Repository interface injection in NewService | WIRED | `NewService(repo Repository, store filestore.Backend, baseURL string)` |
| `internal/domain/file/service.go` | `internal/filestore/store.go` | filestore.Backend injection in NewService | WIRED | `store filestore.Backend` field used in Upload (Write), Get (Read), Delete |
| `internal/domain/file/service.go` | `internal/slug/slug.go` | slug.New(12) and slug.New(24) | WIRED | `slug.New(slugLength)` and `slug.New(deleteSecretLength)` in Upload |
| `internal/storage/file_repo.go` | `internal/domain/file/repository.go` | fileRepo implements file.Repository | WIRED | `var _ file.Repository = (*fileRepo)(nil)` compile-time check |
| `internal/storage/file_repo.go` | `internal/storage/db.go` | WriteDB for writes, ReadDB for reads | WIRED | `r.db.WriteDB.ExecContext` for Create/MarkDownloaded/Delete; `r.db.ReadDB.QueryRowContext` for GetBySlug/ListExpired |
| `internal/handler/file.go` | `internal/domain/file/service.go` | FileService interface injected via NewFileHandler | WIRED | Handler defines `FileService` interface; NewFileHandler accepts `FileService`; svc.Upload/Get/Delete called |
| `internal/handler/file.go` | `internal/domain/file/service.go` | errors.Is checks against all 5 sentinel errors | WIRED | All 5 errors (ErrNotFound, ErrExpired, ErrAlreadyConsumed, ErrWrongPassword, ErrBadDeleteSecret) mapped in Serve/Delete/Info |
| `cmd/pbin/main.go` | `internal/storage/file_repo.go` | NewFileRepo(db) | WIRED | `fileRepo := storage.NewFileRepo(db)` |
| `cmd/pbin/main.go` | `internal/domain/file/service.go` | file.NewService(repo, fs, baseURL) | WIRED | `fileSvc := file.NewService(fileRepo, fs, baseURL)` |
| `cmd/pbin/main.go` | `internal/handler/file.go` | fileHandler.Upload/Serve/Info/Delete on mux | WIRED | All 4 handler methods registered on correct routes |

---

## Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| FILE-01 | 02-01, 02-02, 02-03, 02-04 | User can upload a file and receive a shareable link | SATISFIED | POST /api/upload returns `{"url":"...","delete_url":"...","expires_at":...,"is_image":...}` |
| FILE-02 | 02-03, 02-04 | User can download a file via direct URL (curl-friendly) | SATISFIED | GET /{slug} streams bytes with Content-Disposition; works with curl |
| FILE-03 | 02-01, 02-02, 02-04 | User can set expiry on upload (10 presets) | SATISFIED | validExpiries map with all 9 presets; expires_at stored as Unix timestamp; read-time expiry enforcement |
| FILE-04 | 02-01, 02-03, 02-04 | User receives a deletion token and can delete the file | SATISFIED | delete_secret stored in DB; GET /delete/{slug}/{secret} with constant-time comparison; returns {"deleted":true} |
| FILE-05 | 02-01, 02-03, 02-04 | User can password-protect a file share | SATISFIED | bcrypt hash stored; handler requires X-Password or query param; browser gets password form; API gets 401 |
| FILE-06 | 02-01, 02-02, 02-03, 02-04 | User can mark file as one-time download | SATISFIED (with gap) | MarkDownloaded atomic UPDATE; second GET returns 410 — but Info page also consumes the one-use slot (gap noted) |
| FILE-07 | 02-01, 02-03, 02-04 | User can get a direct embed link for validated image files | SATISFIED | Magic byte validation for PNG/JPEG/GIF/WebP/BMP; validated images served inline with correct Content-Type |
| FILE-08 | 02-03, 02-04 | User is shown ready-to-copy HTML, BBCode, and Markdown embed codes for image uploads | SATISFIED | GET /{slug}/info renders HTML page with 4 labeled embed code blocks |

---

## Anti-Patterns Found

No stubs, placeholders, TODO/FIXME comments, or empty implementations found across any modified files.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/handler/file.go` | 317 | `Info` calls `svc.Get` which consumes one-use files | Blocker | One-use image files are consumed by visiting /{slug}/info before the actual download |

---

## Human Verification Required

### 1. One-Use File Info Page Behavior

**Test:** Upload a file with `one_use=1` that is an image. Then visit `GET /{slug}/info`. Then attempt `GET /{slug}` (the download).
**Expected (after fix):** Info page renders embed codes; the download still works (200).
**Current behavior (bug):** The download will return 410 Gone because Info already consumed the file.
**Why human:** Confirms the real-world user impact and verifies any fix works end-to-end.

### 2. Browser Password Form Rendering

**Test:** Upload a password-protected file. Open `http://localhost:8080/{slug}` in a real browser (not curl).
**Expected:** An HTML form prompts for a password. Entering the correct password delivers the file.
**Why human:** Browser rendering of inline HTML cannot be verified programmatically; form submission behavior requires browser interaction.

### 3. Image Inline Serving

**Test:** Upload a valid PNG file. `curl -v http://localhost:8080/{slug}`.
**Expected:** `Content-Disposition: inline; filename="..."` and `Content-Type: image/png` headers; body contains PNG bytes.
**Why human:** Confirms streaming behavior and correct MIME type passthrough in a live environment.

---

## Gaps Summary

One gap blocking complete goal achievement:

**Gap: One-use file consumption by Info handler**

The `Info` handler at `GET /{slug}/info` calls `service.Get()` to retrieve file metadata. However, `service.Get()` is also the download path — it atomically calls `MarkDownloaded` for any `OneUse` file. This means visiting the info/share page for a one-use image file consumes the file's single allowed download.

The plan's intent was explicit: "Info does NOT consume the file (does not serve bytes) — it calls service.Get only to retrieve metadata (F and IsImage); it immediately closes result.Content without reading it." However, closing the content does not undo the `MarkDownloaded` call that already fired atomically in the service layer.

**Root cause:** There is no metadata-only access path in `file.Service`. The single `Get()` method conflates expiry enforcement, password checking, one-use consumption, and byte streaming into one call.

**Fix required:**
- Add a `GetMeta(ctx, slug, password)` method to `file.Service` (and `FileService` interface) that returns metadata without calling `MarkDownloaded`
- Change `Info` handler to call `svc.GetMeta` instead of `svc.Get`
- Or: add a `skipConsume bool` parameter/option to `Get()`

This gap affects FILE-06 (one-use download) when combined with FILE-08 (image share page). For non-image one-use files `Info` redirects immediately after `Get()`, but the file is still consumed by the redirect-path `Get()` call. All one-use files are affected by this gap.

---

_Verified: 2026-03-19_
_Verifier: Claude (gsd-verifier)_
