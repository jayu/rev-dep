package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigProcessor_UnresolvedImports(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/configProcessorProject")

	// Create a simple config enabling unresolved imports detection for the whole project
	cfg := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"unresolvedImportsDetection": { "enabled": true }
			}
		]
	}`

	configPath := filepath.Join(testCwd, "unresolved-config.json")
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	configs, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	result, err := ProcessConfig(&configs[0], testCwd, "package.json", "tsconfig.json", false)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	// There should be at least one rule result
	if len(result.RuleResults) == 0 {
		t.Fatalf("Expected at least one rule result")
	}

	// Collect unresolved imports
	found := false
	for _, rr := range result.RuleResults {
		for _, u := range rr.UnresolvedImports {
			if u.Request != "" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		t.Errorf("Expected to find unresolved imports in fixture project, but none were detected")
	}
}
