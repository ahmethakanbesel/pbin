// Package file provides the File domain service and business rules.
package file

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/filestore"
	"github.com/ahmethakanbesel/pbin/internal/slug"
	"golang.org/x/crypto/bcrypt"
)

// Sentinel errors returned by Service methods. Handlers map these to HTTP status codes.
var (
	ErrNotFound        = errors.New("file: not found")
	ErrExpired         = errors.New("file: expired")
	ErrAlreadyConsumed = errors.New("file: one-use file already consumed")
	ErrWrongPassword   = errors.New("file: wrong password")
	ErrBadDeleteSecret = errors.New("file: invalid delete secret")
)

const (
	slugLength         = 12
	deleteSecretLength = 24
	bcryptCost         = 12
)

// Service implements file upload, download, and deletion business rules.
type Service struct {
	repo    Repository
	store   filestore.Backend
	baseURL string
}

// NewService constructs a Service with the given repository, storage backend, and base URL.
// baseURL must not have a trailing slash (e.g., "https://pbin.example.com").
func NewService(repo Repository, store filestore.Backend, baseURL string) *Service {
	return &Service{repo: repo, store: store, baseURL: baseURL}
}

// UploadRequest carries all inputs for a file upload.
type UploadRequest struct {
	Filename string
	MimeType string
	Size     int64
	Expiry   string    // preset: "10m", "1h", ... "never"
	Password string    // plain text; empty = no protection
	OneUse   bool
	Content  io.Reader
}

// UploadResult is returned on successful upload.
type UploadResult struct {
	Slug      string
	URL       string
	DeleteURL string
	ExpiresAt *time.Time
	IsImage   bool
}

// GetResult is returned on successful file retrieval.
type GetResult struct {
	F         File
	Content   io.ReadCloser
	ExpiresAt *time.Time
	IsImage   bool
}

// Upload stores a new file and returns shareable URLs.
func (s *Service) Upload(ctx context.Context, req UploadRequest) (UploadResult, error) {
	shareSlug, err := slug.New(slugLength)
	if err != nil {
		return UploadResult{}, fmt.Errorf("file service upload: generate slug: %w", err)
	}

	deleteSecret, err := slug.New(deleteSecretLength)
	if err != nil {
		return UploadResult{}, fmt.Errorf("file service upload: generate delete secret: %w", err)
	}

	var passwordHash string
	if req.Password != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
		if err != nil {
			return UploadResult{}, fmt.Errorf("file service upload: hash password: %w", err)
		}
		passwordHash = string(h)
	}

	f, err := New(shareSlug, req.Filename, req.MimeType, req.Expiry, deleteSecret, req.Size, passwordHash, req.OneUse)
	if err != nil {
		return UploadResult{}, fmt.Errorf("file service upload: construct entity: %w", err)
	}

	// Compute absolute expiry timestamp (nil = never).
	var expiresAt *time.Time
	if d := ExpiryDuration(req.Expiry); d > 0 {
		t := time.Now().Add(d)
		expiresAt = &t
	}

	// Persist bytes first — if DB insert fails we can clean up the file.
	if err := s.store.Write(ctx, shareSlug, req.Content); err != nil {
		return UploadResult{}, fmt.Errorf("file service upload: write bytes: %w", err)
	}

	if err := s.repo.Create(ctx, f, expiresAt); err != nil {
		// Best-effort cleanup — ignore error from Delete.
		_ = s.store.Delete(ctx, shareSlug)
		return UploadResult{}, fmt.Errorf("file service upload: persist metadata: %w", err)
	}

	return UploadResult{
		Slug:      shareSlug,
		URL:       s.baseURL + "/" + shareSlug + "/info",
		DeleteURL: s.baseURL + "/delete/" + shareSlug + "/" + deleteSecret,
		ExpiresAt: expiresAt,
		IsImage:   IsImage(req.MimeType),
	}, nil
}

// Get retrieves a file by slug, enforcing expiry, one-use, and password protection.
// The caller MUST close GetResult.Content when done.
// passwordAttempt is the plain-text password; pass "" if the file is not password-protected.
func (s *Service) Get(ctx context.Context, shareSlug, passwordAttempt string) (GetResult, error) {
	f, err := s.repo.GetBySlug(ctx, shareSlug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return GetResult{}, ErrNotFound
		}
		return GetResult{}, fmt.Errorf("file service get: lookup: %w", err)
	}

	// Enforce expiry at read time (requires repository to populate ExpiresAt).
	if f.ExpiresAt != nil && time.Now().After(*f.ExpiresAt) {
		return GetResult{}, ErrExpired
	}

	// Enforce password before returning content.
	if f.PasswordHash != "" {
		if passwordAttempt == "" {
			return GetResult{}, ErrWrongPassword
		}
		if err := bcrypt.CompareHashAndPassword([]byte(f.PasswordHash), []byte(passwordAttempt)); err != nil {
			return GetResult{}, ErrWrongPassword
		}
	}

	// Atomically mark one-use file as consumed.
	if f.OneUse {
		consumed, err := s.repo.MarkDownloaded(ctx, shareSlug)
		if err != nil {
			return GetResult{}, fmt.Errorf("file service get: mark downloaded: %w", err)
		}
		if !consumed {
			return GetResult{}, ErrAlreadyConsumed
		}
	}

	rc, err := s.store.Read(ctx, shareSlug)
	if err != nil {
		return GetResult{}, fmt.Errorf("file service get: read bytes: %w", err)
	}

	return GetResult{
		F:         f,
		Content:   rc,
		ExpiresAt: f.ExpiresAt,
		IsImage:   IsImage(f.MimeType),
	}, nil
}

// GetMeta retrieves file metadata by slug without consuming one-use files.
// Used by the Info/embed page to show metadata without triggering the one-use download.
func (s *Service) GetMeta(ctx context.Context, shareSlug string) (File, error) {
	f, err := s.repo.GetBySlug(ctx, shareSlug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return File{}, ErrNotFound
		}
		return File{}, fmt.Errorf("file service get meta: lookup: %w", err)
	}

	if f.ExpiresAt != nil && time.Now().After(*f.ExpiresAt) {
		return File{}, ErrExpired
	}

	return f, nil
}

// Delete removes a file if the deleteSecret matches.
func (s *Service) Delete(ctx context.Context, shareSlug, deleteSecret string) error {
	f, err := s.repo.GetBySlug(ctx, shareSlug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("file service delete: lookup: %w", err)
	}

	// Timing-safe comparison to prevent secret enumeration.
	if subtle.ConstantTimeCompare([]byte(f.DeleteSecret), []byte(deleteSecret)) != 1 {
		return ErrBadDeleteSecret
	}

	// Delete disk bytes first; if this fails the DB row stays (operator can clean up).
	if err := s.store.Delete(ctx, shareSlug); err != nil {
		return fmt.Errorf("file service delete: remove bytes: %w", err)
	}

	if err := s.repo.Delete(ctx, shareSlug); err != nil {
		return fmt.Errorf("file service delete: remove metadata: %w", err)
	}

	return nil
}
