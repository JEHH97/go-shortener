package domain

import (
	"strings"
	"testing"
)

func TestNewCode(t *testing.T) {
	code, err := NewCode(8)
	if err != nil || len(code) != 8 {
		t.Fatalf("NewCode: code=%q err=%v", code, err)
	}
	for _, char := range code {
		if !strings.ContainsRune(alphabet, char) {
			t.Fatalf("unexpected character %q", char)
		}
	}
	for _, size := range []int{0, -1, 33} {
		if _, err := NewCode(size); err != ErrInvalidCodeSize {
			t.Errorf("NewCode(%d) error = %v", size, err)
		}
	}
}
