# pbin

## What This Is

A self-hosted, single-binary Go application that combines file sharing, multi-file transfer buckets, and a pastebin service into one unified platform. It uses SQLite for storage, serves both a REST API and embedded web UI with drag-and-drop uploads, and follows domain-driven design with idiomatic Go. Runs as `./pbin` with zero external dependencies.

## Core Value

Users can share files, transfer file bundles, and paste text through a single self-hosted service that runs from one binary with zero external dependencies.

## Requirements

### Validated

- ✓ SQLite backend using a CGO-free library — v1.0
- ✓ Single binary distribution — v1.0
- ✓ Domain-driven design, idiomatic Go — v1.0
- ✓ File sharing with shareable links and configurable expiry — v1.0
- ✓ Fixed expiry presets (10min, 1h, 6h, 1d, 7d, 30d, 90d, 1y, never) — v1.0
- ✓ Configurable max upload size — v1.0
- ✓ Multi-file transfer buckets with password protection and one-time download — v1.0
- ✓ Pastebin with syntax highlighting and expiry — v1.0
- ✓ Embedded web UI served from the binary — v1.0
- ✓ REST API for all operations — v1.0
- ✓ Optional basic auth for instance-level access control — v1.0
- ✓ ZIP bundle download for transfer buckets — v1.0
- ✓ One-use pastes (auto-delete after first view) — v1.0
- ✓ Image embed codes (HTML, BBCode, Markdown, Direct link) — v1.0
- ✓ Background expiry cleanup worker — v1.0

### Active

(None — v1.0 shipped. Define next milestone with `/gsd:new-milestone`.)

### Out of Scope

- Client-side or server-side encryption — keep it simple, plaintext storage
- Resumable uploads (tus.io) — standard multipart uploads cover the use case
- OAuth / per-user API keys — optional basic auth is sufficient
- Mobile app — web UI + API covers all clients
- Real-time features (WebSocket notifications, live progress) — not needed
- Web framework — Go stdlib net/http is sufficient
- SVG uploads — XSS risk, deliberately excluded

## Context

Shipped v1.0 with 5,262 lines of Go across 17 plans in 4 phases.

Tech stack: Go 1.25, modernc.org/sqlite (CGO-free), pressly/goose (migrations), knadh/koanf (config), Go stdlib net/http (routing), Pico CSS + highlight.js (CDN).

Architecture: Domain-driven design with domain-per-package (`internal/domain/{file,bucket,paste}`), repository interfaces in domain packages, SQLite implementations in `internal/storage/`, HTTP handlers in `internal/handler/`, middleware in `internal/middleware/`.

Inspired by linx-server, PsiTransfer, and Lenpaste — unified into a single binary.

## Constraints

- **Language**: Go — single binary, no runtime dependencies
- **Database**: SQLite via modernc.org/sqlite (CGO-free, WAL mode, two-pool)
- **Architecture**: Domain-driven design, idiomatic Go patterns
- **No web framework**: Go stdlib `net/http` only
- **Frontend**: Pico CSS + highlight.js from CDN, inline HTML via fmt.Fprintf
- **Deployment**: Single binary + SQLite file, zero external services

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| SQLite over Postgres/MySQL | Single-file DB matches single-binary goal, zero ops | ✓ Good |
| CGO-free SQLite (modernc.org/sqlite) | Enables cross-compilation, simpler builds | ✓ Good |
| No web framework | Idiomatic Go, fewer dependencies, stdlib is capable | ✓ Good |
| Standard uploads over tus.io | Simpler implementation, covers most use cases | ✓ Good |
| Optional basic auth over API keys | Simpler model, instance-level control | ✓ Good |
| Fixed expiry presets over free-form | Better UX, predictable cleanup | ✓ Good |
| Two-pool SQLite (1 write, N read) | Prevents writer contention with WAL mode | ✓ Good |
| goose embedded migrations | Schema versioning from day one, runs at startup | ✓ Good |
| Slug-based delete secrets | No separate token field, timing-safe comparison | ✓ Good |
| highlight.js from CDN | No Go dependency, client-side highlighting | ✓ Good |
| Pico CSS classless framework | Clean defaults, minimal effort | ✓ Good |
| Catch-all GET / dispatcher | Resolved Go 1.22 mux pattern conflicts | ✓ Good |
| /{slug}/raw over /raw/{slug} | Avoid mux pattern conflicts | ✓ Good |

---
*Last updated: 2026-03-20 after v1.0 milestone*
