package testutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// RepoRoot walks up from the current working directory until it finds go.mod.
func RepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found in parent directories")
		}
		dir = parent
	}
}

// GoldenPath returns an absolute path to a golden file in the repo testdata directory.
func GoldenPath(name string) (string, error) {
	root, err := RepoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "testdata", name), nil
}

// FixturePath returns an absolute path inside the repo __fixtures__ directory.
func FixturePath(parts ...string) (string, error) {
	root, err := RepoRoot()
	if err != nil {
		return "", err
	}
	all := append([]string{root, "__fixtures__"}, parts...)
	return filepath.Join(all...), nil
}
