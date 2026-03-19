# Pitfalls Research

**Domain:** Self-hosted file sharing + multi-file transfer buckets + pastebin, Go + SQLite + single binary
**Researched:** 2026-03-19
**Confidence:** HIGH (core Go/SQLite mechanics) / MEDIUM (domain-specific patterns from community evidence)

---

## Critical Pitfalls

### Pitfall 1: SQLite "Database Is Locked" Under Concurrent Uploads

**What goes wrong:**
Multiple simultaneous file upload requests each open a write transaction. With the default journal mode (DELETE/ROLLBACK), only one writer is allowed at a time; all others fail immediately with `SQLITE_BUSY`. Even with WAL mode enabled, Go's `database/sql` pool spawns multiple connections, each capable of issuing a write, producing the same error under burst traffic.

**Why it happens:**
Developers enable WAL and assume the problem is solved. The real fix requires a two-pool architecture: a single-connection write pool (`db.SetMaxOpenConns(1)`) and a separate multi-connection read pool. Additionally, `database/sql` opens transactions as `BEGIN DEFERRED` by default — a transaction that starts with `SELECT` and later does `INSERT/UPDATE` tries to upgrade the lock and fails instantly even with a busy timeout set.

With `ncruces/go-sqlite3` (WASM-based, CGO-free), `EXCLUSIVE` locking mode is required to use WAL databases, which forces `SetMaxOpenConns(1)` for all connections, so the single-writer constraint is even more explicit.

**How to avoid:**
- Enable WAL mode at startup: `PRAGMA journal_mode=WAL`
- Set a busy timeout: `PRAGMA busy_timeout=5000`
- Separate read and write `*sql.DB` instances; cap write pool to 1 connection
- Open write transactions as `BEGIN IMMEDIATE` (or `BEGIN EXCLUSIVE`) explicitly, never `BEGIN DEFERRED`
- With `modernc.org/sqlite`: set `_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)` in the DSN

**Warning signs:**
- `database is locked` or `SQLITE_BUSY` errors in logs under concurrent load
- WAL file (`.db-wal`) grows unboundedly and never checkpoints
- Upload/download errors that appear intermittently but not in sequential tests

**Phase to address:** Foundation / data layer phase (before any handler code)

---

### Pitfall 2: Multipart Upload Memory Exhaustion

**What goes wrong:**
`r.ParseMultipartForm(maxMem)` buffers files up to `maxMem` in RAM; anything larger spills to a temp file. If the handler never calls `r.MultipartForm.RemoveAll()` (or defers it), temp files accumulate on disk indefinitely. Under concurrent upload bursts the server can exhaust RAM or disk inodes before any explicit size limit is triggered.

Additionally, if `http.MaxBytesReader` is not applied before parsing, an attacker can send an arbitrarily large body — the stdlib will happily read it all, exhausting memory long before ParseMultipartForm sees the content.

**Why it happens:**
The stdlib's multipart API looks simple but has a two-step trap: size enforcement (`MaxBytesReader`) must happen *before* `ParseMultipartForm`, and cleanup (`RemoveAll`) must be deferred *after*. Omitting either step is easy and produces no compile-time warning.

**How to avoid:**
```go
// Always: wrap body BEFORE parsing
r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

if err := r.ParseMultipartForm(32 << 20); err != nil { /* 413 */ }
defer r.MultipartForm.RemoveAll()
```
For very large files, bypass `ParseMultipartForm` entirely and use `r.Body` with a streaming `multipart.Reader` so no temp file is created at all — write directly to the destination file.

**Warning signs:**
- `/tmp` fills up during upload-heavy periods
- RSS memory grows monotonically and never drops between requests
- Go profile shows `multipart.(*Reader).readForm` holding large allocations

**Phase to address:** File upload handler phase (core upload feature implementation)

---

### Pitfall 3: One-Time Download / One-Use Paste Race Condition (TOCTOU)

**What goes wrong:**
Two concurrent requests hit the same one-time-download link simultaneously. Both pass the `"has this been consumed?"` check before either marks it consumed. Both receive the file/paste. The one-time semantic is violated.

