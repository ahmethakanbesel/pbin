# Stack Research

**Domain:** Self-hosted single-binary Go file sharing / pastebin
**Researched:** 2026-03-19
**Confidence:** HIGH (all major choices verified against official docs or pkg.go.dev)

---

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.24.x | Language runtime | Latest stable (released Feb 11 2025, patch 1.24.13 as of Feb 2026). Generic type aliases stable, Swiss-table maps, 2-3% CPU improvement. Single binary, zero runtime, cross-compile to any target. |
| `modernc.org/sqlite` | v1.47.0 | SQLite driver — CGO-free | Pure-Go transpilation of SQLite C source. No CGO means `GOOS=linux GOARCH=arm64 go build` just works from any machine. Bundles SQLite 3.51.3. WAL mode, concurrent reads, full SQL support. Battle-tested (used in production since ~2021). |
| `net/http` stdlib | Go 1.24 | HTTP server and router | Go 1.22 added method + wildcard routing to `http.ServeMux` (`GET /files/{id}`). No external router needed; this satisfies all REST API patterns for this project. Fewer dependencies, no upgrade churn. |
| `embed` stdlib | Go 1.16+ | Bundle web UI into binary | `//go:embed static/*` includes all frontend assets at compile time. `embed.FS` is read-only, goroutine-safe, and zero-copy on modern Go. No separate static file server. Combined with `http.FileServer(http.FS(...))` for serving. |
| `log/slog` stdlib | Go 1.21+ | Structured logging | Standard library since 1.21. JSON output, levels, key-value attributes. Zero additional dependencies. Sufficient for an app that logs to stdout and optionally a file. No need for zap or logrus. |

### Database Layer

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `modernc.org/sqlite` | v1.47.0 | SQLite engine | See above. Prefer over `ncruces/go-sqlite3` (wazero-based) because modernc is a simpler dependency graph (pure Go, no WASM runtime), more mature production track record, and implements `database/sql` interface directly. |
| `pressly/goose/v3` | v3.27.0 | Schema migrations | Embed migration SQL files with `go:embed`, run `goose.SetBaseFS()` so migrations run from the binary at startup. Lightweight, no CLI dependency at runtime. Supports up/down migrations, versioned filenames. |
| `sqlc` | v1.30.0 (CLI tool, dev-only) | Type-safe SQL codegen | Write plain SQL queries, sqlc generates Go structs + method bodies. No ORM overhead. Works with SQLite. Configure `engine: sqlite` in `sqlc.yaml`. Generated code uses `database/sql` directly — no runtime dependency. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `knadh/koanf/v2` | v2.3.3 | Configuration loading | Load config from TOML/YAML file + environment variable overrides. Modular: only import the providers you need. Binary size stays small versus Viper (313% smaller per benchmark). Use for: max upload size, storage path, base URL, expiry presets, optional basic auth credentials. |
| `archive/zip` stdlib | Go 1.24 | ZIP bundle download | Built-in package. `zip.NewWriter` + `io.Copy` streams per-file content into the ZIP writer which itself writes to the HTTP response body (`http.ResponseWriter` is an `io.Writer`). No intermediate buffering needed. |
| `mime/multipart` stdlib | Go 1.24 | File upload parsing | `r.ParseMultipartForm(maxMemory)` spills to disk when upload exceeds memory threshold. Set `maxMemory` to e.g. 32MB; files larger than that are written to OS temp directory automatically. Sufficient for this use case — no tus.io needed. |
| `crypto/subtle` stdlib | Go 1.24 | Constant-time comparisons | Use `subtle.ConstantTimeCompare` in basic auth middleware to prevent timing attacks. No external auth library needed. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `sqlc` CLI | Generate type-safe database code from `.sql` files | Run via `go generate ./...`. Install with `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`. Not a runtime dependency — codegen only. |
| `pressly/goose` CLI | Create new migration files with correct sequence numbers | Optional; migrations can also be hand-written. `go install github.com/pressly/goose/v3/cmd/goose@latest`. |
| `golangci-lint` | Static analysis and formatting enforcement | Standard Go linter bundle. Run in CI. `revive`, `staticcheck`, `errcheck` are the most useful linters for this codebase. |
| `air` | Live reload during development | `cosmtrek/air` watches for Go file changes and rebuilds. Avoids restarting manually. Dev-only, not shipped. |

