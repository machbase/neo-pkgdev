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
		{
			name:            "build/",
			stripComponents: 1,
			expected:        "",
		},
		{
			name:            "build/vite.svg",
			stripComponents: 1,
			expected:        "vite.svg",
		},
		{
			name:            "build/assets/",
			stripComponents: 1,
			expected:        "assets/",
		},
		{
			name:            "build/assets/index-00d92eee.js",
			stripComponents: 1,
			expected:        "assets/index-00d92eee.js",
		},
		{
			name:            "build/assets/index-903374e8.css",
			stripComponents: 1,
			expected:        "assets/index-903374e8.css",
		},
		{
			name:            "build/index.html",
			stripComponents: 1,
			expected:        "index.html",
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