**Why it happens:**
The naive implementation is: `SELECT ... WHERE id = ? AND downloaded_at IS NULL`, return file, `UPDATE ... SET downloaded_at = NOW()`. Under concurrency these two statements are not atomic — any gap between them is a TOCTOU window. Because SQLite allows concurrent reads in WAL mode, both requests can complete the `SELECT` before either `UPDATE` is committed.

**How to avoid:**
Use a single atomic `UPDATE ... WHERE downloaded_at IS NULL` and check rows-affected:
```go
res, err := db.ExecContext(ctx,
    `UPDATE files SET downloaded_at = ? WHERE id = ? AND downloaded_at IS NULL`,
    time.Now(), fileID)
n, _ := res.RowsAffected()
if n == 0 { /* already consumed — 404/410 */ }
```
The first request wins the UPDATE; the second sees `RowsAffected == 0` and returns 410 Gone. No SELECT needed. This works because SQLite serialises writes.

**Warning signs:**
- Integration tests pass but load tests show duplicate deliveries of one-time pastes
- Manual concurrent `curl` to the same link returns two 200s instead of one 200 and one 404

**Phase to address:** File sharing / paste core logic phase

---

### Pitfall 4: Content-Type Sniffing and Stored XSS via Uploaded Files

**What goes wrong:**
A user uploads a `.png` file whose body is actually `<svg><script>alert(1)</script></svg>`. The server stores it and later serves it with the user-supplied or auto-detected `Content-Type`. Browsers sniff the content and execute the JavaScript, resulting in a stored XSS attack against any visitor who views the "image" via the service's domain.

SVG is the most dangerous case — it is a valid XML image format that can contain `<script>` tags and executes in the browser's HTML context. HTML files uploaded to the same origin are equivalent.

**Why it happens:**
Developers trust `http.DetectContentType` or the client's `Content-Type` header. Neither is safe: `DetectContentType` identifies SVG as `text/xml`, which browsers execute, and the client header is entirely attacker-controlled.

**How to avoid:**
- Never serve user-uploaded files from the same origin as the UI (`https://pbin.example.com/u/file.png` on the same host as the app is dangerous)
- If serving from the same origin, force `Content-Disposition: attachment` for all uploads so they download rather than render
- Override `Content-Type` for known dangerous types: SVG → `application/octet-stream`, HTML → `application/octet-stream`
- Set `X-Content-Type-Options: nosniff` on all file-serving responses
- Set a strict `Content-Security-Policy` on all pages

**Warning signs:**
- Files served inline from the application origin without `Content-Disposition: attachment`
- SVG or HTML uploads viewable in-browser without explicit download prompt
- Missing `X-Content-Type-Options: nosniff` header on served files

**Phase to address:** File storage and serving phase; security hardening phase

---

### Pitfall 5: Disk Exhaustion via Upload Flooding (No Total Storage Quota)

**What goes wrong:**
`maxUploadSize` per file is enforced, but there is no cap on total storage. An attacker uploads thousands of max-size files with `never` expiry. The disk fills up, the SQLite database file itself cannot grow (WAL writes fail), and the service crashes for all users.

**Why it happens:**
Developers focus on per-request limits and forget instance-level limits. SQLite metadata stays small even as the files directory grows enormous.

**How to avoid:**
- Enforce a configurable total storage quota: check `du` or maintain a running `bytes_used` counter in SQLite
- Require a configurable max number of active uploads per IP or per API key
- Disallow `never` expiry unless explicitly configured by the operator
- Implement a startup check that warns if less than `X` MB of disk is free

**Warning signs:**
- No `storage_quota_bytes` config option exists
- Files directory size not monitored or logged
- Operator relies only on `maxUploadSize` and expiry to bound storage

**Phase to address:** Configuration / operator hardening phase

---

### Pitfall 6: CGO-Free SQLite Driver Behaving Differently from `mattn/go-sqlite3`

**What goes wrong:**
The project uses `modernc.org/sqlite` or `ncruces/go-sqlite3` for CGO-free cross-compilation. Both drivers have subtle differences from `mattn/go-sqlite3` that cause silent misbehaviour: pragma strings parsed differently, missing features, different WAL locking semantics, or different `database/sql` hook behaviour.

