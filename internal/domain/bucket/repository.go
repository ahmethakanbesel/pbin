package bucket

import (
	"context"
	"time"
)

// Repository defines persistence operations for Bucket entities.
// Implemented in internal/storage (SQLite). Defined here so the domain
// has no knowledge of storage details.
type Repository interface {
	Create(ctx context.Context, b Bucket, expiresAt *time.Time) error
	AddFile(ctx context.Context, bf BucketFile) error
	GetBySlug(ctx context.Context, slug string) (Bucket, error)
	// MarkDownloaded atomically sets downloaded_at for one-use buckets.
	// Returns (false, nil) if already downloaded.
	MarkDownloaded(ctx context.Context, slug string) (bool, error)
	Delete(ctx context.Context, slug string) error
	ListExpired(ctx context.Context) ([]Bucket, error)
}
