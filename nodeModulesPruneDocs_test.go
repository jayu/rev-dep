package main

import (
	"os"
	"path/filepath"
	"testing"
)

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestNodeModulesPruneDocsCmd_DefaultPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "node_modules", "dep1")

	mustWriteFile(t, filepath.Join(pkgDir, "package.json"), `{"name":"dep1","version":"1.0.0"}`)
	mustWriteFile(t, filepath.Join(pkgDir, "README.md"), "readme")
	mustWriteFile(t, filepath.Join(pkgDir, "LICENSE"), "license")
	mustWriteFile(t, filepath.Join(pkgDir, "docs", "guide.txt"), "docs")
	mustWriteFile(t, filepath.Join(pkgDir, "index.js"), "module.exports = 1")

	_, err := NodeModulesPruneDocsCmd(tmpDir, []string{}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exists(filepath.Join(pkgDir, "README.md")) {
		t.Fatalf("expected README.md to be removed")
	}
	if exists(filepath.Join(pkgDir, "LICENSE")) {
		t.Fatalf("expected LICENSE to be removed")
	}
	if exists(filepath.Join(pkgDir, "docs", "guide.txt")) {
		t.Fatalf("expected docs/** files to be removed")
	}
	if !exists(filepath.Join(pkgDir, "index.js")) {
		t.Fatalf("expected index.js to stay")
	}
}

func TestNodeModulesPruneDocsCmd_CustomPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "node_modules", "@scope", "dep2")

	mustWriteFile(t, filepath.Join(pkgDir, "package.json"), `{"name":"@scope/dep2","version":"1.0.0"}`)
	mustWriteFile(t, filepath.Join(pkgDir, "notes.md"), "notes")
	mustWriteFile(t, filepath.Join(pkgDir, "README.md"), "readme")
	mustWriteFile(t, filepath.Join(pkgDir, "LICENSE"), "license")

	_, err := NodeModulesPruneDocsCmd(tmpDir, []string{"*.md"}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exists(filepath.Join(pkgDir, "notes.md")) {
		t.Fatalf("expected *.md file to be removed")
	}
	if exists(filepath.Join(pkgDir, "README.md")) {
		t.Fatalf("expected README.md to be removed by *.md")
	}
	if !exists(filepath.Join(pkgDir, "LICENSE")) {
		t.Fatalf("expected LICENSE to stay")
	}
}

func TestNodeModulesPruneDocsCmd_RequiresPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := NodeModulesPruneDocsCmd(tmpDir, []string{}, false)
	if err == nil {
		t.Fatalf("expected error when no patterns are provided")
	}
}
