package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigOutput_Limiting(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-output-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a config that will generate many issues
	configContent := `{
		"configVersion": "1.1",
		"rules": [
			{
				"path": ".",
				"orphanFilesDetection": {"enabled": true},
				"moduleBoundaries": [
					{
						"name": "test-boundary",
						"pattern": "src/**/*",
						"allow": ["src/utils/**/*"]
					}
				]
			}
		]
	}`

	configPath := filepath.Join(tempDir, ".rev-dep.config.jsonc")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create multiple files that will generate issues
	files := []string{
		"src/file1.ts",
		"src/file2.ts",
		"src/file3.ts",
		"src/file4.ts",
		"src/file5.ts",
		"src/file6.ts",
		"src/file7.ts",
		"src/file8.ts",
	}

	for _, file := range files {
		fullPath := filepath.Join(tempDir, file)
		dir := filepath.Dir(fullPath)
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}

		// Create files that import from disallowed locations to generate boundary violations
		content := `import { something } from '../src/boundary/private';`
		if file == "src/file1.ts" {
			content = `// This will be an orphan file - no imports or exports`
		}

		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	// Create boundary directory structure
	boundaryDir := filepath.Join(tempDir, "src", "boundary")
	err = os.MkdirAll(boundaryDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create boundary dir: %v", err)
	}

	privateFile := filepath.Join(boundaryDir, "private.ts")
	err = os.WriteFile(privateFile, []byte(`export const something = 'test';`), 0644)
	if err != nil {
		t.Fatalf("Failed to write private file: %v", err)
	}

	// Load and process config
	configs, err := LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	result, err := ProcessConfig(&configs[0], tempDir, "", "")
	if err != nil {
		t.Fatalf("Failed to process config: %v", err)
	}

	// Test output limiting (default behavior)
	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Capture output with limiting (listAll = false)
	formatAndPrintConfigResults(result, tempDir, false)

	w.Close()
	os.Stdout = originalStdout
	buf.ReadFrom(r)

	output := buf.String()

	// Should show only first 5 issues and mention remaining
	if strings.Count(output, "- src/file") > 5 {
		t.Errorf("Expected at most 5 files to be listed, but found: %d", strings.Count(output, "- src/file"))
	}

	if !strings.Contains(output, "... and") {
		t.Errorf("Expected output to contain '... and' indicating more issues")
	}

	// Test full output (listAll = true)
	buf.Reset()
	r, w, _ = os.Pipe()
	os.Stdout = w

	formatAndPrintConfigResults(result, tempDir, true)

	w.Close()
	os.Stdout = originalStdout
	buf.ReadFrom(r)

	fullOutput := buf.String()

	// Should show all issues
	expectedFileCount := strings.Count(fullOutput, "- src/file")
	if expectedFileCount < len(files)-1 { // -1 because file1.ts is orphan, not boundary violation
		t.Errorf("Expected at least %d files to be listed in full output, but found: %d", len(files)-1, expectedFileCount)
	}

	if strings.Contains(fullOutput, "... and") {
		t.Errorf("Expected full output to not contain '... and' indicating more issues")
	}
}
