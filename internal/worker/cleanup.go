// Package worker provides background workers for the pbin service.
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/domain/bucket"
	"github.com/ahmethakanbesel/pbin/internal/domain/file"
	"github.com/ahmethakanbesel/pbin/internal/domain/paste"
	"github.com/ahmethakanbesel/pbin/internal/filestore"
)

// FileRepository is the subset of file.Repository used by the cleanup worker.
type FileRepository interface {
	ListExpired(ctx context.Context) ([]file.File, error)
	Delete(ctx context.Context, slug string) error
}

// BucketRepository is the subset of bucket.Repository used by the cleanup worker.
type BucketRepository interface {
	ListExpired(ctx context.Context) ([]bucket.Bucket, error)
	Delete(ctx context.Context, slug string) error
}

// PasteRepository is the subset of paste.Repository used by the cleanup worker.
type PasteRepository interface {
	ListExpired(ctx context.Context) ([]paste.Paste, error)
	Delete(ctx context.Context, slug string) error
}

// Cleanup is a background worker that periodically deletes expired records and
// their associated on-disk storage keys.
type Cleanup struct {
	fileRepo   FileRepository
	bucketRepo BucketRepository
	pasteRepo  PasteRepository
	store      filestore.Backend
	interval   time.Duration
	stop       chan struct{}
	done       chan struct{}
}

// NewCleanup constructs a Cleanup worker. Call Start() to begin background cleanup.
func NewCleanup(
	fileRepo FileRepository,
	bucketRepo BucketRepository,
	pasteRepo PasteRepository,
	store filestore.Backend,
	interval time.Duration,
) *Cleanup {
	return &Cleanup{
		fileRepo:   fileRepo,
		bucketRepo: bucketRepo,
		pasteRepo:  pasteRepo,
		store:      store,
		interval:   interval,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
}

// Start launches the background cleanup goroutine. Call once at startup.
// The goroutine restarts itself on panic (defer/recover).
func (c *Cleanup) Start() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("cleanup worker panicked, restarting", "panic", r)
				time.Sleep(5 * time.Second)
				c.runLoop()
			}
		}()
		c.runLoop()
	}()
}

// Stop signals the cleanup goroutine to exit and blocks until it acknowledges.
func (c *Cleanup) Stop() {
	close(c.stop)
	<-c.done
}

// runLoop runs the cleanup ticker loop. It returns when the stop channel is closed.
func (c *Cleanup) runLoop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	defer close(c.done)

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			c.sweep(ctx)
			cancel()
		case <-c.stop:
			return
		}
	}
}

// sweep deletes expired records across all three domains.
func (c *Cleanup) sweep(ctx context.Context) {
	nFiles := c.sweepFiles(ctx)
	nBuckets := c.sweepBuckets(ctx)
	nPastes := c.sweepPastes(ctx)

	slog.Info("cleanup sweep complete", "files", nFiles, "buckets", nBuckets, "pastes", nPastes)
}

// sweepFiles deletes expired file records and their on-disk storage keys.
func (c *Cleanup) sweepFiles(ctx context.Context) int {
	expired, err := c.fileRepo.ListExpired(ctx)
	if err != nil {
		slog.Error("cleanup: list expired files", "error", err)
		return 0
	}

	n := 0
	for _, f := range expired {
		// Files use their slug as the on-disk storage key (see file.Service.Delete).
		if err := c.store.Delete(ctx, f.Slug); err != nil {
			slog.Error("cleanup: delete file storage key", "slug", f.Slug, "error", err)
		}
		if err := c.fileRepo.Delete(ctx, f.Slug); err != nil {
			slog.Error("cleanup: delete file record", "slug", f.Slug, "error", err)
			continue
		}
		n++
	}
	return n
}

// sweepBuckets deletes expired bucket records and all associated on-disk storage keys.
func (c *Cleanup) sweepBuckets(ctx context.Context) int {
	expired, err := c.bucketRepo.ListExpired(ctx)
	if err != nil {
		slog.Error("cleanup: list expired buckets", "error", err)
		return 0
	}

	n := 0
	for _, b := range expired {
		for _, bf := range b.Files {
			if err := c.store.Delete(ctx, bf.StorageKey); err != nil {
				slog.Error("cleanup: delete bucket file storage key", "bucket", b.Slug, "key", bf.StorageKey, "error", err)
			}
		}
		if err := c.bucketRepo.Delete(ctx, b.Slug); err != nil {
			slog.Error("cleanup: delete bucket record", "slug", b.Slug, "error", err)
			continue
		}
		n++
	}
	return n
}

// sweepPastes deletes expired paste records. Pastes have no on-disk storage.
func (c *Cleanup) sweepPastes(ctx context.Context) int {
	expired, err := c.pasteRepo.ListExpired(ctx)
	if err != nil {
		slog.Error("cleanup: list expired pastes", "error", err)
		return 0
	}

	n := 0
	for _, p := range expired {
		if err := c.pasteRepo.Delete(ctx, p.Slug); err != nil {
			slog.Error("cleanup: delete paste record", "slug", p.Slug, "error", err)
			continue
		}
		n++
	}
	return n
}
