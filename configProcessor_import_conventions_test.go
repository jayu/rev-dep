package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigProcessor_ImportConventions(t *testing.T) {
	// Change to the test fixture directory
	tempDir, _ := filepath.Abs(filepath.Join(".", "__fixtures__", "importConventionsProject"))
	originalCwd, _ := os.Getwd()
	defer os.Chdir(originalCwd)
	os.Chdir(tempDir)

	// Read and parse the config
	configBytes, err := os.ReadFile("rev-dep.config.json")
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	configs, err := ParseConfig(configBytes)
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	if len(configs) == 0 {
		t.Fatalf("Expected at least 1 config, got %d", len(configs))
	}

	config := &configs[0]

	// Process the config
	result, err := ProcessConfig(config, tempDir, "package.json", "tsconfig.json")
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	// Check that we have results
	if len(result.RuleResults) == 0 {
		t.Fatalf("Expected at least 1 rule result, got %d", len(result.RuleResults))
	}

	ruleResult := result.RuleResults[0]

	// Check that import-conventions is in enabled checks
	hasImportConventions := false
	for _, check := range ruleResult.EnabledChecks {
		if check == "import-conventions" {
			hasImportConventions = true
			break
		}
	}
	if !hasImportConventions {
		t.Error("Expected 'import-conventions' to be in enabled checks")
	}

	// Check that we have import convention violations
	if len(ruleResult.ImportConventionViolations) == 0 {
		t.Error("Expected import convention violations, got none")
	}

	// Check for specific violation types
	violationTypes := make(map[string]bool)
	for _, violation := range ruleResult.ImportConventionViolations {
		violationTypes[violation.ViolationType] = true
	}

	// We should have "should-be-relative" violations (intra-domain aliases)
	if !violationTypes["should-be-relative"] {
		t.Error("Expected 'should-be-relative' violations")
	}

	// We should have "should-be-aliased" violations (inter-domain relative paths)
	if !violationTypes["should-be-aliased"] {
		t.Error("Expected 'should-be-aliased' violations")
	}

	// Check that HasFailures is true when violations exist
	if !result.HasFailures {
		t.Error("Expected HasFailures to be true when violations exist")
	}
}
