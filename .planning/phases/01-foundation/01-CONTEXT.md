# Phase 1: Foundation - Context

**Gathered:** 2026-03-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Go module setup with SQLite (CGO-free, WAL mode), versioned schema migrations, configuration loading, cryptographic slug generation, and local filesystem storage backend. The binary compiles and runs, applies migrations, loads config, and serves a health endpoint. No user-facing features — this is the skeleton everything builds on.

</domain>

<decisions>
## Implementation Decisions

### Project structure
- Domain-per-package: `internal/paste/`, `internal/file/`, `internal/bucket/` — each contains entity, repository interface, and service
- Shared concerns in `internal/pkg/` — slug generation, expiry logic, storage abstraction
- HTTP handlers in `internal/http/` — separate transport layer that imports domain services, handles routing and request/response encoding
- Entrypoint at `cmd/pbin/main.go` — standard Go binary layout, wires dependencies and starts server
- SQLite repository implementations in each domain package (e.g., `internal/paste/sqlite_repo.go`) or in a shared `internal/storage/` — Claude's discretion on exact placement

### Technology stack (locked from project setup)
- `modernc.org/sqlite` — CGO-free SQLite driver via `database/sql`
- WAL mode enabled at connection time
- `pressly/goose` — embedded schema migrations run at startup via `embed.FS`
- `knadh/koanf/v2` — TOML config file with environment variable overrides
- Go 1.22+ `net/http.ServeMux` — method + wildcard routing, no external router

### Claude's Discretion
- Config structure and default values (port, storage path, max upload size, DB path)
- Slug length, charset, and collision strategy
- File storage layout on disk (flat vs nested, naming scheme)
- Exact SQLite connection pool configuration (two-pool read/write split)
- Migration file naming and organization
- Whether SQLite repo implementations live inside domain packages or in a shared storage package

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

No external specs — requirements fully captured in decisions above. Research files provide technical context:

### Research
- `.planning/research/STACK.md` — Technology recommendations, versions, rationale
- `.planning/research/ARCHITECTURE.md` — DDD package layout, data flow, component boundaries
- `.planning/research/PITFALLS.md` — SQLite WAL pitfalls, multipart temp file leaks, security concerns

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- None — greenfield project, no existing code

### Established Patterns
- None — this phase establishes the foundational patterns

### Integration Points
- None — first phase, no existing system to integrate with

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches. The user wants idiomatic Go with DDD, following established community patterns.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-foundation*
*Context gathered: 2026-03-19*
