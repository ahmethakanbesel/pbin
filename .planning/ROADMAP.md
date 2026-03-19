# Roadmap: pbin

## Overview

pbin is built bottom-up: the foundation establishes the Go module, SQLite with WAL mode, migrations, slug generation, and local file storage — everything every feature depends on. File sharing ships next as the simplest domain that validates the full upload-store-serve path. Transfer buckets and paste follow, sharing the same storage primitives and adding the two highest-risk implementation details: streaming ZIP downloads and atomic one-use semantics. The final phase wires up the REST API surface, embeds the web UI, adds Basic Auth middleware and the background expiry cleanup worker, and ships the single binary.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation** - Go module, SQLite/WAL, migrations, slug generation, config, local file storage (completed 2026-03-19)
- [x] **Phase 2: File Sharing** - Upload, download, expiry, deletion tokens, password protection, one-use, image embed links (completed 2026-03-19)
- [x] **Phase 3: Buckets and Paste** - Multi-file transfer buckets with ZIP download; pastebin with syntax highlighting and one-use (completed 2026-03-19)
- [ ] **Phase 4: API, Web UI, and Delivery** - Finalized REST API, embedded web UI, Basic Auth, expiry cleanup worker, single-binary build

## Phase Details

### Phase 1: Foundation
**Goal**: The project compiles and runs as a single binary with a working SQLite database, versioned schema migrations, configuration loading, unique slug generation, and a local filesystem storage backend — ready for domain feature work.
**Depends on**: Nothing (first phase)
**Requirements**: INFRA-06
**Success Criteria** (what must be TRUE):
  1. Running `go build ./cmd/pbin` produces a single binary with no CGO and no external runtime dependencies
  2. The binary starts, applies all pending goose migrations automatically, and serves a health endpoint without error
  3. Config values (port, max upload size, storage path, Basic Auth credentials) load from a TOML file with environment variable overrides
  4. The slug generator produces cryptographically random, URL-safe identifiers with no collisions under repeated calls
**Plans**: 3 plans

Plans:
- [ ] 01-01-PLAN.md — Go module init, config loading (koanf/v2 + TOML + env), slug generator (crypto/rand)
- [ ] 01-02-PLAN.md — SQLite two-pool WAL setup + goose migrations + full schema + LocalFS filestore backend
- [ ] 01-03-PLAN.md — Domain entities with constructor validation, repository interfaces, health endpoint, main.go binary wiring

### Phase 2: File Sharing
**Goal**: Users can upload files, receive shareable links with configurable expiry, download files via direct URL, protect shares with passwords, mark files as one-time download, and get embed codes for images.
**Depends on**: Phase 1
**Requirements**: FILE-01, FILE-02, FILE-03, FILE-04, FILE-05, FILE-06, FILE-07, FILE-08
**Success Criteria** (what must be TRUE):
  1. User uploads a file and immediately receives a shareable URL and a deletion token
  2. User downloads a file via its direct URL with a browser and with `curl` (Content-Disposition forces attachment; no inline serving of user content)
  3. User selects an expiry preset (10min, 1h, 6h, 1d, 7d, 30d, 90d, 1y, never) and the file is inaccessible after that time
  4. User sends the deletion token to delete their file before expiry; the file and its on-disk data are both removed
  5. User sets a password on a file share; downloading requires the correct password or the request is rejected
  6. User marks a file as one-time download; the second download attempt after the first successful delivery returns 410 Gone
  7. User uploads a PNG/JPEG/GIF/WebP image and receives direct embed link plus ready-to-copy HTML, BBCode, and Markdown embed codes
**Plans**: 4 plans

Plans:
- [ ] 02-01-PLAN.md — Migration 002 (delete_secret column), File entity extension (DeleteSecret + IsImage), FileService (Upload/Get/Delete)
- [ ] 02-02-PLAN.md — SQLite file repository (file.Repository implementation), ExpiresAt on File entity
- [ ] 02-03-PLAN.md — HTTP handlers (Upload POST /api/upload, Serve GET /{slug}, Delete GET /delete/{slug}/{secret})
- [ ] 02-04-PLAN.md — main.go wiring + end-to-end human verification

