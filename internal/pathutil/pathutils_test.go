package pathutil

import (
	"path/filepath"
	"testing"
)

func TestIsAbsoluteInternalPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// POSIX absolute
		{"/repo/src/file.ts", true},
		{"/", true},
		// Windows absolute (internal form keeps the drive, forward slashes)
		{"C:/repo/src/file.ts", true},
		{"d:/x", true},
		// Not absolute: relative paths and bare module specifiers
		{"src/file.ts", false},
		{"./src/file.ts", false},
		{"../src/file.ts", false},
		{"react-dom/client", false},
		{"@scope/pkg", false},
		{"", false},
	}

	for _, c := range cases {
		if got := IsAbsoluteInternalPath(c.path); got != c.want {
			t.Errorf("IsAbsoluteInternalPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestConfigRoot(t *testing.T) {
	cases := []struct {
		configPath string
		want       string
	}{
		{"/repo/.rev-dep.config.jsonc", "/repo"},
		{"/repo/.config/rev-dep.jsonc", "/repo"},
		{"/repo/.config/rev-dep.config.json", "/repo"},
	}

	for _, c := range cases {
		if got := ConfigRoot(filepath.FromSlash(c.configPath)); got != filepath.FromSlash(c.want) {
			t.Errorf("ConfigRoot(%q) = %q, want %q", c.configPath, got, c.want)
		}
	}
}
