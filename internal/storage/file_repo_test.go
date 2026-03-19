package storage_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/domain/file"
	"github.com/ahmethakanbesel/pbin/internal/storage"
)

// openTestDB opens a temporary SQLite database for testing.
func openTestDB(t *testing.T) *storage.DBPair {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	pair, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	t.Cleanup(func() { pair.Close() })
	return pair
}

// newTestFile builds a valid File entity for use in tests.
func newTestFile(t *testing.T, slug string) file.File {
	t.Helper()
	f, err := file.New(slug, "test.txt", "text/plain", "1d", "deletesecret", 123, "", false)
	if err != nil {
		t.Fatalf("newTestFile: %v", err)
	}
	return f
}

func TestFileRepo_NewFileRepo_ImplementsRepository(t *testing.T) {
	// Compile-time check is in file_repo.go; this just confirms NewFileRepo returns non-nil.
	pair := openTestDB(t)
	repo := storage.NewFileRepo(pair)
	if repo == nil {
		t.Fatal("NewFileRepo returned nil")
	}
}

func TestFileRepo_Create_GetBySlug_RoundTrip(t *testing.T) {
	ctx := context.Background()
	pair := openTestDB(t)
	repo := storage.NewFileRepo(pair)

	f := newTestFile(t, "slug01")
	expiresAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

	if err := repo.Create(ctx, f, &expiresAt); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetBySlug(ctx, "slug01")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}

	if got.Slug != f.Slug {
		t.Errorf("Slug: got %q, want %q", got.Slug, f.Slug)
	}
	if got.Filename != f.Filename {
		t.Errorf("Filename: got %q, want %q", got.Filename, f.Filename)
	}
	if got.Size != f.Size {
		t.Errorf("Size: got %d, want %d", got.Size, f.Size)
	}
	if got.MimeType != f.MimeType {
		t.Errorf("MimeType: got %q, want %q", got.MimeType, f.MimeType)
	}
	if got.DeleteSecret != f.DeleteSecret {
		t.Errorf("DeleteSecret: got %q, want %q", got.DeleteSecret, f.DeleteSecret)
	}
	if got.OneUse != f.OneUse {
		t.Errorf("OneUse: got %v, want %v", got.OneUse, f.OneUse)
	}
	if got.ExpiresAt == nil {
		t.Fatal("ExpiresAt: got nil, want non-nil")
	}
	if !got.ExpiresAt.Equal(expiresAt) {
		t.Errorf("ExpiresAt: got %v, want %v", got.ExpiresAt, expiresAt)
	}
}

func TestFileRepo_Create_NilExpiry(t *testing.T) {
	ctx := context.Background()
	pair := openTestDB(t)
	repo := storage.NewFileRepo(pair)

	f := newTestFile(t, "slug02")
	if err := repo.Create(ctx, f, nil); err != nil {
		t.Fatalf("Create with nil expiresAt: %v", err)
	}

	got, err := repo.GetBySlug(ctx, "slug02")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.ExpiresAt != nil {
		t.Errorf("ExpiresAt: got %v, want nil", got.ExpiresAt)
	}
}

func TestFileRepo_GetBySlug_NotFound(t *testing.T) {
	ctx := context.Background()
	pair := openTestDB(t)
	repo := storage.NewFileRepo(pair)

	_, err := repo.GetBySlug(ctx, "doesnotexist")
	if !errors.Is(err, file.ErrNotFound) {
		t.Errorf("expected file.ErrNotFound, got %v", err)
	}
}

