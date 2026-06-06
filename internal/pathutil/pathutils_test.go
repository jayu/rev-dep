package pathutil

import "testing"

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
