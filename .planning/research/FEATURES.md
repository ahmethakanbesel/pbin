# Feature Research

**Domain:** Self-hosted file sharing / multi-file transfer / pastebin
**Researched:** 2026-03-19
**Confidence:** HIGH (based on direct analysis of linx-server, PsiTransfer, Lenpaste, PrivateBin, 0x0.st, transfer.sh, Hastebin)

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Shareable link on upload | Core purpose of the tool — a URL is the artifact | LOW | Must be immediate, no extra steps |
| File expiry / auto-deletion | Every reference product has it; users need TTL control | LOW | Fixed presets (10min, 1h, 1d, 7d, 30d, never) beat free-form input |
| Configurable max upload size | Operators need to guard storage; users need to know limits | LOW | Config value exposed in UI and API error messages |
| Direct download URL | Must be curl-friendly, no redirect chains | LOW | Content-Disposition header for forced download |
| Raw/direct access to paste content | Every pastebin provides `/raw/` endpoint; tooling depends on it | LOW | Critical for CLI workflows |
| Syntax highlighting in paste view | Hastebin, Lenpaste, linx-server all do this; absence is jarring | MEDIUM | Use a client-side highlighter (highlight.js or Prism) — language auto-detect or user-specified |
| Deletion token / delete URL | Users need a way to remove what they uploaded | LOW | Return a secret token at upload; accept it on DELETE endpoint |
| One-use / burn-after-reading | PsiTransfer, Lenpaste, PrivateBin all offer this; privacy-conscious users require it | LOW | Flag in DB row; delete on first read |
| REST API (upload, retrieve, delete) | All reference products expose APIs; CLI and integration users depend on it | MEDIUM | Consistent JSON responses, documented endpoints |
| Web UI for upload and view | Required for non-technical users; should work without JS for paste view | MEDIUM | File drag-and-drop, paste editor, download page |
| Multi-file upload / bundle download | PsiTransfer defines user expectation for "transfer bucket" use case | MEDIUM | Accept multiple files in one session; offer ZIP download of bundle |
| Password protection on share | PsiTransfer, linx-server, transfer.sh all offer this | LOW | Bcrypt-hash the password; gate download page behind form |
| Paste title field | Lenpaste supports it; absent title is disorienting in link previews | LOW | Optional; render in `<title>` and `<h1>` |
| Instance-level access control | Self-hosters need to lock down uploads to known users | LOW | HTTP Basic Auth header check on write endpoints; read can remain open |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required, but valuable.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Unified three-in-one service | No existing tool combines file share + transfer bucket + pastebin in one binary | LOW | This is the core positioning — lean into it |
| Single-binary, zero-dependency deployment | Most competitors require Node.js, PHP, or separate DB; this runs `./pbin` and done | LOW | Already a constraint; make it prominent in UI and docs |
| ZIP bundle download for transfer buckets | PsiTransfer has this; it's the feature that makes transfer buckets genuinely useful | MEDIUM | Stream a zip on the fly using `archive/zip`; no temp file needed |
| One-time download on individual files (not just pastes) | linx-server and 0x0.st do not offer per-file one-shot; PsiTransfer scopes it to buckets | LOW | Extend the burn-after-reading flag to file shares |
| Fixed expiry presets with never option | Free-form TTL input creates confusion; presets (10min, 1h, 6h, 1d, 7d, 30d, 90d, 1y, never) are faster and predictable | LOW | Render as a segmented picker, not a text field |
| No-JS paste view and raw access | Lenpaste proves this is achievable and valued by privacy-conscious users | LOW | Server-side syntax highlight via Chroma (Go library) for the no-JS case |
| curl-friendly upload API (PUT and POST) | transfer.sh and 0x0.st define the pattern; `curl --upload-file` just works | LOW | Accept both PUT /<filename> and POST multipart/form-data |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Client-side or end-to-end encryption | PrivateBin uses it; some users ask for it | Requires JavaScript, complicates the no-JS goal, adds significant frontend complexity, and makes server-side search/moderation impossible | Make clear that encryption is the operator's responsibility (TLS + disk encryption); document this as a conscious choice |
| Per-user accounts and login | Users want history of their uploads | Requires auth system, session management, user DB schema — multiplies scope; most self-hosters run single-operator instances | Instance-level Basic Auth covers the admin case; anonymous upload with deletion tokens covers the user case |
| OAuth / social login | Lowers friction for multi-user deployments | Adds OAuth provider dependencies, callback URLs, token refresh — all complexity with minimal payoff for a single-binary tool | Optional Basic Auth covers the use case; if multi-user auth is needed, put a reverse proxy (Authelia, Caddy basic_auth) in front |
| Resumable uploads (tus.io) | Large file uploads fail on bad connections | tus.io requires a stateful upload session, additional endpoints, and a JS client — substantial complexity for an edge case | Standard multipart/form-data with a generous timeout and a clear max-size error message covers 95% of uploads |
| Real-time notifications / WebSockets | "Notify me when someone downloads my file" | Server push requires persistent connections or polling — operational complexity with no user-facing payoff in a share-and-forget tool | Return download count in the paste/file metadata API; let users poll if needed |
| URL shortener | Seems like a natural addition | Separate domain of concerns; changes routing logic; 0x0.st bundles it but it muddies the data model | Short IDs (nanoid-style) on generated share URLs already achieve this implicitly |
| Full-text search across pastes | Power users want to find old pastes | Requires indexing, adds operational weight, and encourages hoarding — contrary to ephemeral-first design | Operator can query SQLite directly; not worth building a UI for |
| Thumbnail / media preview generation | Image and video hosts do this | Requires ffmpeg/ImageMagick, CGO or subprocess calls — breaks the single-binary CGO-free goal | Serve the raw file with correct Content-Type; modern browsers preview natively |
| Versioning / edit history | GitHub Gist / Opengist does this | Turns a paste service into a VCS — completely different scope | Out of scope; users who need versioning should use Opengist |

