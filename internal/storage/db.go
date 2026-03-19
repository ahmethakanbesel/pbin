package storage

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// DBPair holds separate read and write connection pools.
// WriteDB has SetMaxOpenConns(1) to prevent SQLITE_BUSY under concurrent writes.
// ReadDB is unbounded for concurrent reads in WAL mode.
type DBPair struct {
	WriteDB *sql.DB
	ReadDB  *sql.DB
}

// Close shuts down both pools.
func (p *DBPair) Close() error {
	werr := p.WriteDB.Close()
	rerr := p.ReadDB.Close()
	if werr != nil {
		return werr
	}
	return rerr
}

// dsn builds the modernc.org/sqlite DSN with WAL mode and busy timeout.
// pragma syntax for modernc: _pragma=key(value)
func dsn(path string) string {
	return fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)",
		path,
	)
}

// Open opens (or creates) the SQLite database at path, runs all pending goose migrations,
// and returns a DBPair with a single-connection write pool and a multi-connection read pool.
// path may be ":memory:" for testing.
func Open(path string) (*DBPair, error) {
	if path == "" {
		return nil, fmt.Errorf("storage.Open: path must not be empty")
	}

	// Write pool: max 1 connection to serialise writes
	writeDB, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("storage.Open write pool: %w", err)
	}
	writeDB.SetMaxOpenConns(1)

	if err := writeDB.Ping(); err != nil {
		writeDB.Close()
		return nil, fmt.Errorf("storage.Open write pool ping: %w", err)
	}

	// Run migrations using the write connection (migrations need a write lock)
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		writeDB.Close()
		return nil, fmt.Errorf("storage.Open goose dialect: %w", err)
	}
	if err := goose.Up(writeDB, "migrations"); err != nil {
		writeDB.Close()
		return nil, fmt.Errorf("storage.Open goose migrate: %w", err)
	}

	// Read pool: separate connection pool for concurrent reads
	readDB, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		writeDB.Close()
		return nil, fmt.Errorf("storage.Open read pool: %w", err)
	}
	if err := readDB.Ping(); err != nil {
		writeDB.Close()
		readDB.Close()
		return nil, fmt.Errorf("storage.Open read pool ping: %w", err)
	}

	return &DBPair{WriteDB: writeDB, ReadDB: readDB}, nil
}
