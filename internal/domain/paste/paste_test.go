package paste_test

import (
	"testing"

	"github.com/ahmethakanbesel/pbin/internal/domain/paste"
)

func TestNew_ValidExpiry(t *testing.T) {
	p, err := paste.New("abcdef", "My Title", "some content", "text", "7d", "", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Slug != "abcdef" {
		t.Errorf("expected slug abcdef, got %s", p.Slug)
	}
	if p.Expiry != "7d" {
		t.Errorf("expected expiry 7d, got %s", p.Expiry)
	}
}

func TestNew_InvalidExpiry(t *testing.T) {
	_, err := paste.New("abcdef", "", "some content", "text", "2d", "", false)
	if err == nil {
		t.Fatal("expected ErrInvalidExpiry, got nil")
	}
	if err != paste.ErrInvalidExpiry {
		t.Errorf("expected ErrInvalidExpiry, got %v", err)
	}
}

func TestNew_EmptyContent(t *testing.T) {
	_, err := paste.New("abcdef", "", "", "text", "1h", "", false)
	if err == nil {
		t.Fatal("expected ErrEmptyContent, got nil")
	}
	if err != paste.ErrEmptyContent {
		t.Errorf("expected ErrEmptyContent, got %v", err)
	}
}

func TestNew_EmptySlug(t *testing.T) {
	_, err := paste.New("", "", "some content", "text", "1h", "", false)
	if err == nil {
		t.Fatal("expected ErrEmptySlug, got nil")
	}
	if err != paste.ErrEmptySlug {
		t.Errorf("expected ErrEmptySlug, got %v", err)
	}
}

func TestNew_DefaultLang(t *testing.T) {
	p, err := paste.New("abcdef", "", "some content", "", "1h", "", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Lang != "text" {
		t.Errorf("expected default lang 'text', got %s", p.Lang)
	}
}
