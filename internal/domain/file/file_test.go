package file_test

import (
	"testing"
	"time"

	"github.com/ahmethakanbesel/pbin/internal/domain/file"
)

func TestNew_ValidExpiry(t *testing.T) {
	f, err := file.New("abcdef", "test.txt", "text/plain", "1h", 100, "", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if f.Slug != "abcdef" {
		t.Errorf("expected slug abcdef, got %s", f.Slug)
	}
	if f.Expiry != "1h" {
		t.Errorf("expected expiry 1h, got %s", f.Expiry)
	}
}

func TestNew_InvalidExpiry(t *testing.T) {
	_, err := file.New("abcdef", "test.txt", "text/plain", "2h", 100, "", false)
	if err == nil {
		t.Fatal("expected ErrInvalidExpiry, got nil")
	}
	if err != file.ErrInvalidExpiry {
		t.Errorf("expected ErrInvalidExpiry, got %v", err)
	}
}

func TestNew_EmptySlug(t *testing.T) {
	_, err := file.New("", "test.txt", "text/plain", "1h", 100, "", false)
	if err == nil {
		t.Fatal("expected ErrEmptySlug, got nil")
	}
	if err != file.ErrEmptySlug {
		t.Errorf("expected ErrEmptySlug, got %v", err)
	}
}

func TestExpiryDuration(t *testing.T) {
	tests := []struct {
		expiry string
		want   time.Duration
	}{
		{"10m", 10 * time.Minute},
		{"never", 0},
		{"1y", 365 * 24 * time.Hour},
		{"1h", time.Hour},
		{"7d", 7 * 24 * time.Hour},
	}
	for _, tc := range tests {
		got := file.ExpiryDuration(tc.expiry)
		if got != tc.want {
			t.Errorf("ExpiryDuration(%q) = %v, want %v", tc.expiry, got, tc.want)
		}
	}
}
