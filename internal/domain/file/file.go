package file

import (
	"errors"
	"time"
)

var (
	ErrInvalidExpiry = errors.New("file: expiry must be a valid preset")
	ErrEmptySlug     = errors.New("file: slug must not be empty")
)

// validExpiries maps preset string to duration (0 = never).
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

// File is the core domain entity for a single uploaded file.
type File struct {
	Slug         string
	Filename     string // original filename (stored in DB, never used as disk key)
	Size         int64
	MimeType     string
	PasswordHash string // bcrypt hash if password-protected; empty otherwise
	OneUse       bool
	Expiry       string // preset string e.g. "1d"
}

// New validates and constructs a File. Slug and expiry are required.
func New(slug, filename, mimeType, expiry string, size int64, passwordHash string, oneUse bool) (File, error) {
	if slug == "" {
		return File{}, ErrEmptySlug
	}
	if _, ok := validExpiries[expiry]; !ok {
		return File{}, ErrInvalidExpiry
	}
	return File{
		Slug:         slug,
		Filename:     filename,
		Size:         size,
		MimeType:     mimeType,
		PasswordHash: passwordHash,
		OneUse:       oneUse,
		Expiry:       expiry,
	}, nil
}

// ExpiryDuration converts an expiry preset string to a time.Duration.
// Returns 0 for "never". Panics if expiry is not a valid preset (caller must validate first).
func ExpiryDuration(expiry string) time.Duration {
	d, ok := validExpiries[expiry]
	if !ok {
		panic("file.ExpiryDuration: invalid expiry preset: " + expiry)
	}
	return d
}