Specific known issues:
- `ncruces/go-sqlite3` requires `EXCLUSIVE` locking to use WAL, effectively forcing single-connection mode
- `modernc.org/sqlite` is slower than `mattn` at high write throughput (known benchmark gap)
- Neither driver supports all SQLite extensions that docs for `mattn` reference

**Why it happens:**
Most Go SQLite tutorials and StackOverflow answers target `mattn/go-sqlite3`. Developers copy patterns from those sources without checking CGO-free driver docs.

**How to avoid:**
- Decide on the driver early (`modernc.org/sqlite` is the most widely used CGO-free option; `ncruces/go-sqlite3` offers WASM isolation)
- Read the chosen driver's README completely before writing any database code
- Add an integration test that opens the DB, writes, checkpoints, reads back, and verifies WAL file cleanup
- Benchmark write throughput with the chosen driver under expected concurrency before committing to it

**Warning signs:**
- Copy-pasted pragmas from `mattn/go-sqlite3` tutorials not verified against chosen driver
- No driver-specific integration tests

**Phase to address:** Foundation phase (driver selection and connection setup)

---

### Pitfall 7: Expiry Cleanup Goroutine Blocking the WAL or Leaking

**What goes wrong:**
A background goroutine runs `DELETE FROM files WHERE expires_at < NOW()` every N minutes. If the goroutine holds a `rows` object (e.g., fetching file IDs to delete from disk before deleting the DB row) and never closes it, the WAL checkpoint is blocked. The WAL file grows indefinitely until the process is restarted.

Additionally, if the goroutine panics and the server catches it at the top level, the cleanup loop stops silently — files are never deleted, and disk fills over time.

**Why it happens:**
Long-running goroutines are "fire and forget" in many Go codebases. Unclosed `sql.Rows` in Go + SQLite is a known footgun documented in the SQLite/Go community (turso.tech blog post, 2024).

**How to avoid:**
- Always call `rows.Close()` or use `defer rows.Close()` immediately after `db.QueryContext`
- Prefer `db.ExecContext` for bulk deletes (no rows object at all):
  `DELETE FROM files WHERE expires_at < ? AND one_time_downloaded = FALSE`
- For disk file cleanup, collect IDs first (close rows), then delete files, then delete DB rows
- Wrap the cleanup goroutine in a recover-and-restart loop
- Log on every cleanup cycle: rows deleted, bytes freed, next run time

**Warning signs:**
- `.db-wal` file growing over time without bound
- No metrics or log output from the cleanup goroutine
- Cleanup goroutine not tested in integration tests

**Phase to address:** Expiry / cleanup implementation phase

---

### Pitfall 8: Missing Schema Migration Strategy Causes Upgrade Pain

**What goes wrong:**
The SQLite schema is created with `CREATE TABLE IF NOT EXISTS` scattered through application code. When the schema changes in v2 (adding a column, renaming a table), existing deployments either fail to start or silently run with the old schema.

SQLite's `ALTER TABLE` is extremely limited — it cannot drop columns, rename columns, or change column types. Adding them later often requires recreating the entire table (create new, copy data, drop old, rename).

**Why it happens:**
For a v1 greenfield project, migrations feel like premature overhead. But without a versioned migration system from day one, the first schema change becomes a painful manual operation for every existing deployment.

**How to avoid:**
- Use `golang-migrate` or `pressly/goose` from the very first commit, even if there is only one migration
- Store migrations as embedded SQL files (`embed.FS`)
- Run migrations automatically at startup before the HTTP server binds
- Never modify an already-applied migration file — always add a new one
- Write each migration as idempotent or test it against a populated database, not just a fresh one

**Warning signs:**
- `CREATE TABLE IF NOT EXISTS` in application Go code rather than migration files
- No `schema_migrations` or `goose_db_version` table in the database
- Schema documented only in comments or README

**Phase to address:** Foundation phase (database setup)

---

### Pitfall 9: Embedded Frontend Makes Development Iteration Slow