---

## Installation

```bash
# Runtime dependencies (go.mod)
go get modernc.org/sqlite@v1.47.0
go get github.com/pressly/goose/v3@v3.27.0
go get github.com/knadh/koanf/v2@v2.3.3
go get github.com/knadh/koanf/providers/file
go get github.com/knadh/koanf/providers/env
go get github.com/knadh/koanf/parsers/toml

# Dev/codegen tools (not in go.mod runtime, use go.mod tool directives in Go 1.24)
go get -tool github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0
go get -tool github.com/pressly/goose/v3/cmd/goose@v3.27.0
```

> Go 1.24 supports `tool` directives in `go.mod` — use this instead of the old `tools.go` blank-import workaround for tracking dev tools.

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `modernc.org/sqlite` | `ncruces/go-sqlite3` | Only if you specifically need wazero's sandboxing model, or if benchmarks show the WASM-based driver is faster for your specific query pattern. For this project, modernc's simpler dependency graph wins. |
| `modernc.org/sqlite` | `mattn/go-sqlite3` (CGO) | Only if cross-compilation is NOT a requirement and maximum raw performance is critical. mattn is faster in CGO benchmarks but breaks `GOOS`/`GOARCH` cross-compile without a C toolchain. This project's single-binary cross-compile goal rules it out. |
| `pressly/goose` | `golang-migrate/migrate` | golang-migrate is fine but goose's `embed.FS` integration is cleaner and its support for Go-function migrations is useful if you need data transformations alongside schema changes. |
| `koanf/v2` | `spf13/viper` | Viper if you need remote config providers (etcd, Consul) or already have it in your org's standard stack. For a self-hosted single binary with no distributed config needs, koanf is strictly better. |
| `koanf/v2` | `caarlos0/env` | env-only parsing if config is purely environment variables with no file support needed. For a self-hosted app where operators expect a config file, koanf's multi-source support is worth it. |
| `log/slog` | `uber-go/zap` | Only if profiling shows logging is a CPU bottleneck (extremely unlikely for a file-share service). slog is sufficient and adds zero dependencies. |
| `net/http` ServeMux | `go-chi/chi` or `gorilla/mux` | If you need advanced middleware chaining, nested route groups, or URL encoding edge cases. For this project's route count (~15 endpoints), Go 1.22+ ServeMux is sufficient and keeps the dependency list clean. |
| `archive/zip` stdlib | `alexmullins/zip` | Only if you need AES-encrypted ZIPs. Out of scope per PROJECT.md. |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `spf13/viper` | Forcibly lowercases all config keys (breaks convention), pulls 10+ heavy dependencies into the core, produces 3x larger binary than koanf. Known issues with YAML/JSON spec compliance. | `knadh/koanf/v2` |
| `mattn/go-sqlite3` | Requires CGO. Cross-compilation to Linux from macOS requires a C cross-compiler. Breaks the single-binary promise unless you control the build environment tightly. | `modernc.org/sqlite` |
| Any ORM (GORM, ent, bun) | Adds significant abstraction and dependencies for a schema with ~4 tables. GORM's auto-migration is unsafe for production schema management. sqlc + goose gives full SQL control with type safety at zero runtime cost. | `sqlc` (codegen) + `pressly/goose` |
| `gin`, `echo`, `fiber` | PROJECT.md explicitly prohibits web frameworks. Go 1.22 ServeMux covers all routing needs. External frameworks add dependency churn and conflict with the idiomatic Go constraint. | `net/http` stdlib |
| `gorilla/sessions` or JWT libraries | No session state needed. Basic auth is stateless by design — verify credentials on every request using `r.BasicAuth()` + `subtle.ConstantTimeCompare`. No token storage. | `crypto/subtle` stdlib |
| `sirupsen/logrus` | Abandoned maintenance period (2020-2022), now resumed but slog supersedes it as the standard. Logrus has no advantage over slog for new projects. | `log/slog` stdlib |

