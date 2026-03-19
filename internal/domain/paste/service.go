package paste

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/slug"
	"golang.org/x/crypto/bcrypt"
)

const (
	slugLength         = 12
	deleteSecretLength = 24
	bcryptCost         = 12
)

// Service implements paste creation, retrieval, and deletion business rules.
type Service struct {
	repo    Repository
	baseURL string
}

// NewService constructs a Service with the given repository and base URL.
// baseURL must not have a trailing slash (e.g., "https://pbin.example.com").
func NewService(repo Repository, baseURL string) *Service {
	return &Service{repo: repo, baseURL: baseURL}
}

// CreateRequest carries all inputs for paste creation.
type CreateRequest struct {
	Title    string
	Content  string
	Lang     string // language hint; empty defaults to "text"
	Expiry   string
	Password string
	OneUse   bool
}

// CreateResult is returned on successful paste creation.
type CreateResult struct {
	Slug      string
	URL       string
	DeleteURL string
	ExpiresAt *time.Time
}

// GetResult is returned on successful paste retrieval.
type GetResult struct {
	P         Paste
	ExpiresAt *time.Time
}

// Create stores a new paste and returns shareable URLs.
func (s *Service) Create(ctx context.Context, req CreateRequest) (CreateResult, error) {
	shareSlug, err := slug.New(slugLength)
	if err != nil {
		return CreateResult{}, fmt.Errorf("paste service create: generate slug: %w", err)
	}

	deleteSecret, err := slug.New(deleteSecretLength)
	if err != nil {
		return CreateResult{}, fmt.Errorf("paste service create: generate delete secret: %w", err)
	}

	var passwordHash string
	if req.Password != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
		if err != nil {
			return CreateResult{}, fmt.Errorf("paste service create: hash password: %w", err)
		}
		passwordHash = string(h)
	}

	lang := req.Lang
	if lang == "" {
		lang = "text"
	}

	p, err := New(shareSlug, req.Title, req.Content, lang, req.Expiry, passwordHash, req.OneUse)
	if err != nil {
		return CreateResult{}, fmt.Errorf("paste service create: construct entity: %w", err)
	}
	p.DeleteSecret = deleteSecret

	var expiresAt *time.Time
	if d := ExpiryDuration(req.Expiry); d > 0 {
		t := time.Now().Add(d)
		expiresAt = &t
	}

	if err := s.repo.Create(ctx, p, expiresAt); err != nil {
		return CreateResult{}, fmt.Errorf("paste service create: persist: %w", err)
	}

	return CreateResult{
		Slug:      shareSlug,
		URL:       s.baseURL + "/" + shareSlug,
		DeleteURL: s.baseURL + "/delete/" + shareSlug + "/" + deleteSecret,
		ExpiresAt: expiresAt,
	}, nil
}

// Get retrieves a paste by slug, enforcing expiry, one-use, and password protection.
// On a one-use paste, the paste is atomically consumed on first successful Get.
func (s *Service) Get(ctx context.Context, shareSlug, passwordAttempt string) (GetResult, error) {
	p, err := s.repo.GetBySlug(ctx, shareSlug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return GetResult{}, ErrNotFound
		}
		return GetResult{}, fmt.Errorf("paste service get: lookup: %w", err)
	}

	if p.ExpiresAt != nil && time.Now().After(*p.ExpiresAt) {
		return GetResult{}, ErrExpired
	}

	if p.PasswordHash != "" {
		if passwordAttempt == "" {
			return GetResult{}, ErrWrongPassword
		}
		if err := bcrypt.CompareHashAndPassword([]byte(p.PasswordHash), []byte(passwordAttempt)); err != nil {
			return GetResult{}, ErrWrongPassword
		}
	}

	if p.OneUse {
		consumed, err := s.repo.MarkViewed(ctx, shareSlug)
		if err != nil {
			return GetResult{}, fmt.Errorf("paste service get: mark viewed: %w", err)
		}
		if !consumed {
			return GetResult{}, ErrAlreadyConsumed
		}
	}

	return GetResult{P: p, ExpiresAt: p.ExpiresAt}, nil
}

// Delete removes a paste if the deleteSecret matches.
func (s *Service) Delete(ctx context.Context, shareSlug, deleteSecret string) error {
	p, err := s.repo.GetBySlug(ctx, shareSlug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("paste service delete: lookup: %w", err)
	}

	if subtle.ConstantTimeCompare([]byte(p.DeleteSecret), []byte(deleteSecret)) != 1 {
		return ErrBadDeleteSecret
	}

	if err := s.repo.Delete(ctx, shareSlug); err != nil {
		return fmt.Errorf("paste service delete: remove record: %w", err)
	}

	return nil
}
