package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModuleBoundariesCmdFn(t *testing.T) {
	cwd, _ := filepath.Abs("__fixtures__/moduleBoundariesProject")
	configPath := filepath.Join(cwd, "rev-dep.config.json")

	// Test case: Should fail due to violations
	report, hasViolations, err := moduleBoundariesCmdFn(cwd, configPath, "", "", []string{}, true)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !hasViolations {
		t.Error("Expected hasViolations to be true, got false")
	}

	if report == "" {
		t.Error("Expected a violation report, got empty string")
	}

	// Optional: verify report content
	if !strings.Contains(report, "Violation [Client Boundary]") {
		t.Errorf("Report missing expected violation string. Got: %s", report)
	}
}

func TestConfigValidation(t *testing.T) {
	// Create a temporary invalid config file
	cwd, _ := os.Getwd()
	invalidConfig := `
	{
		"module_boundaries": [
			{
				"name": "Invalid Boundary",
				"pattern": "./packages/client/**",
				"allow": [],
				"deny": []
			}
		]
	}
	`
	tmpConfigPath := filepath.Join(cwd, "invalid-rev-dep.config.json")
	if err := os.WriteFile(tmpConfigPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("Failed to create temp config: %v", err)
	}
	defer os.Remove(tmpConfigPath)

	// Test case: Should error due to validation
	_, _, err := moduleBoundariesCmdFn(cwd, tmpConfigPath, "", "", []string{}, true)

	if err == nil {
		t.Error("Expected validation error, got nil")
	} else if !strings.Contains(err.Error(), "starts with './' or '.\\', which is not allowed") {
		t.Errorf("Expected relative path validation error, got: %v", err)
	}
}
