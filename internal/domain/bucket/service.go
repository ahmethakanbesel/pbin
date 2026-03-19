package bucket

import (
	"archive/zip"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/filestore"
	"github.com/ahmethakanbesel/pbin/internal/slug"
	"golang.org/x/crypto/bcrypt"
)

const (
	slugLength         = 12
	deleteSecretLength = 24
	bcryptCost         = 12
)

// Service implements bucket creation, download, and deletion business rules.
type Service struct {
	repo    Repository
	store   filestore.Backend
	baseURL string
}

func NewService(repo Repository, store filestore.Backend, baseURL string) *Service {
	return &Service{repo: repo, store: store, baseURL: baseURL}
}

// FileInput carries one file within a create request.
type FileInput struct {
	Filename string
	MimeType string
	Size     int64
	Content  io.Reader
}

// CreateRequest carries all inputs for bucket creation.
type CreateRequest struct {
	Files    []FileInput
	Expiry   string
	Password string
	OneUse   bool
}

// FileInfo is a file entry in the CreateResult.
type FileInfo struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

// CreateResult is returned on successful bucket creation.
type CreateResult struct {
	Slug      string
	URL       string
	DeleteURL string
	ExpiresAt *time.Time
	FileCount int
	Files     []FileInfo
}

// GetResult is returned on successful bucket retrieval (without consuming one-use).
type GetResult struct {
	B         Bucket
	ExpiresAt *time.Time
}

// Create stores all files and inserts the bucket record.
// Files are written to filestore before DB insertion — on DB failure the stored files are cleaned up best-effort.
func (s *Service) Create(ctx context.Context, req CreateRequest) (CreateResult, error) {
	bucketSlug, err := slug.New(slugLength)
	if err != nil {
		return CreateResult{}, fmt.Errorf("bucket service create: generate slug: %w", err)
	}

	deleteSecret, err := slug.New(deleteSecretLength)
	if err != nil {
		return CreateResult{}, fmt.Errorf("bucket service create: generate delete secret: %w", err)
	}

	var passwordHash string
	if req.Password != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
		if err != nil {
			return CreateResult{}, fmt.Errorf("bucket service create: hash password: %w", err)
		}
		passwordHash = string(h)
	}

	b, err := New(bucketSlug, deleteSecret, passwordHash, req.Expiry, req.OneUse)
	if err != nil {
		return CreateResult{}, fmt.Errorf("bucket service create: construct entity: %w", err)
	}

	var expiresAt *time.Time
	if d := ExpiryDuration(req.Expiry); d > 0 {
		t := time.Now().Add(d)
		expiresAt = &t
	}

	// Write all files to filestore first; track storage keys for rollback.
	type storedFile struct {
		storageKey string
		bf         BucketFile
	}
	var stored []storedFile

	for _, fi := range req.Files {
		storageKey, err := slug.New(slugLength)
		if err != nil {
			// Best-effort cleanup of already-written files.
			for _, sf := range stored {
				_ = s.store.Delete(ctx, sf.storageKey)
			}
			return CreateResult{}, fmt.Errorf("bucket service create: generate file key: %w", err)
		}

		if err := s.store.Write(ctx, storageKey, fi.Content); err != nil {
			for _, sf := range stored {
				_ = s.store.Delete(ctx, sf.storageKey)
			}
			return CreateResult{}, fmt.Errorf("bucket service create: write file %q: %w", fi.Filename, err)
		}

		mimeType := fi.MimeType
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		stored = append(stored, storedFile{
			storageKey: storageKey,
			bf: BucketFile{
				BucketSlug: bucketSlug,
				Filename:   fi.Filename,
				Size:       fi.Size,
				MimeType:   mimeType,
				StorageKey: storageKey,
			},
		})
	}

	// Insert bucket row.
	if err := s.repo.Create(ctx, b, expiresAt); err != nil {
		for _, sf := range stored {
			_ = s.store.Delete(ctx, sf.storageKey)
		}
		return CreateResult{}, fmt.Errorf("bucket service create: persist bucket: %w", err)
	}

	// Insert bucket_file rows.
	var fileInfos []FileInfo
	for _, sf := range stored {
		if err := s.repo.AddFile(ctx, sf.bf); err != nil {
			// Bucket row exists but files are partially inserted — log and return error.
			return CreateResult{}, fmt.Errorf("bucket service create: persist file %q: %w", sf.bf.Filename, err)
		}
		fileInfos = append(fileInfos, FileInfo{
			Filename: sf.bf.Filename,
			Size:     sf.bf.Size,
			MimeType: sf.bf.MimeType,
		})
	}

	return CreateResult{
		Slug:      bucketSlug,
		URL:       s.baseURL + "/b/" + bucketSlug,
		DeleteURL: s.baseURL + "/b/delete/" + bucketSlug + "/" + deleteSecret,
		ExpiresAt: expiresAt,
		FileCount: len(stored),
		Files:     fileInfos,
	}, nil
}