**What goes wrong:**
`//go:embed` bakes the frontend assets into the binary at compile time. Every CSS or HTML change requires a full `go build` + restart. Frontend development iteration, which should be fast, becomes painful. Developers either skip the embed pattern during development (risking divergence between dev and prod) or tolerate slow iteration.

**Why it happens:**
The embed package is designed for production distribution. The development ergonomics of `go:embed` are not its concern, and the Go toolchain offers no live-reload for embedded assets.

**How to avoid:**
Use a build tag to switch between embedded and disk-served assets:
```go
//go:build !dev
// +build !dev

var staticFS = fs.FS(embeddedFS) // uses go:embed
```
```go
//go:build dev
// +build dev

var staticFS = os.DirFS("./web/static") // serves from disk live
```
Run with `go run -tags dev .` during development. Production builds use the default (no tag). Tools like `air` or `watchexec` handle Go restart on backend changes.

**Warning signs:**
- No `dev` build tag or equivalent mechanism in place
- Developers complain about slow frontend iteration
- Frontend assets committed without a way to serve them without recompiling

**Phase to address:** Frontend embedding phase

---

### Pitfall 10: ZIP Bundle Download Reads All Files Into Memory

**What goes wrong:**
Transfer buckets support ZIP download of all files. The naive implementation uses `archive/zip` to write into a `bytes.Buffer`, then sends the buffer as the response. For a bucket containing several gigabytes of files, this exhausts RAM and causes OOM kills.

**Why it happens:**
The `archive/zip` example in Go docs writes to an `io.Writer`. Developers reach for `bytes.Buffer` as the `io.Writer` because it is simple. They miss that `http.ResponseWriter` itself is an `io.Writer` and supports streaming.

**How to avoid:**
Stream the ZIP directly to the response writer:
```go
w.Header().Set("Content-Type", "application/zip")
w.Header().Set("Content-Disposition", `attachment; filename="bundle.zip"`)
zw := zip.NewWriter(w)
defer zw.Close()
for _, f := range bucketFiles {
    fw, _ := zw.Create(f.Name)
    // stream from disk to fw
    src, _ := os.Open(f.StoragePath)
    io.Copy(fw, src)
    src.Close()
}
```
Do not set `Content-Length` when streaming (length is unknown at start). Accept the chunked transfer encoding.

**Warning signs:**
- ZIP creation uses `bytes.Buffer` or `io.Pipe` where `w http.ResponseWriter` would work directly
- Large bucket downloads cause memory spikes visible in server metrics

