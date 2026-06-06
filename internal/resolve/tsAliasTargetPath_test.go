package resolve

import "testing"

// isValidTsAliasTargetPath must reject absolute targets on every platform. A
// Windows-absolute path ("C:/...") has no leading "/", so the previous
// strings.HasPrefix(path, "/") check let it through; it must now be rejected too.
func TestIsValidTsAliasTargetPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// Absolute targets are rejected (both POSIX and Windows forms).
		{"/abs/path/*", false},
		{"C:/abs/path/*", false},
		{"d:/abs/path/*", false},
		// URLs, node_modules and other-alias targets are rejected.
		{"http://example.com/x", false},
		{"https://example.com/x", false},
		{"node_modules/foo", false},
		{"@some/alias", false},
		// Valid relative / bare / file targets are accepted.
		{"./src/*", true},
		{"../lib/*", true},
		{"src/*", true},
		{"index.ts", true},
	}

	for _, c := range cases {
		if got := isValidTsAliasTargetPath(c.path); got != c.want {
			t.Errorf("isValidTsAliasTargetPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
