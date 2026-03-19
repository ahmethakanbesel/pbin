package paste

import (
	"errors"
	"time"
)

var (
	ErrInvalidExpiry   = errors.New("paste: expiry must be a valid preset")
	ErrEmptyContent    = errors.New("paste: content must not be empty")
	ErrEmptySlug       = errors.New("paste: slug must not be empty")
	ErrNotFound        = errors.New("paste: not found")
	ErrExpired         = errors.New("paste: expired")
	ErrAlreadyConsumed = errors.New("paste: one-use paste already consumed")
	ErrWrongPassword   = errors.New("paste: wrong password")
	ErrBadDeleteSecret = errors.New("paste: invalid delete secret")
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

// Paste is the core domain entity for a text paste.
type Paste struct {
	Slug         string
	Title        string
	Content      string
	Lang         string // language hint for highlighting; "text" = no highlighting
	PasswordHash string
	OneUse       bool
	Expiry       string
	DeleteSecret string
	ExpiresAt    *time.Time
}

// New validates and constructs a Paste.
func New(slug, title, content, lang, expiry, passwordHash string, oneUse bool) (Paste, error) {
	if slug == "" {
		return Paste{}, ErrEmptySlug
	}
	if content == "" {
		return Paste{}, ErrEmptyContent
	}
	if _, ok := validExpiries[expiry]; !ok {
		return Paste{}, ErrInvalidExpiry
	}
	if lang == "" {
		lang = "text"
	}
	return Paste{
		Slug:         slug,
		Title:        title,
		Content:      content,
		Lang:         lang,
		PasswordHash: passwordHash,
		OneUse:       oneUse,
		Expiry:       expiry,
	}, nil
}

// ExpiryDuration converts expiry preset to time.Duration (0 = never).
func ExpiryDuration(expiry string) time.Duration {
	d, ok := validExpiries[expiry]
	if !ok {
		panic("paste.ExpiryDuration: invalid expiry preset: " + expiry)
	}
	return d
}
