package monorepo

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"rev-dep-go/internal/pathutil"
)

func writePkg(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"`+name+`"}`), 0644); err != nil {
		t.Fatalf("write %s: %v", dir, err)
	}
}

// discovered reports whether any standalone dir ends with the given rel suffix.
func discoveredSuffix(dirs []string, suffix string) bool {
	for _, d := range dirs {
		if filepath.ToSlash(d) == suffix || len(d) >= len(suffix) && filepath.ToSlash(d[len(d)-len(suffix):]) == suffix {
			return true
		}
	}
	return false
}

func TestDiscoverStandalonePackages_DiscoversDeepPackages(t *testing.T) {
	root, err := os.MkdirTemp("", "rev-dep-deep")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.RemoveAll(root)

	writePkg(t, filepath.Join(root, "sub", "shallow-pkg"), "shallow")
	// Deeply nested package: the walk is unbounded, so it must still be discovered.
	deep := filepath.Join(root, "a", "b", "c", "d", "e", "f", "g", "h", "i", "j")
	writePkg(t, deep, "deep")

	ctx := NewMonorepoContext(pathutil.NormalizePathForInternal(filepath.Clean(root)))
	dirs := ctx.DiscoverStandalonePackages(nil)

	if !discoveredSuffix(dirs, "sub/shallow-pkg") {
		t.Errorf("expected the shallow package to be discovered, got %v", dirs)
	}
	if !discoveredSuffix(dirs, "i/j") {
		t.Errorf("expected the deep package to be discovered (walk is unbounded), got %v", dirs)
	}
}

func TestDiscoverStandalonePackages_ShallowTree(t *testing.T) {
	root, err := os.MkdirTemp("", "rev-dep-shallow")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.RemoveAll(root)

	writePkg(t, filepath.Join(root, "services", "api"), "api")
	writePkg(t, filepath.Join(root, "services", "web"), "web")

	ctx := NewMonorepoContext(pathutil.NormalizePathForInternal(filepath.Clean(root)))
	dirs := ctx.DiscoverStandalonePackages(nil)

	if len(dirs) != 2 {
		t.Errorf("expected 2 packages, got %v", dirs)
	}
	if !slices.IsSorted(dirs) {
		t.Errorf("expected sorted results, got %v", dirs)
	}
}
