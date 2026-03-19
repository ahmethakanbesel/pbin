package paste

import (
	"context"
	"time"
)

// Repository defines persistence operations for Paste entities.
// Implemented in internal/storage (SQLite). Defined here so the domain
// has no knowledge of storage details.
type Repository interface {
	Create(ctx context.Context, p Paste, expiresAt *time.Time) error
	GetBySlug(ctx context.Context, slug string) (Paste, error)
	// MarkViewed atomically sets viewed_at for one-use pastes.
	// Returns (false, nil) if already viewed.
	MarkViewed(ctx context.Context, slug string) (bool, error)
	Delete(ctx context.Context, slug string) error
	ListExpired(ctx context.Context) ([]Paste, error)
}
