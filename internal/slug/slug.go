package slug

import (
	"crypto/rand"
	"errors"
	"math/big"
)

// charset is the alphabet for generated slugs.
// URL-safe (no +, /, = from base64), unambiguous (no 0/O, 1/l/I excluded).
const charset = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var charsetLen = big.NewInt(int64(len(charset)))

// ErrInvalidLength is returned when n <= 0.
var ErrInvalidLength = errors.New("slug: length must be greater than zero")

// New generates a cryptographically random URL-safe slug of length n.
// Returns ErrInvalidLength if n <= 0.
func New(n int) (string, error) {
	if n <= 0 {
		return "", ErrInvalidLength
	}
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		b[i] = charset[idx.Int64()]
	}
	return string(b), nil
}

// MustNew generates a slug of length n and panics on error.
// Intended for use in tests and init code where errors are unrecoverable.
func MustNew(n int) string {
	s, err := New(n)
	if err != nil {
		panic(err)
	}
	return s
}