// GetMeta retrieves bucket metadata and file list without consuming one-use.
func (s *Service) GetMeta(ctx context.Context, bucketSlug, passwordAttempt string) (GetResult, error) {
	b, err := s.repo.GetBySlug(ctx, bucketSlug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return GetResult{}, ErrNotFound
		}
		return GetResult{}, fmt.Errorf("bucket service get meta: lookup: %w", err)
	}

	if b.ExpiresAt != nil && time.Now().After(*b.ExpiresAt) {
		return GetResult{}, ErrExpired
	}

	if b.PasswordHash != "" {
		if passwordAttempt == "" {
			return GetResult{}, ErrWrongPassword
		}
		if err := bcrypt.CompareHashAndPassword([]byte(b.PasswordHash), []byte(passwordAttempt)); err != nil {
			return GetResult{}, ErrWrongPassword
		}
	}

	return GetResult{B: b, ExpiresAt: b.ExpiresAt}, nil
}

// StreamZIP writes all bucket files as a streaming ZIP to w.
// Enforces expiry, password, and atomic one-use semantics.
// The caller must NOT set Content-Length before calling (chunked transfer).
func (s *Service) StreamZIP(ctx context.Context, bucketSlug, passwordAttempt string, w http.ResponseWriter) error {
	b, err := s.repo.GetBySlug(ctx, bucketSlug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("bucket service stream zip: lookup: %w", err)
	}

	if b.ExpiresAt != nil && time.Now().After(*b.ExpiresAt) {
		return ErrExpired
	}

	if b.PasswordHash != "" {
		if passwordAttempt == "" {
			return ErrWrongPassword
		}
		if err := bcrypt.CompareHashAndPassword([]byte(b.PasswordHash), []byte(passwordAttempt)); err != nil {
			return ErrWrongPassword
		}
	}

	if b.OneUse {
		consumed, err := s.repo.MarkDownloaded(ctx, bucketSlug)
		if err != nil {
			return fmt.Errorf("bucket service stream zip: mark downloaded: %w", err)
		}
		if !consumed {
			return ErrAlreadyConsumed
		}
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, bucketSlug))

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, bf := range b.Files {
		fw, err := zw.Create(bf.Filename)
		if err != nil {
			return fmt.Errorf("bucket service stream zip: create entry %q: %w", bf.Filename, err)
		}
		rc, err := s.store.Read(ctx, bf.StorageKey)
		if err != nil {
			return fmt.Errorf("bucket service stream zip: read file %q: %w", bf.Filename, err)
		}
		_, copyErr := io.Copy(fw, rc)
		rc.Close()
		if copyErr != nil {
			return fmt.Errorf("bucket service stream zip: stream file %q: %w", bf.Filename, copyErr)
		}
	}

	return nil
}

// Delete removes a bucket and all its files if the deleteSecret matches.
func (s *Service) Delete(ctx context.Context, bucketSlug, deleteSecret string) error {
	b, err := s.repo.GetBySlug(ctx, bucketSlug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("bucket service delete: lookup: %w", err)
	}

	if subtle.ConstantTimeCompare([]byte(b.DeleteSecret), []byte(deleteSecret)) != 1 {
		return ErrBadDeleteSecret
	}

	// Delete on-disk files first (best-effort per file).
	for _, bf := range b.Files {
		_ = s.store.Delete(ctx, bf.StorageKey)
	}

	if err := s.repo.Delete(ctx, bucketSlug); err != nil {
		return fmt.Errorf("bucket service delete: remove record: %w", err)
	}

	return nil
}
