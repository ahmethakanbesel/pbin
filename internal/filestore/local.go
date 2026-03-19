package filestore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// validKey matches slugs produced by internal/slug — alphanumeric only.
var validKey = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

// LocalFS is a filesystem-backed implementation of Backend.
// Files are stored under {root}/{prefix}/{key} where prefix is the first two
// characters of key (for inode distribution across directories).
type LocalFS struct {
	root string
}

// NewLocal creates a LocalFS rooted at root, creating the directory if needed.
func NewLocal(root string) (*LocalFS, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, fmt.Errorf("filestore: create root %s: %w", root, err)
	}
	return &LocalFS{root: root}, nil
}

func (l *LocalFS) keyPath(key string) (string, error) {
	if !validKey.MatchString(key) {
		return "", fmt.Errorf("filestore: invalid key %q (must be alphanumeric)", key)
	}
	if len(key) < 2 {
		return filepath.Join(l.root, key), nil
	}
	return filepath.Join(l.root, key[:2], key), nil
}

func (l *LocalFS) Write(ctx context.Context, key string, r io.Reader) error {
	path, err := l.keyPath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("filestore: mkdir %s: %w", filepath.Dir(path), err)
	}
	// Write to a temp file in the same directory, then rename for atomicity
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-")
	if err != nil {
		return fmt.Errorf("filestore: create temp: %w", err)
	}
	defer os.Remove(tmp.Name()) // cleanup if rename fails
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return fmt.Errorf("filestore: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("filestore: close temp: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("filestore: rename: %w", err)
	}
	return nil
}

func (l *LocalFS) Read(ctx context.Context, key string) (io.ReadCloser, error) {
	path, err := l.keyPath(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("filestore: read %s: %w", key, err)
	}
	return f, nil
}

func (l *LocalFS) Delete(ctx context.Context, key string) error {
	path, err := l.keyPath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("filestore: delete %s: %w", key, err)
	}
	return nil
}

func (l *LocalFS) Exists(ctx context.Context, key string) (bool, error) {
	path, err := l.keyPath(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("filestore: exists %s: %w", key, err)
}