**Phase to address:** Transfer bucket / ZIP download phase

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| `CREATE TABLE IF NOT EXISTS` in Go code instead of migration files | No migration tooling setup | First schema change breaks all existing deployments; ALTER TABLE limitations in SQLite | Never — use migration files from day 1 |
| Single `*sql.DB` for reads and writes | Simpler setup | `SQLITE_BUSY` errors under any concurrent load | Never for a service with concurrent requests |
| Serve uploads from same origin without `Content-Disposition: attachment` | Simpler URL structure | Stored XSS via uploaded SVG/HTML | Never |
| Skip `r.MultipartForm.RemoveAll()` | Less boilerplate | Temp files accumulate, disk exhaustion | Never |
| Hard-code storage path as `./uploads` | Trivial to start | Breaks single-binary deployment to arbitrary directories | Only in the first local prototype; remove before first beta |
| No background cleanup goroutine (rely on reads to check expiry) | No goroutine management | Expired files never deleted, disk fills over time | Only in early alpha with manual cleanup accepted |
| `http.DetectContentType` result served as-is | Easy content type detection | SVG/HTML served as renderable types, XSS risk | Never for publicly accessible files |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| `modernc.org/sqlite` DSN pragmas | Copying DSN pragma syntax from `mattn` docs | Read `modernc.org/sqlite` README; use `_pragma=` prefix format; test each pragma actually takes effect with a `PRAGMA journal_mode` query after open |
| `ncruces/go-sqlite3` WAL mode | Assuming WAL works like `mattn` | Set `EXCLUSIVE` locking mode; use single connection (`SetMaxOpenConns(1)`); see driver wiki |
| `archive/zip` + `http.ResponseWriter` | Using intermediate buffer | Write ZIP directly to `http.ResponseWriter`; do not buffer in memory |
| `golang-migrate` + SQLite | Including `BEGIN`/`COMMIT` in migration SQL | The driver wraps each migration in an implicit transaction; explicit transaction statements cause errors |
| `//go:embed` glob patterns | Embedding only top-level files | Use `all:` prefix to include dotfiles; use recursive glob `**` for subdirectories; test embed in CI against a clean checkout |
| Basic auth middleware | Only checking `Authorization` header without timing-safe comparison | Use `subtle.ConstantTimeCompare` for credential comparison to prevent timing attacks |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| WAL file grows without checkpointing | Disk usage climbs; queries slow down as WAL lengthens | Close all `sql.Rows` promptly; use `PRAGMA wal_autocheckpoint=1000`; run periodic `PRAGMA wal_checkpoint(TRUNCATE)` during low-traffic windows | WAL exceeds ~100 MB |
| No index on `expires_at` column | Cleanup `DELETE WHERE expires_at < ?` scans full table | Add `CREATE INDEX idx_expires ON files(expires_at)` in the initial migration | ~50k+ rows in table |
| Synchronous ZIP assembly for large buckets | Request timeout or OOM during download | Stream ZIP directly to `ResponseWriter` as described above | Bucket > ~200 MB |
| Per-request disk stat calls to enforce quota | Latency spike on every upload | Maintain a `bytes_used` counter in SQLite, updated atomically with file metadata insertion | High upload concurrency |
| `http.FileServer` for serving uploads | No control over `Content-Type` or `Content-Disposition`; expiry not enforced | Write a dedicated handler that checks expiry, sets headers explicitly, and streams from disk | Day 1 — never use `http.FileServer` for uploads |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Serving SVG or HTML uploads inline on the application origin | Stored XSS against all users of the service | Force `Content-Disposition: attachment` for all uploads; set `Content-Type: application/octet-stream` for SVG and HTML; set `X-Content-Type-Options: nosniff` |
| Using `filepath.Join(storageRoot, userSuppliedFilename)` without sanitization | Path traversal: attacker writes to `../../etc/` | Never use user-supplied filenames for storage paths; generate a random UUID or hash as the on-disk filename; store the original filename in the database only |
| No rate limiting on upload or paste creation endpoints | Disk exhaustion, CPU exhaustion from hash operations | Apply per-IP rate limiting in middleware before upload logic; require configurable `UPLOAD_RATE_LIMIT` |
| Password protection using plain `==` string comparison | Timing attack reveals valid passwords | Use `subtle.ConstantTimeCompare` for all secret/password comparisons |
| Predictable sequential IDs for share links | Enumeration: attacker iterates IDs to discover private files | Use cryptographically random tokens (e.g., `crypto/rand` 128-bit) as share link IDs, never auto-increment integers |
| Storing uploaded filenames in the filesystem path | Path injection, filesystem case-sensitivity issues, Unicode normalization bugs | Store original filename in DB only; use a random UUID as the on-disk key |
| No `Content-Security-Policy` on the web UI | XSS from paste content rendered in UI, clickjacking | Set `Content-Security-Policy: default-src 'self'` and `X-Frame-Options: DENY` on all UI responses |
| Basic auth credentials compared without timing-safe function | Timing oracle reveals valid username length | `subtle.ConstantTimeCompare` for both username and password |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Showing raw token in the URL after upload without a copy button | Users manually copy long URLs, make mistakes | Provide a one-click copy button; show the full share URL prominently after creation |
| No progress indicator for large uploads | Users think the page is hung; they refresh and lose the upload | Show browser-native upload progress via `XMLHttpRequest` / `fetch` with `onprogress`; at minimum show a spinner |
| Expiry countdown shown as a Unix timestamp | Users cannot interpret "1742000000" | Show human-readable relative times: "expires in 6 hours", "expires March 25" |
| Paste "raw" view returns HTML-escaped content | Developers using the API get `&lt;` instead of `<` | The `/raw/` endpoint must return `Content-Type: text/plain` with the original unescaped content |
| ZIP download with no progress feedback for large buckets | Users think nothing is happening during large ZIP assembly | Either stream with chunked encoding (browser shows download progress) or show a "preparing download" state |
| One-time paste consumed on page load — no confirmation | User accidentally opens their own paste and destroys it | Show a "You are about to view a one-time paste — it will be destroyed. Continue?" interstitial, or delay consumption until the user explicitly confirms |

