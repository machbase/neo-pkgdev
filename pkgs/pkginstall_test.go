package pkgs

import (
	"path/filepath"
	"testing"

	"github.com/machbase/neo-pkgdev/pkgs/untar"
)

func TestStripComponents(t *testing.T) {
	tests := []struct {
		name            string
		stripComponents int
		expected        string
	}{
		{
			name:            "/a/b/c",
			stripComponents: 0,
			expected:        "/a/b/c",
		},
		{
			name:            "/a/b/c",
			stripComponents: 1,
			expected:        "b/c",
		},
		{
			name:            "/a/b/c",
			stripComponents: 2,
			expected:        "c",
		},
	}
	for _, tt := range tests {
		rel := filepath.FromSlash(tt.name)
		rel = untar.StripComponents(rel, tt.stripComponents)
		if rel != filepath.FromSlash(tt.expected) {
			t.Errorf("expected %q, got %q", tt.expected, rel)
			t.Fail()
		}
	}
}
