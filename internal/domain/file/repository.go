package file

import (
	"context"
	"time"
)

// Repository defines persistence operations for File entities.
// Implemented in internal/storage (SQLite). Defined here so the domain
// has no knowledge of storage details.
type Repository interface {
	Create(ctx context.Context, f File, expiresAt *time.Time) error
	GetBySlug(ctx context.Context, slug string) (File, error)
	// MarkDownloaded atomically sets downloaded_at for one-use files.
	// Returns (false, nil) if already downloaded (one-use consumed).
	MarkDownloaded(ctx context.Context, slug string) (bool, error)
	Delete(ctx context.Context, slug string) error
	ListExpired(ctx context.Context) ([]File, error)
}