---

## "Looks Done But Isn't" Checklist

- [ ] **File expiry:** Expiry is stored and displayed — verify the cleanup goroutine actually deletes expired files AND their on-disk data, not just the database row
- [ ] **One-time downloads:** Download link returns 200 once — verify a second concurrent request to the same link returns 410 (not 200) by running two simultaneous requests in a test
- [ ] **Upload size limit:** `maxUploadSize` is enforced — verify the limit applies to the raw body size with `http.MaxBytesReader`, not just the parsed form field
- [ ] **Password-protected bucket:** Bucket requires password — verify the password check uses `subtle.ConstantTimeCompare` and that the raw file contents are inaccessible without the password (not just the index page)
- [ ] **ZIP download:** ZIP button produces a zip — verify with a bucket containing >500 MB of files that memory usage does not spike (streaming, not buffering)
- [ ] **Share link IDs:** Links work — verify IDs are generated with `crypto/rand`, not `math/rand` or sequential counters
- [ ] **Syntax highlighting:** Paste renders highlighted — verify raw HTML from the highlighter library is sanitized before insertion into the template (no XSS via crafted language identifier)
- [ ] **Embed in binary:** `go build` produces working binary — verify that `go build` on a clean checkout with no `web/static` directory on disk still serves the UI correctly
- [ ] **Database migrations:** App starts — verify that running the binary against an older schema (e.g., v1 DB with v2 binary) applies migrations cleanly, not panics

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| WAL file grown to multi-GB | MEDIUM | Stop service; run `PRAGMA wal_checkpoint(TRUNCATE)` via sqlite3 CLI; identify and fix the unclosed `rows` object; restart |
| Disk full from accumulated temp files | LOW | `find /tmp -name 'multipart-*' -mtime +1 -delete`; fix `RemoveAll` defer; restart |
| Disk full from un-expired files | LOW | Manual `DELETE FROM files WHERE expires_at < unixepoch()` + delete orphaned on-disk files; implement cleanup goroutine |
| Schema without migration tooling, needs column added | HIGH | Write a one-off migration script using `CREATE TABLE new AS SELECT ...; DROP TABLE old; ALTER TABLE new RENAME TO old`; adopt golang-migrate going forward |
| Stored XSS via SVG upload | HIGH | Force-set `Content-Disposition: attachment` on all served files immediately; audit logs for exploit attempts; notify affected users |
| TOCTOU on one-time links — duplicate delivery | MEDIUM | Switch to atomic `UPDATE WHERE downloaded_at IS NULL` + `RowsAffected` check; no user-facing fix (data already delivered) |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| SQLite write concurrency / WAL setup | Phase 1 — Foundation & data layer | Integration test: 50 concurrent writes, zero SQLITE_BUSY errors |
| CGO-free driver selection and config | Phase 1 — Foundation & data layer | WAL enabled, checkpoint working, write-pool capped at 1 |
| Schema migration strategy | Phase 1 — Foundation & data layer | `schema_migrations` table exists; adding a column without re-creating DB works |
| Multipart memory exhaustion | Phase 2 — File upload handlers | Profiling test: upload 100 MB file, RSS does not spike; `/tmp` has no stale multipart files after test |
| Upload flooding / disk quota | Phase 2 — File upload handlers | Config option `max_storage_bytes` exists and is enforced |
| Content-Type XSS via uploaded files | Phase 2 — File upload handlers + Phase 5 Security | All served files have `Content-Disposition: attachment` or safe `Content-Type`; `X-Content-Type-Options: nosniff` present |
| Path traversal via filename | Phase 2 — File upload handlers | On-disk filename is always a UUID, never user input |
| One-time download race condition | Phase 3 — Core share/paste logic | Concurrent test: two simultaneous requests to same one-time link — only one 200, one 410 |
| Predictable share link IDs | Phase 3 — Core share/paste logic | IDs are 128-bit `crypto/rand` tokens (verified by inspection and test) |
| Syntax highlighting XSS | Phase 3 — Paste implementation | Fuzz test: paste with `<script>` in content and language field — no JS executes on paste view |
| Expiry cleanup goroutine correctness | Phase 4 — Expiry and cleanup | Test: insert expired file, run cleanup, verify DB row and on-disk file both deleted |
| ZIP streaming (not buffering) | Phase 4 — Transfer bucket ZIP | Load test: download a 1 GB bucket ZIP; server RSS stays stable |
| Embedded frontend dev workflow | Phase 5 — Frontend embedding | `-tags dev` serves from disk; default build serves embedded assets; both tested in CI |
| Security headers | Phase 5 — Security hardening | CSP, X-Content-Type-Options, X-Frame-Options verified with curl on all endpoints |

