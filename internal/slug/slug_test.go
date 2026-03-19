package slug

import (
	"errors"
	"testing"
)

func TestNewLength(t *testing.T) {
	s, err := New(10)
	if err != nil {
		t.Fatalf("New(10) returned error: %v", err)
	}
	if len(s) != 10 {
		t.Errorf("New(10) returned length %d, want 10", len(s))
	}
}

func TestNewCharset(t *testing.T) {
	for i := 0; i < 100; i++ {
		s, err := New(10)
		if err != nil {
			t.Fatalf("New(10) returned error: %v", err)
		}
		for _, c := range s {
			if !isAlphanumeric(c) {
				t.Errorf("New(10) produced character %q that is not alphanumeric", c)
			}
		}
	}
}

func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func TestNewUniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 10000)
	for i := 0; i < 10000; i++ {
		s, err := New(10)
		if err != nil {
			t.Fatalf("New(10) returned error on iteration %d: %v", i, err)
		}
		if _, exists := seen[s]; exists {
			t.Errorf("New(10) produced a duplicate slug %q on iteration %d", s, i)
		}
		seen[s] = struct{}{}
	}
	if len(seen) != 10000 {
		t.Errorf("generated %d unique slugs, want 10000", len(seen))
	}
}

func TestNewZeroLengthError(t *testing.T) {
	_, err := New(0)
	if err == nil {
		t.Error("New(0) did not return an error, want ErrInvalidLength")
	}
	if !errors.Is(err, ErrInvalidLength) {
		t.Errorf("New(0) returned %v, want ErrInvalidLength", err)
	}
}

func TestNewNegativeLengthError(t *testing.T) {
	_, err := New(-1)
	if err == nil {
		t.Error("New(-1) did not return an error, want ErrInvalidLength")
	}
	if !errors.Is(err, ErrInvalidLength) {
		t.Errorf("New(-1) returned %v, want ErrInvalidLength", err)
	}
}

func TestMustNewPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("MustNew(0) did not panic, expected panic")
		}
	}()
	MustNew(0)
}
