# Architecture Research

**Domain:** Self-hosted file sharing + pastebin (Go, single binary, SQLite, embedded UI)
**Researched:** 2026-03-19
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Delivery Layer                            │
│  (cmd/pbin — main.go, server bootstrap, signal handling)         │
├──────────────┬──────────────────┬──────────────────┬────────────┤
│  HTTP Mux    │  Static Files    │  Auth Middleware  │  CORS /    │
│  (stdlib     │  (embed.FS →     │  (basic auth,     │  Security  │
│   ServeMux)  │   http.FileServer│   middleware chain│  headers)  │
├──────────────┴──────────────────┴──────────────────┴────────────┤
│                        Handler Layer                             │
│  (internal/handler)                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ FileHandler  │  │BucketHandler │  │PasteHandler  │           │
│  │ (upload,     │  │ (create,     │  │ (create,     │           │
│  │  download,   │  │  upload file,│  │  view, raw,  │           │
│  │  delete,     │  │  download,   │  │  delete)     │           │
│  │  info)       │  │  ZIP, delete)│  │              │           │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘           │
├─────────┴─────────────────┴─────────────────┴───────────────────┤
│                        Service Layer                             │
│  (internal/file, internal/bucket, internal/paste)               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ FileService  │  │BucketService │  │PasteService  │           │
│  │ (business    │  │ (business    │  │ (business    │           │
│  │  rules,      │  │  rules,      │  │  rules,      │           │
│  │  slug gen,   │  │  ZIP build,  │  │  one-use     │           │
│  │  expiry)     │  │  password    │  │  logic,      │           │
│  │              │  │  check)      │  │  expiry)     │           │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘           │
├─────────┴─────────────────┴─────────────────┴───────────────────┤
│                     Domain / Port Layer                          │
│  (interfaces defined in each domain package)                     │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │ FileRepository  BucketRepository  PasteRepository        │    │
│  │ StorageBackend  (interfaces only — no implementation)    │    │
│  └──────────────────────────────────────────────────────────┘    │
├──────────────────────────────────────────────────────────────────┤
│                     Infrastructure Layer                         │
│  (internal/storage, internal/db)                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │  SQLiteRepo  │  │  LocalFS     │  │  Cleanup     │           │
│  │  (metadata,  │  │  (file bytes │  │  Worker      │           │
│  │   pastes,    │  │   on disk)   │  │  (goroutine  │           │
│  │   buckets)   │  │              │  │   + ticker)  │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
├──────────────────────────────────────────────────────────────────┤
│                        Config Layer                              │
│  (internal/config — env vars, flags, defaults)                   │
└──────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Communicates With |
|-----------|----------------|-------------------|
| `cmd/pbin/main.go` | Wire dependencies, start HTTP server, handle OS signals | Config, all services, DB, storage backend |
| `internal/handler` | Decode HTTP request → call service → encode response | Service layer only |
| `internal/file` | File domain: slug generation, expiry validation, one-use flag logic | FileRepository (interface), StorageBackend (interface) |
| `internal/bucket` | Bucket domain: password hashing, ZIP streaming, multi-file logic | BucketRepository (interface), StorageBackend (interface) |
| `internal/paste` | Paste domain: one-use logic, syntax lang validation, expiry | PasteRepository (interface) |
| `internal/storage` | SQLite implementations of all repository interfaces | SQLite DB pool |
| `internal/filestore` | Local filesystem implementation of StorageBackend | OS filesystem |
| `internal/cleanup` | Background goroutine: DELETE expired rows + orphaned files periodically | FileRepository, BucketRepository, PasteRepository |
| `internal/config` | Parse config from flags/env, expose typed Config struct | Nothing (read-only) |
| `web/` | Embedded HTML/CSS/JS templates served via embed.FS | Delivered by main HTTP mux |

## Recommended Project Structure

```
pbin/
├── cmd/
│   └── pbin/
│       └── main.go              # Wire-up only: config, DB, services, routes, listen
├── internal/
│   ├── config/
│   │   └── config.go            # Config struct, Parse() from flags+env
│   ├── domain/
│   │   ├── file/
│   │   │   ├── file.go          # File entity, value objects (Slug, Expiry), constructor
│   │   │   ├── repository.go    # FileRepository interface
│   │   │   └── service.go       # FileService: create, get, delete, expiry logic
│   │   ├── bucket/
│   │   │   ├── bucket.go        # Bucket + BucketFile entities, password hashing
│   │   │   ├── repository.go    # BucketRepository interface
│   │   │   └── service.go       # BucketService: create, add file, ZIP, one-time check
│   │   └── paste/
│   │       ├── paste.go         # Paste entity, lang validation, one-use flag
│   │       ├── repository.go    # PasteRepository interface
│   │       └── service.go       # PasteService: create, get (with one-use delete), expiry
│   ├── storage/
│   │   ├── db.go                # Open SQLite, run migrations
│   │   ├── file_repo.go         # FileRepository implementation
│   │   ├── bucket_repo.go       # BucketRepository implementation
│   │   ├── paste_repo.go        # PasteRepository implementation
│   │   └── migrations/
│   │       └── 001_init.sql     # Schema: files, buckets, bucket_files, pastes tables
│   ├── filestore/
│   │   ├── store.go             # StorageBackend interface (Write, Read, Delete, Exists)
│   │   └── local.go             # LocalFS implementation — writes to configurable dir
│   ├── handler/
│   │   ├── file.go              # HTTP handlers for file upload/download/delete/info
│   │   ├── bucket.go            # HTTP handlers for bucket CRUD + ZIP download
│   │   ├── paste.go             # HTTP handlers for paste create/view/raw/delete
│   │   ├── middleware.go        # Auth middleware, request logging, size limit
│   │   └── router.go            # Register all routes on *http.ServeMux
│   ├── cleanup/
│   │   └── worker.go            # Background goroutine: ticker-based expiry sweep
│   └── slug/
│       └── slug.go              # Cryptographically random slug generation (shared)
├── web/
│   ├── static/                  # CSS, JS, favicon
│   │   └── ...
│   ├── templates/               # Go html/template files
│   │   ├── file_view.html
│   │   ├── bucket_view.html
│   │   └── paste_view.html
│   └── embed.go                 # //go:embed directives, exports web.FS
└── go.mod
```

### Structure Rationale

- **`internal/domain/{file,bucket,paste}/`**: Each domain is a self-contained package with its entity, repository interface, and service. The interface lives next to the entity it describes — this is the idiomatic Go DDD pattern (interfaces near the consumer, not the implementor).
- **`internal/storage/`**: All SQLite repository implementations together, sharing a single `*sql.DB` pool. Grouped by infrastructure concern, not by domain.
- **`internal/filestore/`**: Separate from storage because bytes-on-disk is a different concern from metadata-in-SQLite. This boundary makes it easy to add an S3 backend later without touching SQLite code.
- **`internal/handler/`**: HTTP concern only. Handlers decode requests, call services, encode responses. No business logic.
- **`internal/cleanup/`**: Isolated because it crosses all three domains and is a lifecycle concern, not a request-handling concern.
- **`internal/slug/`**: Shared utility. Slug generation is the same algorithm across files, buckets, and pastes.
- **`web/`**: Outside `internal/` so the embed directive can reference it from `cmd/pbin/main.go` if needed, or via `web/embed.go` exporting the FS.
- **`cmd/pbin/main.go`**: Pure wiring only — create config, open DB, instantiate repositories, inject into services, inject services into handlers, register routes, start cleanup worker, call `http.ListenAndServe`.

## Architectural Patterns

### Pattern 1: Repository Interface in Domain Package

**What:** Define the repository interface in the same package as the domain entity it serves, not in the infrastructure package. The infrastructure package imports the domain package to implement the interface.

**When to use:** Always in this project. Dependency direction flows inward: infrastructure → domain, handler → domain.

**Trade-offs:** Domain package has no outbound dependencies (no SQL, no HTTP). Slightly more files. Worth it for testability.

**Example:**
```go
// internal/domain/paste/repository.go
package paste

import "context"

// Repository is satisfied by any persistence implementation.
// Defined here so the domain has no knowledge of storage details.
type Repository interface {
    Create(ctx context.Context, p Paste) error
    GetBySlug(ctx context.Context, slug string) (Paste, error)
    Delete(ctx context.Context, slug string) error
    ListExpired(ctx context.Context) ([]Paste, error)
}
```

```go
// internal/storage/paste_repo.go
package storage

import "git.example.com/pbin/internal/domain/paste"

type pasteSQLite struct{ db *sql.DB }

func (r *pasteSQLite) Create(ctx context.Context, p paste.Paste) error { ... }
// implements paste.Repository
```

### Pattern 2: Constructor Validation (Invariants at Creation)

**What:** Validate all business rules in the entity constructor. Never expose a mutable zero-value that can violate invariants.

**When to use:** All three domain entities (File, Bucket, Paste). Especially important for Expiry (must be a preset value) and Slug (must be non-empty, URL-safe).

**Trade-offs:** Slightly more boilerplate constructors. Eliminates scattered validation across handlers and services.

**Example:**
```go
// internal/domain/paste/paste.go
package paste

import "errors"

var ErrInvalidExpiry = errors.New("expiry must be a valid preset")

var validExpiries = map[string]bool{
    "10m": true, "1h": true, "6h": true, "1d": true,
    "7d": true, "30d": true, "90d": true, "1y": true, "never": true,
}

type Paste struct {
    Slug    string
    Content string
    Lang    string
    Expiry  string
    OneUse  bool
}

func New(slug, content, lang, expiry string, oneUse bool) (Paste, error) {
    if !validExpiries[expiry] {
        return Paste{}, ErrInvalidExpiry
    }
    // ... other validation
    return Paste{Slug: slug, Content: content, Lang: lang, Expiry: expiry, OneUse: oneUse}, nil
}
```

### Pattern 3: Background Cleanup Goroutine with Stop Channel

**What:** A single background goroutine sweeps all three domains for expired records at a configurable interval. Uses a stop channel for clean shutdown.

**When to use:** Required for expiry to work. One worker handles all domains to avoid coordination complexity.

**Trade-offs:** Expiry is eventually consistent within the tick interval (acceptable). Stop channel ensures clean shutdown on SIGTERM.

**Example:**
```go
// internal/cleanup/worker.go
package cleanup

type Worker struct {
    files   file.Repository
    buckets bucket.Repository
    pastes  paste.Repository
    stop    chan struct{}
}

func (w *Worker) Start(interval time.Duration) {
    ticker := time.NewTicker(interval)
    go func() {
        for {
            select {
            case <-ticker.C:
                w.sweep(context.Background())
            case <-w.stop:
                ticker.Stop()
                return
            }
        }
    }()
}

func (w *Worker) Stop() { close(w.stop) }
```

### Pattern 4: Embedded Web UI via embed.FS

**What:** Use `//go:embed` to bundle all HTML templates and static assets into the binary at compile time. Serve via `http.FileServer(http.FS(sub))`.

**When to use:** Always — this is what enables single-binary distribution.

**Trade-offs:** Binary size grows with UI assets. No separate frontend build step needed to run the server. Development can use `os.DirFS` fallback.

**Example:**
```go
// web/embed.go
package web

import "embed"

//go:embed static templates
var FS embed.FS
```

```go
// cmd/pbin/main.go
sub, _ := fs.Sub(web.FS, "static")
mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
```

## Data Flow

### Upload File Request

```
HTTP POST /api/v1/files
    |
    v
middleware.go (auth check, size limit via http.MaxBytesReader)
    |
    v
handler/file.go — ParseMultipartForm, extract file + metadata
    |
    v
domain/file/service.go — generate slug, validate expiry preset, construct File entity
    |         |
    |         v
    |     filestore/local.go — io.Copy bytes to disk at {storage_dir}/{slug}
    |
    v
storage/file_repo.go — INSERT INTO files (slug, filename, size, mime, expiry_at, created_at)
    |
    v
handler/file.go — JSON response {slug, url, expiry}
```

### One-Use Paste View Request

```
HTTP GET /p/{slug}
    |
    v
middleware.go (auth check if instance auth enabled)
    |
    v
handler/paste.go — extract slug from URL
    |
    v
domain/paste/service.go — GetBySlug → if OneUse: call Delete immediately after fetch
    |
    v
storage/paste_repo.go — SELECT ... + conditional DELETE (in same tx for atomicity)
    |
    v
handler/paste.go — render template or JSON response
```

### Bucket ZIP Download

```
HTTP GET /b/{slug}/download
    |
    v
handler/bucket.go — extract slug, optional password from query/header
    |
    v
domain/bucket/service.go — GetBucket → verify password hash → list BucketFiles
    |
    v
handler/bucket.go — set Content-Type: application/zip, Content-Disposition header
    |
    v
archive/zip.Writer streaming into http.ResponseWriter
    |
    v
filestore/local.go — Read each file → stream into zip entry (no full buffering)
```

### Expiry Cleanup Sweep

```
cleanup/worker.go (ticker fires, e.g., every 5 minutes)
    |
    +--- storage/paste_repo.go  — DELETE FROM pastes WHERE expires_at < NOW() AND expires_at != 0
    |
    +--- storage/file_repo.go   — SELECT slug FROM files WHERE expires_at < NOW() AND expires_at != 0
    |       |
    |       v
    |     filestore/local.go    — os.Remove for each returned slug
    |       |
    |       v
    |     storage/file_repo.go  — DELETE FROM files WHERE slug = ?
    |
    +--- storage/bucket_repo.go — similar: delete bucket_files rows, remove from disk, delete bucket row
```

### Key Data Flow Principles

1. **Bytes and metadata flow separately:** File bytes go through `filestore`, metadata goes through `storage`. The service layer coordinates both — it never conflates them.
2. **One-use atomicity:** One-use paste deletion must happen in the same database transaction as the read, or with a "viewed" flag set atomically, to prevent double delivery under concurrent requests.
3. **ZIP streaming, not buffering:** Bucket ZIP downloads pipe file bytes directly from disk into the response writer through `zip.NewWriter`. Do not buffer entire ZIP in memory.
4. **Handlers own no business logic:** If a handler does more than parse input / call service / write response, that logic belongs in the service.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 0-1k users | Single binary, SQLite WAL mode, default config. No changes needed. |
| 1k-10k users | Enable SQLite WAL+PRAGMA busy_timeout. Tune cleanup interval. Consider offloading static files to CDN. |
| 10k+ users | SQLite becomes a write bottleneck for metadata. Migration path: swap SQLite repos for Postgres repos (interfaces already defined). File storage: swap LocalFS for S3 (StorageBackend interface). No handler/domain changes needed. |

### Scaling Priorities

1. **First bottleneck:** SQLite write lock under concurrent uploads. Mitigation: WAL mode (`PRAGMA journal_mode=WAL`) and connection pool with max 1 writer.
2. **Second bottleneck:** Disk I/O for large file streaming. Mitigation: `io.Copy` with sensible buffer, `http.ServeContent` for range request support.

## Anti-Patterns

### Anti-Pattern 1: Business Logic in Handlers

**What people do:** Put expiry validation, slug generation, or password hashing directly in the HTTP handler function.
**Why it's wrong:** Handlers become untestable without spinning up an HTTP server. Logic is not reusable by a future CLI or background worker.
**Do this instead:** Handlers call service methods with parsed primitive inputs. All domain decisions live in the service/entity layer.

### Anti-Pattern 2: Shared Global DB Variable

**What people do:** `var db *sql.DB` at package level, used directly in handlers or service functions.
**Why it's wrong:** Impossible to test in isolation, creates implicit coupling, makes dependency graph invisible.
**Do this instead:** Inject `*sql.DB` into repository structs at startup. Pass repositories into services via constructor. Services are injected into handlers.

### Anti-Pattern 3: Buffering Large Files in Memory

**What people do:** `io.ReadAll(r.Body)` or `ioutil.ReadAll(file)` before writing to disk or sending ZIP.
**Why it's wrong:** A single large upload or ZIP download can exhaust server memory. Especially dangerous for bucket ZIP of many files.
**Do this instead:** `io.Copy` from source to destination. Use `http.MaxBytesReader` to enforce upload limits before reading begins.

### Anti-Pattern 4: Storing File Bytes in SQLite

**What people do:** Use a BLOB column in SQLite for file content alongside metadata.
**Why it's wrong:** SQLite performance degrades for large BLOBs. Backup/restore becomes unwieldy. File streaming is awkward.
**Do this instead:** Store metadata (name, size, slug, MIME, expiry) in SQLite. Store bytes on disk keyed by slug. `filestore` handles the separation.

### Anti-Pattern 5: One-Use Logic at the Route Level

**What people do:** Check `OneUse` flag in the HTTP handler after rendering, then call delete.
**Why it's wrong:** Under concurrent requests, two requests can both pass the "not yet deleted" check before either deletes.
**Do this instead:** Atomically read-and-delete in the repository using a transaction with `SELECT ... FOR UPDATE` semantics, or use a single `DELETE ... RETURNING *` (SQLite supports this since 3.35).

## Integration Points

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| handler → domain/service | Direct Go function call | Handler imports service type, not interface — services are concrete in this project |
| domain/service → repository | Interface call (domain-defined interface) | Enables swapping SQLite for Postgres without touching service |
| domain/service → filestore | Interface call (StorageBackend) | Enables swapping local FS for S3 later |
| cleanup/worker → repositories | Interface call | Worker holds repository interfaces for all three domains |
| main.go → all | Constructor injection | All dependencies wired at startup in main.go |
| web/embed.go → handler | `embed.FS` passed to router | Template rendering and static file serving |

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| SQLite | `database/sql` + `modernc.org/sqlite` driver (CGO-free) | Open once in main, pass pool to repo constructors |
| Filesystem | `os` package via `filestore.StorageBackend` interface | Configurable root dir from config |
| Templates | `html/template` parsed from `embed.FS` at startup | Pre-parse at init, not per-request |

## Build Order Implications

Dependencies flow bottom-up. Build and test in this order:

1. **`internal/config`** — no dependencies, needed by everything
2. **`internal/slug`** — no dependencies, needed by all three domain services
3. **`internal/domain/paste`** — entity + interface + service (no infrastructure needed to test)
4. **`internal/domain/file`** — entity + interface + service
5. **`internal/domain/bucket`** — entity + interface + service (depends on file entity for BucketFile)
6. **`internal/filestore`** — StorageBackend interface + local implementation
7. **`internal/storage`** — SQLite repository implementations (depends on domain packages for types)
8. **`internal/cleanup`** — depends on all three domain repository interfaces
9. **`internal/handler`** — depends on all three domain services, config, filestore
10. **`web/`** — embed.FS setup, templates; no Go logic dependencies
11. **`cmd/pbin/main.go`** — wire everything together

Each layer can be built and unit-tested before the next layer exists. Domain services can be tested with in-memory repository mocks. Storage repos can be tested against a real SQLite `:memory:` database.

## Sources

- Three Dots Labs: DDD Lite in Go — https://threedots.tech/post/ddd-lite-in-go-introduction/
- Calhoun.io: Moving Towards DDD in Go — https://www.calhoun.io/moving-towards-domain-driven-design-in-go/
- Programming Percy: How to Structure DDD in Go — https://programmingpercy.tech/blog/how-to-structure-ddd-in-go/
- Go embed package official docs — https://pkg.go.dev/embed
- Lenpaste internal package structure — https://pkg.go.dev/git.lcomrade.su/root/lenpaste/internal
- Linx-server source layout — https://pkg.go.dev/github.com/andreimarcu/linx-server
- Go Project Structure Practices & Patterns (2025) — https://www.glukhov.org/post/2025/12/go-project-structure/
- Clean Architecture in Go (2025) — https://dasroot.net/posts/2026/01/go-project-structure-clean-architecture/

---
*Architecture research for: pbin — self-hosted file sharing + pastebin, Go stdlib, SQLite, single binary*
*Researched: 2026-03-19*
