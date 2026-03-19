# Requirements: pbin

**Defined:** 2026-03-19
**Core Value:** Users can share files, transfer file bundles, and paste text through a single self-hosted service that runs from one binary with zero external dependencies.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### File Sharing

- [x] **FILE-01**: User can upload a file and receive a shareable link
- [x] **FILE-02**: User can download a file via direct URL (curl-friendly)
- [x] **FILE-03**: User can set expiry on upload (10min, 1h, 6h, 1d, 7d, 30d, 90d, 1y, never)
- [x] **FILE-04**: User receives a deletion token on upload and can delete the file with it
- [x] **FILE-05**: User can password-protect a file share
- [x] **FILE-06**: User can mark a file as one-time download (auto-deletes after first download)
- [x] **FILE-07**: User can get a direct embed link for validated image files (extension + magic byte validation)
- [x] **FILE-08**: User is shown ready-to-copy HTML, BBCode, and Markdown embed codes for image uploads

### Transfer Buckets

- [x] **BUCK-01**: User can upload multiple files to a single bucket and receive a shareable link
- [ ] **BUCK-02**: User can set expiry on a bucket (same presets as file sharing)
- [ ] **BUCK-03**: User can download all files in a bucket as a ZIP bundle
- [x] **BUCK-04**: User can password-protect a transfer bucket
- [x] **BUCK-05**: User can mark a bucket as one-time download (auto-deletes after first download)

### Pastebin

- [x] **PASTE-01**: User can create a text paste with optional title and receive a shareable link
- [ ] **PASTE-02**: User can view a paste with syntax highlighting (language selectable)
- [ ] **PASTE-03**: User can access raw paste content via `/raw/{id}` endpoint
- [x] **PASTE-04**: User can set expiry on a paste (same presets)
- [x] **PASTE-05**: User can mark a paste as one-use (auto-deletes after first view)

### Infrastructure

- [ ] **INFRA-01**: All operations available via REST API with JSON responses
- [ ] **INFRA-02**: Embedded web UI served from the binary (upload forms, share pages, paste editor)
- [ ] **INFRA-03**: Optional instance-level Basic Auth gating write endpoints
- [ ] **INFRA-04**: Background worker automatically cleans up expired files, buckets, and pastes
- [ ] **INFRA-05**: Admin can configure max upload size
- [x] **INFRA-06**: Application runs as a single binary with SQLite (CGO-free), zero external dependencies

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### CLI & API Enhancements

- **CLI-01**: curl-friendly PUT upload (`curl --upload-file`)
- **CLI-02**: Server-side syntax highlighting via Chroma for no-JS paste view

### Administration

- **ADMIN-01**: Admin dashboard (view all uploads, force-delete, usage stats)
- **ADMIN-02**: Webhook notification on file download

### Storage

- **STOR-01**: S3 / object storage backend option

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Client-side / E2E encryption | Breaks no-JS goal, complicates frontend, makes moderation impossible |
| Per-user accounts / OAuth | Multiplies scope; instance-level Basic Auth covers the use case |
| Resumable uploads (tus.io) | Substantial complexity for edge case; standard multipart covers 95% |
| Real-time notifications / WebSocket | No user-facing payoff in a share-and-forget tool |
| URL shortener | Short IDs already achieve this implicitly |
| Full-text search | Operators can query SQLite directly |
| Thumbnail / media preview generation | Requires ffmpeg/CGO, breaks single-binary goal |
| Versioning / edit history | Different scope entirely (use Opengist) |
| Mobile app | Web UI + API covers all clients |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| FILE-01 | Phase 2 | Complete |
| FILE-02 | Phase 2 | Complete |
| FILE-03 | Phase 2 | Complete |
| FILE-04 | Phase 2 | Complete |
| FILE-05 | Phase 2 | Complete |
| FILE-06 | Phase 2 | Complete |
| FILE-07 | Phase 2 | Complete |
| FILE-08 | Phase 2 | Complete |
| BUCK-01 | Phase 3 | Complete |
| BUCK-02 | Phase 3 | Pending |
| BUCK-03 | Phase 3 | Pending |
| BUCK-04 | Phase 3 | Complete |
| BUCK-05 | Phase 3 | Complete |
| PASTE-01 | Phase 3 | Complete |
| PASTE-02 | Phase 3 | Pending |
| PASTE-03 | Phase 3 | Pending |
| PASTE-04 | Phase 3 | Complete |
| PASTE-05 | Phase 3 | Complete |
| INFRA-01 | Phase 4 | Pending |
| INFRA-02 | Phase 4 | Pending |
| INFRA-03 | Phase 4 | Pending |
| INFRA-04 | Phase 4 | Pending |
| INFRA-05 | Phase 4 | Pending |
| INFRA-06 | Phase 1 | Complete |

**Coverage:**
- v1 requirements: 24 total
- Mapped to phases: 24
- Unmapped: 0

---
*Requirements defined: 2026-03-19*
*Last updated: 2026-03-19 after roadmap creation*