## Feature Dependencies

```
[File Share Upload]
    └──requires──> [Shareable Link Generation]
                       └──requires──> [Unique ID / Slug System]

[Multi-File Transfer Bucket]
    └──requires──> [Shareable Link Generation]
    └──requires──> [File Upload]
    └──enables──>  [ZIP Bundle Download]

[Paste Creation]
    └──requires──> [Shareable Link Generation]
    └──enables──>  [Syntax Highlighting]
    └──enables──>  [Raw Endpoint]

[Password Protection]
    └──requires──> [Unique ID / Slug System]
    └──applies-to──> [File Share] AND [Transfer Bucket]

[One-Use / Burn-After-Reading]
    └──requires──> [Unique ID / Slug System]
    └──applies-to──> [File Share] AND [Paste]

[Expiry]
    └──requires──> [Unique ID / Slug System]
    └──requires──> [Background Cleanup Job]

[Deletion Token]
    └──requires──> [Unique ID / Slug System]

[REST API]
    └──requires──> [File Share Upload]
    └──requires──> [Paste Creation]
    └──requires──> [Transfer Bucket]

[Instance-Level Basic Auth]
    └──wraps──> [REST API write endpoints]
    └──wraps──> [Web UI upload forms]

[ZIP Bundle Download]
    └──requires──> [Multi-File Transfer Bucket]

[Syntax Highlighting]
    └──requires──> [Paste Creation]
    └──enhances──> [Raw Endpoint] (language hint in URL)

[Background Cleanup Job]
    └──required-by──> [Expiry]
    └──required-by──> [One-Use / Burn-After-Reading]
```

### Dependency Notes

- **Unique ID / Slug System is foundational:** Every feature that produces a shareable link depends on it. Implement first.
- **Background Cleanup Job gates Expiry:** Without it, expired rows accumulate. A simple ticker goroutine is sufficient.
- **ZIP Bundle Download requires Transfer Bucket:** Do not generalize to arbitrary multi-file zip — scope to bucket sessions only to keep the data model simple.
- **Password Protection and One-Use are independent flags:** They can coexist on the same share. Design the schema to allow both simultaneously.
- **Basic Auth wraps write paths only:** Read paths (download, view paste) should remain accessible without auth so links can be shared externally.

## MVP Definition

### Launch With (v1)

Minimum viable product — what's needed to validate the concept.

- [ ] File share upload with shareable link, expiry, deletion token — core file sharing
- [ ] Transfer bucket (multi-file session) with shareable link, expiry, password, ZIP download — core transfer use case
- [ ] Paste creation with title, syntax highlighting, expiry, one-use flag, raw endpoint — core pastebin use case
- [ ] Fixed expiry presets (10min, 1h, 6h, 1d, 7d, 30d, 90d, 1y, never) across all three domains
- [ ] One-use / burn-after-reading for files and pastes
- [ ] Password protection for file shares and transfer buckets
- [ ] REST API for all three domains (upload, retrieve, delete)
- [ ] Embedded web UI with upload forms, share pages, paste editor
- [ ] Instance-level Basic Auth gating write endpoints
- [ ] Background expiry cleanup goroutine

