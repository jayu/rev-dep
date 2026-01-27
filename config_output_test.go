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

func TestConfigOutput_CLIFlagConsistency(t *testing.T) {
	// Test that the CLI flag uses "issues" terminology consistently
	tempDir, err := os.MkdirTemp("", "rev-dep-cli-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple config
	configContent := `{
		"configVersion": "1.1",
		"rules": [
			{
				"path": ".",
				"orphanFilesDetection": {"enabled": true}
			}
		]
	}`

	configPath := filepath.Join(tempDir, ".rev-dep.config.jsonc")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create an orphan file
	orphanFile := filepath.Join(tempDir, "orphan.ts")
	err = os.WriteFile(orphanFile, []byte(`// This is an orphan file`), 0644)
	if err != nil {
		t.Fatalf("Failed to write orphan file: %v", err)
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

	// Test that the function parameter works correctly with both true and false
	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test with listAll=false (limited output)
	formatAndPrintConfigResults(result, tempDir, false)

	w.Close()
	os.Stdout = originalStdout
	buf.ReadFrom(r)

	limitedOutput := buf.String()

	// Should contain issues terminology
	if !strings.Contains(limitedOutput, "Issues") {
		t.Errorf("Expected limited output to contain 'Issues' terminology")
	}

	// Test with listAll=true (full output)
	buf.Reset()
	r, w, _ = os.Pipe()
	os.Stdout = w

	formatAndPrintConfigResults(result, tempDir, true)

	w.Close()
	os.Stdout = originalStdout
	buf.ReadFrom(r)

	fullOutput := buf.String()

	// Should also contain issues terminology
	if !strings.Contains(fullOutput, "Issues") {
		t.Errorf("Expected full output to contain 'Issues' terminology")
	}
}

func TestConfigOutput_Terminology(t *testing.T) {
	// Test that the output uses "issues" terminology consistently
	tempDir, err := os.MkdirTemp("", "rev-dep-terminology-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple config
	configContent := `{
		"configVersion": "1.1",
		"rules": [
			{
				"path": ".",
				"orphanFilesDetection": {"enabled": true}
			}
		]
	}`

	configPath := filepath.Join(tempDir, ".rev-dep.config.jsonc")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create an orphan file
	orphanFile := filepath.Join(tempDir, "orphan.ts")
	err = os.WriteFile(orphanFile, []byte(`// This is an orphan file`), 0644)
	if err != nil {
		t.Fatalf("Failed to write orphan file: %v", err)
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

	// Capture output
	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatAndPrintConfigResults(result, tempDir, false)

	w.Close()
	os.Stdout = originalStdout
	buf.ReadFrom(r)

	output := buf.String()

	// Check for "Issues" terminology
	if !strings.Contains(output, "Orphan Files Issues") {
		t.Errorf("Expected output to contain 'Orphan Files Issues', got: %s", output)
	}

	// Check that it doesn't contain old "Violations" terminology for this check
	if strings.Contains(output, "Orphan Files (") && !strings.Contains(output, "Orphan Files Issues (") {
		t.Errorf("Expected output to use 'Issues' terminology, found old format")
	}
}