### Phase 3: Buckets and Paste
**Goal**: Users can upload multiple files to a transfer bucket and download them as a ZIP bundle, and users can create syntax-highlighted pastes with raw access — both supporting expiry, password protection, and one-use semantics.
**Depends on**: Phase 2
**Requirements**: BUCK-01, BUCK-02, BUCK-03, BUCK-04, BUCK-05, PASTE-01, PASTE-02, PASTE-03, PASTE-04, PASTE-05
**Success Criteria** (what must be TRUE):
  1. User uploads multiple files to a bucket and receives a shareable bucket URL; all files are accessible from that URL
  2. User downloads all files in a bucket as a ZIP bundle that streams directly to the browser without buffering in server memory
  3. User marks a bucket as one-time download; the second download attempt returns 410 Gone (enforced atomically, no TOCTOU race)
  4. User creates a paste with optional title and language selection and receives a shareable URL with syntax highlighting rendered in the view
  5. User accesses `/raw/{id}` and receives the paste content as plain text (curl-friendly)
  6. User marks a paste as one-use; the second view attempt after the first successful read returns 410 Gone
**Plans**: 6 plans

Plans:
- [ ] 03-01-PLAN.md — Migration 003: add delete_secret column to buckets and pastes tables
- [ ] 03-02-PLAN.md — Bucket domain: extend entity, BucketService, bucketRepo (SQLite)
- [ ] 03-03-PLAN.md — Paste domain: extend entity, PasteService, pasteRepo (SQLite)
- [ ] 03-04-PLAN.md — BucketHandler: Upload, View, DownloadZIP, Delete
- [ ] 03-05-PLAN.md — PasteHandler: Create, View (highlight.js), Raw, Delete
- [ ] 03-06-PLAN.md — main.go wiring, slug dispatch, end-to-end human verification

### Phase 4: API, Web UI, and Delivery
**Goal**: All operations are available through a consistent REST API, an embedded web UI covers upload forms and share/paste views for non-technical users, Basic Auth gates write endpoints, and a background worker automatically deletes expired records and their on-disk data.
**Depends on**: Phase 3
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04, INFRA-05
**Success Criteria** (what must be TRUE):
  1. All create, retrieve, and delete operations for files, buckets, and pastes are accessible via REST API with JSON responses and no web UI required
  2. The embedded web UI (upload form, share page, paste editor, bucket view) is served from the binary with no separate frontend server or build step
  3. When Basic Auth is configured, unauthenticated requests to write endpoints (upload, create) are rejected with 401; read endpoints remain public
  4. Expired files, buckets, and pastes are automatically deleted (DB row and on-disk data) by a background worker; the worker restarts itself on panic and does not leak open SQL rows
  5. Admin sets max upload size in config and uploads exceeding that limit are rejected before the request body is fully read
**Plans**: 4 plans

Plans:
- [ ] 04-01-PLAN.md — Basic Auth middleware (internal/middleware/auth.go) + expiry cleanup worker (internal/worker/cleanup.go)
- [ ] 04-02-PLAN.md — Web UI form handlers: Home (/), Paste (/paste), Bucket (/bucket) in internal/handler/ui.go
- [ ] 04-03-PLAN.md — main.go wiring: auth middleware on write endpoints, cleanup worker lifecycle, UI route registration
- [ ] 04-04-PLAN.md — End-to-end automated smoke test + human browser verification

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 3/3 | Complete   | 2026-03-19 |
| 2. File Sharing | 4/4 | Complete   | 2026-03-19 |
| 3. Buckets and Paste | 6/6 | Complete   | 2026-03-19 |
| 4. API, Web UI, and Delivery | 0/4 | Not started | - |
