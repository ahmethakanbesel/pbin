package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ahmethakanbesel/pbin/internal/filestore"
)

func TestNewLocal_CreatesRootDirectory(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "storage", "nested")

	fs, err := filestore.NewLocal(root)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}
	_ = fs

	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Errorf("expected root directory %q to be created", root)
	}
}

func TestLocalFS_WriteRead_RoundTrip(t *testing.T) {
	root := t.TempDir()
	fs, err := filestore.NewLocal(root)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}

	ctx := context.Background()
	key := "abc123"
	content := []byte("hello, world")

	if err := fs.Write(ctx, key, bytes.NewReader(content)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file is stored at {root}/ab/abc123
	expected := filepath.Join(root, "ab", "abc123")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("expected file at %q, not found", expected)
	}

	rc, err := fs.Read(ctx, key)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("io.ReadAll failed: %v", err)
	}

	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestLocalFS_Read_NotExist(t *testing.T) {
	root := t.TempDir()
	fs, err := filestore.NewLocal(root)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}

	ctx := context.Background()
	_, err = fs.Read(ctx, "missing99")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestLocalFS_Delete_RemovesFile(t *testing.T) {
	root := t.TempDir()
	fs, err := filestore.NewLocal(root)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}

	ctx := context.Background()
	key := "del12345"
	content := []byte("delete me")

	if err := fs.Write(ctx, key, bytes.NewReader(content)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := fs.Delete(ctx, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Subsequent Read must return os.ErrNotExist
	_, err = fs.Read(ctx, key)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("after Delete, expected os.ErrNotExist from Read, got %v", err)
	}
}

func TestLocalFS_Delete_Idempotent(t *testing.T) {
	root := t.TempDir()
	fs, err := filestore.NewLocal(root)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}

	ctx := context.Background()
	// Delete a key that was never written — must return nil (idempotent)
	if err := fs.Delete(ctx, "neverexisted1"); err != nil {
		t.Errorf("Delete on non-existent key must return nil, got %v", err)
	}
}

func TestLocalFS_Exists(t *testing.T) {
	root := t.TempDir()
	fs, err := filestore.NewLocal(root)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}

	ctx := context.Background()
	key := "exists12"
	content := []byte("i exist")

	// Not yet written — should be false
	exists, err := fs.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists (before write) failed: %v", err)
	}
	if exists {
		t.Error("Exists returned true before Write")
	}

	// Write and check again
	if err := fs.Write(ctx, key, bytes.NewReader(content)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	exists, err = fs.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists (after write) failed: %v", err)
	}
	if !exists {
		t.Error("Exists returned false after Write")
	}

	// Delete and check again
	if err := fs.Delete(ctx, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	exists, err = fs.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists (after delete) failed: %v", err)
	}
	if exists {
		t.Error("Exists returned true after Delete")
	}
}

func TestLocalFS_Write_PathTraversalKey(t *testing.T) {
	root := t.TempDir()
	fs, err := filestore.NewLocal(root)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}

	ctx := context.Background()
	// Keys with path traversal or special characters must be rejected
	badKeys := []string{"../evil", "../../etc/passwd", "foo/bar", "foo bar", "foo\x00bar"}
	for _, key := range badKeys {
		err := fs.Write(ctx, key, bytes.NewReader([]byte("evil")))
		if err == nil {
			t.Errorf("expected error for invalid key %q, got nil", key)
		}
	}
}
