package bucket

import (
	"errors"
	"time"
)

var (
	ErrInvalidExpiry = errors.New("bucket: expiry must be a valid preset")
	ErrEmptySlug     = errors.New("bucket: slug must not be empty")
)

var (
	ErrNotFound        = errors.New("bucket: not found")
	ErrExpired         = errors.New("bucket: expired")
	ErrAlreadyConsumed = errors.New("bucket: one-use bucket already consumed")
	ErrWrongPassword   = errors.New("bucket: wrong password")
	ErrBadDeleteSecret = errors.New("bucket: invalid delete secret")
)

var validExpiries = map[string]time.Duration{
	"10m":   10 * time.Minute,
	"1h":    time.Hour,
	"6h":    6 * time.Hour,
	"1d":    24 * time.Hour,
	"7d":    7 * 24 * time.Hour,
	"30d":   30 * 24 * time.Hour,
	"90d":   90 * 24 * time.Hour,
	"1y":    365 * 24 * time.Hour,
	"never": 0,
}

// Bucket is a multi-file transfer container.
type Bucket struct {
	Slug         string
	PasswordHash string
	OneUse       bool
	Expiry       string
	DeleteSecret string
	ExpiresAt    *time.Time
	Files        []BucketFile
}

// BucketFile represents a single file within a Bucket.
// StorageKey is the on-disk slug (never the user-supplied filename).
type BucketFile struct {
	ID         int64
	BucketSlug string
	Filename   string  // original name for ZIP entry
	Size       int64
	MimeType   string
	StorageKey string // slug used as on-disk key
}

// New validates and constructs a Bucket.
func New(slug, deleteSecret, passwordHash, expiry string, oneUse bool) (Bucket, error) {
	if slug == "" {
		return Bucket{}, ErrEmptySlug
	}
	if _, ok := validExpiries[expiry]; !ok {
		return Bucket{}, ErrInvalidExpiry
	}
	return Bucket{
		Slug:         slug,
		DeleteSecret: deleteSecret,
		PasswordHash: passwordHash,
		OneUse:       oneUse,
		Expiry:       expiry,
	}, nil
}

// ExpiryDuration converts expiry preset to time.Duration (0 = never).
func ExpiryDuration(expiry string) time.Duration {
	d, ok := validExpiries[expiry]
	if !ok {
		panic("bucket.ExpiryDuration: invalid expiry preset: " + expiry)
	}
	return d
}
