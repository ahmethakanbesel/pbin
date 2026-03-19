# pbin

## What This Is

A self-hosted, single-binary Go application that combines file sharing (LinxShare-style), multi-file transfer buckets (PsiTransfer-style), and a pastebin service into one unified platform. It uses SQLite for storage, serves both an API and embedded web UI, and follows domain-driven design with idiomatic Go.

## Core Value

Users can share files, transfer file bundles, and paste text through a single self-hosted service that runs from one binary with zero external dependencies.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] File sharing with shareable links and configurable expiry
- [ ] Multi-file transfer buckets with password protection and one-time download support
- [ ] Pastebin with syntax highlighting and expiry
- [ ] Embedded web UI served from the binary (no separate frontend server)
- [ ] REST API for all operations
- [ ] SQLite backend using a CGO-free library
- [ ] Single binary distribution
- [ ] Optional basic auth for instance-level access control
- [ ] Fixed expiry presets (10min, 1h, 6h, 1d, 7d, 30d, 90d, 1y, never)
- [ ] ZIP bundle download for transfer buckets
- [ ] Configurable max upload size
- [ ] One-use pastes (auto-delete after first view)
- [ ] Domain-driven design, idiomatic Go

### Out of Scope

- Client-side or server-side encryption — keep it simple, plaintext storage
- Resumable uploads (tus.io) — standard multipart uploads cover the use case
- OAuth / per-user API keys — optional basic auth is sufficient
- Mobile app — web UI + API covers all clients
- Real-time features (WebSocket notifications, live progress) — not needed for v1
- Web framework — use Go stdlib net/http

## Context

Inspired by three existing projects:
- **linx-server** (via LinxShare client): Self-hosted file sharing with expiry and shareable links
- **PsiTransfer**: Bucket-based multi-file transfer with password protection, one-time downloads, no auth required
- **Lenpaste** (p.dokuz.gen.tr): Pastebin with syntax highlighting, expiry presets, one-use pastes, optional basic auth

The goal is to unify these three capabilities into a single Go binary backed by SQLite, eliminating the need to run multiple services.

## Constraints

- **Language**: Go — single binary, no runtime dependencies
- **Database**: SQLite via CGO-free library (e.g., modernc.org/sqlite or ncruces/go-sqlite3)
- **Architecture**: Domain-driven design, idiomatic Go patterns
- **No web framework**: Use Go stdlib `net/http` only
- **Frontend**: Embedded in binary via `embed` package, no separate build step required for server
- **Deployment**: Single binary + SQLite file, zero external services

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| SQLite over Postgres/MySQL | Single-file DB matches single-binary goal, zero ops | — Pending |
| CGO-free SQLite | Enables cross-compilation, simpler builds | — Pending |
| No web framework | Idiomatic Go, fewer dependencies, stdlib is capable | — Pending |
| Standard uploads over tus.io | Simpler implementation, covers most use cases | — Pending |
| Optional basic auth over API keys | Simpler model, instance-level control | — Pending |
| Fixed expiry presets over free-form | Better UX, predictable cleanup | — Pending |

---
*Last updated: 2026-03-19 after initialization*