### Add After Validation (v1.x)

Features to add once core is working.

- [ ] curl-friendly PUT /<filename> upload endpoint — after confirming CLI users exist
- [ ] One-time download on individual file shares (not just pastes) — after gauging user interest
- [ ] Server-side syntax highlight (Chroma) for no-JS paste view — after confirming no-JS users exist
- [ ] Configurable max paste lifetime / max file size per instance — after first operator feedback

### Future Consideration (v2+)

Features to defer until product-market fit is established.

- [ ] Admin dashboard (view all uploads, force-delete, usage stats) — defer until operators request it
- [ ] Webhook on download (notify uploader) — defer; polling the metadata endpoint covers the need
- [ ] S3 / object storage backend — defer until SQLite + local disk proves insufficient at scale

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Shareable link generation | HIGH | LOW | P1 |
| File share upload + expiry + deletion | HIGH | LOW | P1 |
| Transfer bucket with ZIP download | HIGH | MEDIUM | P1 |
| Paste with syntax highlight + expiry | HIGH | MEDIUM | P1 |
| One-use / burn-after-reading | HIGH | LOW | P1 |
| Password protection | HIGH | LOW | P1 |
| REST API | HIGH | MEDIUM | P1 |
| Web UI (upload + share + paste views) | HIGH | MEDIUM | P1 |
| Instance-level Basic Auth | MEDIUM | LOW | P1 |
| Background expiry cleanup | HIGH | LOW | P1 |
| curl PUT /<filename> API | MEDIUM | LOW | P2 |
| Server-side syntax highlight (no-JS) | MEDIUM | LOW | P2 |
| One-time download on file shares | MEDIUM | LOW | P2 |
| Admin dashboard | LOW | MEDIUM | P3 |
| S3 storage backend | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for launch
- P2: Should have, add when possible
- P3: Nice to have, future consideration

## Competitor Feature Analysis

| Feature | linx-server | PsiTransfer | Lenpaste | PrivateBin | 0x0.st | transfer.sh | Hastebin | Our Approach |
|---------|-------------|-------------|----------|------------|--------|-------------|---------|--------------|
| File sharing | Yes | Yes | No | No | Yes | Yes | No | Yes (domain 1) |
| Multi-file bucket | No | Yes | No | No | No | No | No | Yes (domain 2) |
| Pastebin | Code display only | No | Yes | Yes | No | No | Yes | Yes (domain 3) |
| Expiry | Yes | Yes | Yes | Yes | Size-based | Yes (headers) | No | Yes, fixed presets |
| One-use / burn-after-read | No | Yes (per bucket) | Yes | Yes | No | No | No | Yes (files + pastes) |
| Password protection | Yes | Yes | No | Yes | No | No | No | Yes (files + buckets) |
| Syntax highlighting | Yes (display) | No | Yes | No | No | No | Yes | Yes (server-side + client) |
| REST API | Yes | No | Yes | No | Yes (curl) | Yes (curl) | Yes | Yes |
| No-JS support | No | No | Yes | No | Yes (curl) | Yes (curl) | No | Yes (paste view) |
| Deletion token | Yes | No | No | No | No | Yes | No | Yes |
| ZIP bundle download | No | Yes | No | No | No | No | No | Yes |
| Single binary | No (Go, but deps) | No (Node.js) | No (Go + Postgres) | No (PHP) | No (Python) | No (Go + deps) | No (Node.js) | Yes |
| SQLite (no external DB) | No | No | No | No | No | No | No | Yes |

## Sources

- [linx-server GitHub (andreimarcu)](https://github.com/andreimarcu/linx-server)
- [PsiTransfer GitHub (psi-4ward)](https://github.com/psi-4ward/psitransfer)
- [Lenpaste review — noted.lol](https://noted.lol/lenpaste/)
- [PrivateBin GitHub](https://github.com/PrivateBin/PrivateBin)
- [transfer.sh GitHub (dutchcoders)](https://github.com/dutchcoders/transfer.sh)
- [0x0.st self-hosting guide — orhun.dev](https://blog.orhun.dev/no-bullshit-file-hosting/)
- [awesome-selfhosted pastebins](https://awesome-selfhosted.net/tags/pastebins.html)
- [Hastebin documentation — Toptal](https://www.toptal.com/developers/hastebin/documentation)

---
*Feature research for: self-hosted file sharing + multi-file transfer + pastebin (pbin)*
*Researched: 2026-03-19*