---

## Sources

- [Go + SQLite Best Practices — Jake Gold](https://jacob.gold/posts/go-sqlite-best-practices/)
- [Something you probably want to know about if you're using SQLite in Golang — Turso](https://turso.tech/blog/something-you-probably-want-to-know-about-if-youre-using-sqlite-in-golang-72547ad625f1)
- [Gotchas with SQLite in Production — Anže Pečar](https://blog.pecar.me/sqlite-prod/)
- [SQLite concurrent writes and "database is locked" errors — Ten Thousand Meters](https://tenthousandmeters.com/blog/sqlite-concurrent-writes-and-database-is-locked-errors/)
- [ncruces/go-sqlite3 Support Matrix (WAL / locking)](https://github.com/ncruces/go-sqlite3/wiki/Support-matrix)
- [net/http: HTTP large file upload fails with "bufio: buffer full" — golang/go#26707](https://github.com/golang/go/issues/26707)
- [proposal: net/http: Add Request.RemoveMultiPartTempFiles — golang/go#76070](https://github.com/golang/go/issues/76070)
- [Large files (more than 10 MB) uploads eat up memory in the GO service — Medium](https://medium.com/@iitk.npyadav/large-files-more-than-10-mb-uploads-eat-up-memory-in-the-go-service-running-on-an-ec2-container-bc6cf93ca7a4)
- [How to Handle File Uploads in Go at Scale — OneUptime](https://oneuptime.com/blog/post/2026-01-07-go-file-uploads-scale/view)
- [Exploiting MIME Sniffing — Beyond XSS](https://aszx87410.github.io/beyond-xss/en/ch5/mime-sniffing/)
- [Path Traversal Vulnerability in ZendTo (CVE-2025-34508) — Horizon3.ai](https://horizon3.ai/attack-research/attack-blogs/cve-2025-34508-another-file-sharing-application-another-path-traversal/)
- [PrivateBin SVG XSS / template path traversal (CVE-2025-64711, CVE-2025-64714)](https://github.com/PrivateBin/PrivateBin/releases)
- [Mastering Database Migrations in Go with golang-migrate and SQLite — DEV Community](https://dev.to/ouma_ouma/mastering-database-migrations-in-go-with-golang-sqlite-3jhb)
- [Race Condition Vulnerabilities — YesWeHack](https://www.yeswehack.com/learn-bug-bounty/ultimate-guide-race-condition-vulnerabilities)
- [go:embed in prod, serve-from-disk in development — brandur.org](https://brandur.org/fragments/go-embed)
- [Common Anti-Patterns in Go Web Applications — Three Dots Labs](https://threedots.tech/post/common-anti-patterns-in-go-web-applications/)
- [Zip Bomb Attack prevention — Mastering File Upload Security (Theodo)](https://blog.theodo.com/2024/03/mastering-file-upload-security-dos-attacks-and-antivirus/)

---
*Pitfalls research for: self-hosted file sharing + pastebin, Go + SQLite + single binary*
*Researched: 2026-03-19*
