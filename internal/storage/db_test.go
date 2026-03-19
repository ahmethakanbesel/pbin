package storage_test

import (
	"path/filepath"
	"testing"

	"github.com/ahmethakanbesel/pbin/internal/storage"
)

func TestOpen_EmptyPath(t *testing.T) {
	_, err := storage.Open("")
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

func TestOpen_MemoryReturnsNonNilPair(t *testing.T) {
	// For :memory: DBs, the two pools are separate (can't share in-memory state),
	// so we use a real temp file to test the full round-trip.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	pair, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { pair.Close() })

	if pair.WriteDB == nil {
		t.Fatal("WriteDB is nil")
	}
	if pair.ReadDB == nil {
		t.Fatal("ReadDB is nil")
	}
}

func TestOpen_WALJournalMode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wal.db")

	pair, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { pair.Close() })

	var mode string
	if err := pair.WriteDB.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode query failed: %v", err)
	}
	if mode != "wal" {
		t.Errorf("expected journal_mode=wal, got %q", mode)
	}
}

func TestOpen_WritePoolMaxOneConnection(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "pool.db")

	pair, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { pair.Close() })

	stats := pair.WriteDB.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("expected WriteDB.MaxOpenConnections=1, got %d", stats.MaxOpenConnections)
	}
}

func TestOpen_GooseMigrationsRan(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "migrations.db")

	pair, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { pair.Close() })

	var count int
	err = pair.WriteDB.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='goose_db_version'",
	).Scan(&count)
	if err != nil {
		t.Fatalf("query goose_db_version existence failed: %v", err)
	}
	if count != 1 {
		t.Error("goose_db_version table does not exist — migrations did not run")
	}
}

func TestOpen_AllFourTablesExist(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "schema.db")

	pair, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { pair.Close() })

	tables := []string{"files", "buckets", "bucket_files", "pastes"}
	for _, tbl := range tables {
		var count int
		err := pair.WriteDB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tbl,
		).Scan(&count)
		if err != nil {
			t.Fatalf("query for table %q failed: %v", tbl, err)
		}
		if count != 1 {
			t.Errorf("table %q does not exist after Open", tbl)
		}
	}
}
