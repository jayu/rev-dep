package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigProcessor_ImportConventionsShortcut(t *testing.T) {
	// Change to the test fixture directory
	tempDir, _ := filepath.Abs(filepath.Join(".", "__fixtures__", "importConventionsShortcutProject"))
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

	// Check violations
	foundShouldBeAliased := false
	foundShouldBeRelative := false
	foundWrongAlias := false

	for _, violation := range ruleResult.ImportConventionViolations {
		t.Logf("Violation: %s in %s (Type: %s, Source: %s, Target: %s, Expected: %s)",
			violation.ImportRequest, violation.FilePath, violation.ViolationType,
			violation.SourceDomain, violation.TargetDomain, violation.ExpectedPattern)

		if violation.ViolationType == "should-be-aliased" &&
			violation.ImportRequest == "../shared" {
			foundShouldBeAliased = true
			// Check if expected pattern contains an inferred alias (starts with @)
			if !strings.HasPrefix(violation.ExpectedPattern, "@") {
				t.Errorf("Expected pattern for ../shared to be an alias, got '%s'", violation.ExpectedPattern)
			}
		}
		if violation.ViolationType == "should-be-relative" &&
			violation.ImportRequest == "@retail/utils" {
			foundShouldBeRelative = true
		}
		if violation.ViolationType == "wrong-alias" {
			foundWrongAlias = true
		}
	}

	if !foundShouldBeAliased {
		t.Error("Expected 'should-be-aliased' violation for '../shared'")
	}
	if !foundShouldBeRelative {
		t.Error("Expected 'should-be-relative' violation for '@retail/utils'")
	}
	if foundWrongAlias {
		t.Error("Did NOT expect 'wrong-alias' violation for inferred aliases")
	}
}

func TestConfigProcessor_ImportConventionsExplicit(t *testing.T) {
	// Change to the test fixture directory
	tempDir, _ := filepath.Abs(filepath.Join(".", "__fixtures__", "importConventionsExplicitProject"))
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

	ruleResult := result.RuleResults[0]

	// Check violations
	foundWrongAlias := false

	for _, violation := range ruleResult.ImportConventionViolations {
		t.Logf("Violation: %s in %s (Type: %s, Source: %s, Target: %s, Expected: %s)",
			violation.ImportRequest, violation.FilePath, violation.ViolationType,
			violation.SourceDomain, violation.TargetDomain, violation.ExpectedPattern)

		if violation.ViolationType == "wrong-alias" &&
			violation.ImportRequest == "@legacy-shared" {
			foundWrongAlias = true
			if violation.ExpectedPattern != "@shared/*" {
				t.Errorf("Expected pattern for @legacy-shared to be '@shared/*', got '%s'", violation.ExpectedPattern)
			}
		}
	}

	if !foundWrongAlias {
		t.Error("Expected 'wrong-alias' violation for '@legacy-shared' when alias is explicitly defined in config")
	}
}
