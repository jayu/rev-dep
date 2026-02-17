package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigRun_RulesFilter(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rev-dep-rules-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a dummy project structure
	os.MkdirAll(filepath.Join(tempDir, "src/features/auth"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "src/features/users"), 0755)
	os.WriteFile(filepath.Join(tempDir, "src/features/auth/index.ts"), []byte("export const a = 1;"), 0644)
	os.WriteFile(filepath.Join(tempDir, "src/features/users/index.ts"), []byte("export const b = 2;"), 0644)
	os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name":"test"}`), 0644)

	// Create a config with 2 rules
	configContent := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": "src/features/auth",
				"orphanFilesDetection": {
					"enabled": true,
					"validEntryPoints": ["index.ts"]
				}
			},
			{
				"path": "src/features/users",
				"orphanFilesDetection": {
					"enabled": true,
					"validEntryPoints": ["index.ts"]
				}
			}
		]
	}`
	os.WriteFile(filepath.Join(tempDir, "rev-dep.config.json"), []byte(configContent), 0644)

	// 1. Run with --rules src/features/auth
	// We need to reset the flags and variables because they are global
	runConfigRules = []string{"src/features/auth"}
	runConfigCwd = tempDir
	runConfigListAll = true
	runConfigFix = false

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute the command logic (we'll call the RunE function logic)
	// Instead of calling cmd.Execute(), let's just call the RunE function directly
	err = configRunCmd.RunE(configRunCmd, []string{})
	if err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("config run failed: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Rule: src/features/auth") {
		t.Errorf("Expected output to contain rule src/features/auth, but got: %s", output)
	}
	if strings.Contains(output, "Rule: src/features/users") {
		t.Errorf("Expected output NOT to contain rule src/features/users, but got: %s", output)
	}

	// 2. Run with multiple rules --rules src/features/auth,src/features/users
	runConfigRules = []string{"src/features/auth", "src/features/users"}
	r, w, _ = os.Pipe()
	os.Stdout = w

	err = configRunCmd.RunE(configRunCmd, []string{})
	if err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("config run failed: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout
	buf.Reset()
	buf.ReadFrom(r)
	output = buf.String()

	if !strings.Contains(output, "Rule: src/features/auth") {
		t.Errorf("Expected output to contain rule src/features/auth, but got: %s", output)
	}
	if !strings.Contains(output, "Rule: src/features/users") {
		t.Errorf("Expected output to contain rule src/features/users, but got: %s", output)
	}

	// 3. Run with --rules src/features/non-existent
	runConfigRules = []string{"src/features/non-existent"}
	err = configRunCmd.RunE(configRunCmd, []string{})
	if err == nil {
		t.Errorf("Expected error for non-existent rule, but got nil")
	} else if !strings.Contains(err.Error(), "none of the requested rules") {
		t.Errorf("Expected error message to contain 'none of the requested rules', but got: %v", err)
	}
}
