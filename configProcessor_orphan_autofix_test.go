package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigProcessor_OrphanFiles_Autofix(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-orphan-autofix-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	srcDir := filepath.Join(tempDir, "src")
	os.MkdirAll(srcDir, 0755)

	// Create an entry point
	entryFile := filepath.Join(srcDir, "index.ts")
	os.WriteFile(entryFile, []byte("import './used';"), 0644)

	// Create a used file
	usedFile := filepath.Join(srcDir, "used.ts")
	os.WriteFile(usedFile, []byte("export const a = 1;"), 0644)

	// Create an orphan file
	orphanFile := filepath.Join(srcDir, "orphan.ts")
	os.WriteFile(orphanFile, []byte("export const b = 2;"), 0644)

	// Create rev-dep config with orphan files autofix enabled
	config := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{
			{
				Path: ".",
				OrphanFilesDetection: &OrphanFilesOptions{
					Enabled:          true,
					ValidEntryPoints: []string{"src/index.ts"},
					Autofix:          true,
				},
			},
		},
	}

	// 1. Verify fixable issues count when fix=false
	result, err := ProcessConfig(&config, tempDir, "", "", false)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	if result.FixableIssuesCount != 1 {
		t.Errorf("Expected 1 fixable issue (the orphan file), got %d", result.FixableIssuesCount)
	}

	if _, err := os.Stat(orphanFile); os.IsNotExist(err) {
		t.Errorf("Orphan file should NOT be removed yet")
	}

	// 2. Process with fix=true and verify removal
	result, err = ProcessConfig(&config, tempDir, "", "", true)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	if result.DeletedFilesCount != 1 {
		t.Errorf("Expected 1 deleted file, got %d", result.DeletedFilesCount)
	}

	if _, err := os.Stat(orphanFile); !os.IsNotExist(err) {
		t.Errorf("Expected orphan file to be removed, but it still exists")
	}

	// Verify used files still exist
	if _, err := os.Stat(entryFile); err != nil {
		t.Errorf("Entry file should still exist: %v", err)
	}
	if _, err := os.Stat(usedFile); err != nil {
		t.Errorf("Used file should still exist: %v", err)
	}
}

func TestConfigProcessor_OrphanFiles_NoAutofix(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rev-dep-orphan-noautofix-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srcDir := filepath.Join(tempDir, "src")
	os.MkdirAll(srcDir, 0755)

	entryFile := filepath.Join(srcDir, "index.ts")
	os.WriteFile(entryFile, []byte("export const a = 1;"), 0644)

	orphanFile := filepath.Join(srcDir, "orphan.ts")
	os.WriteFile(orphanFile, []byte("export const b = 2;"), 0644)

	// Autofix is DISABLED
	config := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{
			{
				Path: ".",
				OrphanFilesDetection: &OrphanFilesOptions{
					Enabled:          true,
					ValidEntryPoints: []string{"src/index.ts"},
					Autofix:          false,
				},
			},
		},
	}

	// 1. Verify fixable issues count is 0 when autofix is disabled
	result, err := ProcessConfig(&config, tempDir, "", "", false)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	if result.FixableIssuesCount != 0 {
		t.Errorf("Expected 0 fixable issues when autofix is disabled, got %d", result.FixableIssuesCount)
	}

	// 2. Even with fix=true, nothing should be deleted
	result, err = ProcessConfig(&config, tempDir, "", "", true)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	if result.DeletedFilesCount != 0 {
		t.Errorf("Expected 0 deleted files, got %d", result.DeletedFilesCount)
	}

	if _, err := os.Stat(orphanFile); os.IsNotExist(err) {
		t.Errorf("Orphan file should NOT be removed when autofix is disabled")
	}
}