---

## Stack Patterns by Variant

**If deploying behind a reverse proxy (Caddy, Nginx, Traefik):**
- Set `BEHIND_PROXY=true` in config and read `X-Forwarded-For` / `X-Real-IP` headers for rate limiting or logging
- Trust proxy headers only when this flag is set — avoids spoofing when running exposed directly

**If cross-compiling for ARM (Raspberry Pi, NAS, etc.):**
- `modernc.org/sqlite` compiles cleanly: `GOOS=linux GOARCH=arm64 go build -o pbin ./cmd/pbin`
- No C toolchain needed on the build machine

**If enabling WAL mode for concurrent reads:**
- Execute `PRAGMA journal_mode=WAL;` on first connection open
- Use a single `*sql.DB` pool across the process — `database/sql` manages connection concurrency
- Set `_busy_timeout=5000` in the DSN: `file:pbin.db?_busy_timeout=5000&_journal_mode=WAL`

**If using `go generate` for sqlc:**
- Add `//go:generate sqlc generate` in a `generate.go` file at the package root
- Commit generated code; regenerate only when SQL changes
- CI should verify generated code is up-to-date with `sqlc vet`

---

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `modernc.org/sqlite v1.47.0` | Go 1.19+ | Tested on Go 1.24 |
| `pressly/goose/v3 v3.27.0` | Go 1.21+ | embed.FS support stable since v3.5 |
| `sqlc v1.30.0` | SQLite engine: full support | Set `engine: "sqlite"` in sqlc.yaml |
| `knadh/koanf/v2 v2.3.3` | Go 1.18+ | v2 module path required |
| Go 1.22 ServeMux routing | Go 1.22+ | Pattern syntax is backward-incompatible if upgrading from pre-1.22 (GODEBUG=httpmuxgo121 to restore old behavior) |

---

## Sources

- [modernc.org/sqlite on pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite) — v1.47.0 confirmed (Mar 17 2026), SQLite 3.51.3, CGO-free
- [ncruces/go-sqlite3 on GitHub](https://github.com/ncruces/go-sqlite3) — wazero-based alternative, v0.30.5, production-ready
- [go-sqlite-bench benchmarks](https://github.com/cvilsmeier/go-sqlite-bench) — driver performance comparison data
- [Routing Enhancements for Go 1.22 — official Go blog](https://go.dev/blog/routing-enhancements) — method matching + wildcards in ServeMux confirmed
- [Go 1.24 release notes](https://go.dev/doc/go1.24) — confirmed Feb 2025 release, tool directive in go.mod
- [pressly/goose v3 on pkg.go.dev](https://pkg.go.dev/github.com/pressly/goose/v3) — v3.27.0 (Feb 22 2026), embed.FS support confirmed
- [sqlc documentation](https://docs.sqlc.dev/) — v1.30.0, SQLite engine support confirmed
- [koanf/v2 on pkg.go.dev](https://pkg.go.dev/github.com/knadh/koanf/v2) — v2.3.3 (Feb 23 2026), MIT license
- [Viper vs koanf comparison (ITNEXT)](https://itnext.io/golang-configuration-management-library-viper-vs-koanf-eea60a652a22) — 313% binary size difference documented
- [Go 1.21 slog official blog](https://go.dev/blog/slog) — standard library structured logging confirmed
- [Basic authentication in Go — Alex Edwards](https://www.alexedwards.net/blog/basic-authentication-in-go) — r.BasicAuth() + subtle.ConstantTimeCompare pattern
- [embed package in Go — leapcell.io](https://leapcell.io/blog/embedding-frontend-assets-in-go-binaries-with-embed-package) — embed.FS + http.FileServer pattern confirmed
- [goose + sqlc integration — pressly docs](https://pressly.github.io/goose/blog/2024/goose-sqlc/) — combined workflow confirmed

---
*Stack research for: pbin — self-hosted single-binary Go file sharing / pastebin*
*Researched: 2026-03-19*