func TestFileRepo_MarkDownloaded_FirstCall_ReturnsTrue(t *testing.T) {
	ctx := context.Background()
	pair := openTestDB(t)
	repo := storage.NewFileRepo(pair)

	f := newTestFile(t, "slug03")
	if err := repo.Create(ctx, f, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	consumed, err := repo.MarkDownloaded(ctx, "slug03")
	if err != nil {
		t.Fatalf("MarkDownloaded: %v", err)
	}
	if !consumed {
		t.Error("MarkDownloaded: expected true on first call, got false")
	}
}

func TestFileRepo_MarkDownloaded_SecondCall_ReturnsFalse(t *testing.T) {
	ctx := context.Background()
	pair := openTestDB(t)
	repo := storage.NewFileRepo(pair)

	f := newTestFile(t, "slug04")
	if err := repo.Create(ctx, f, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// First call should succeed.
	if _, err := repo.MarkDownloaded(ctx, "slug04"); err != nil {
		t.Fatalf("MarkDownloaded first: %v", err)
	}

	// Second call should return false (already consumed).
	consumed, err := repo.MarkDownloaded(ctx, "slug04")
	if err != nil {
		t.Fatalf("MarkDownloaded second: %v", err)
	}
	if consumed {
		t.Error("MarkDownloaded: expected false on second call, got true")
	}
}

func TestFileRepo_Delete_RemovesRow(t *testing.T) {
	ctx := context.Background()
	pair := openTestDB(t)
	repo := storage.NewFileRepo(pair)

	f := newTestFile(t, "slug05")
	if err := repo.Create(ctx, f, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, "slug05"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.GetBySlug(ctx, "slug05")
	if !errors.Is(err, file.ErrNotFound) {
		t.Errorf("after Delete: expected file.ErrNotFound, got %v", err)
	}
}

func TestFileRepo_ListExpired_ReturnsOnlyExpiredFiles(t *testing.T) {
	ctx := context.Background()
	pair := openTestDB(t)
	repo := storage.NewFileRepo(pair)

	// File that expired in the past.
	past := time.Now().Add(-time.Hour)
	fExpired := newTestFile(t, "expired01")
	if err := repo.Create(ctx, fExpired, &past); err != nil {
		t.Fatalf("Create expired file: %v", err)
	}

	// File that expires in the future.
	future := time.Now().Add(time.Hour)
	fFuture := newTestFile(t, "future01")
	if err := repo.Create(ctx, fFuture, &future); err != nil {
		t.Fatalf("Create future file: %v", err)
	}

	// File with no expiry.
	fNever := newTestFile(t, "never01")
	if err := repo.Create(ctx, fNever, nil); err != nil {
		t.Fatalf("Create never-expiring file: %v", err)
	}

	expired, err := repo.ListExpired(ctx)
	if err != nil {
		t.Fatalf("ListExpired: %v", err)
	}

	if len(expired) != 1 {
		t.Fatalf("ListExpired: expected 1 expired file, got %d", len(expired))
	}
	if expired[0].Slug != "expired01" {
		t.Errorf("ListExpired: expected slug %q, got %q", "expired01", expired[0].Slug)
	}
}

func TestFileRepo_Create_WithPassword(t *testing.T) {
	ctx := context.Background()
	pair := openTestDB(t)
	repo := storage.NewFileRepo(pair)

	f, err := file.New("slug06", "secret.txt", "text/plain", "never", "delsecret", 50, "$2a$12$hashvalue", false)
	if err != nil {
		t.Fatalf("file.New: %v", err)
	}

	if err := repo.Create(ctx, f, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetBySlug(ctx, "slug06")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.PasswordHash != f.PasswordHash {
		t.Errorf("PasswordHash: got %q, want %q", got.PasswordHash, f.PasswordHash)
	}
}

func TestFileRepo_Create_OneUseFile(t *testing.T) {
	ctx := context.Background()
	pair := openTestDB(t)
	repo := storage.NewFileRepo(pair)

	f, err := file.New("slug07", "once.txt", "text/plain", "1h", "delsecret", 10, "", true)
	if err != nil {
		t.Fatalf("file.New: %v", err)
	}

	if err := repo.Create(ctx, f, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetBySlug(ctx, "slug07")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if !got.OneUse {
		t.Error("OneUse: expected true, got false")
	}
}
