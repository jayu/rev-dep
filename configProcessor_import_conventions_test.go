package main

import (
	"os"
	"path/filepath"
	"strings"
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

	// Debug: Print file count and tree size
	t.Logf("File count: %d", ruleResult.FileCount)
	t.Logf("Dependency tree size: %d", len(ruleResult.DependencyTree))

	// Debug: Print some files in the dependency tree
	count := 0
	for file := range ruleResult.DependencyTree {
		if count < 5 {
			t.Logf("Found file in tree: %s", file)
			count++
		}
	}

	// Debug: Print rule path and check if files should be included
	t.Logf("Rule path: %s", ruleResult.RulePath)

	// Debug: Check what the current working directory is
	if cwd, err := os.Getwd(); err == nil {
		t.Logf("Current working directory: %s", cwd)
		// Debug: Show what absolute rule path would be
		absoluteRulePath := filepath.Join(cwd, ".")
		t.Logf("Absolute rule path would be: %s", absoluteRulePath)

		// Debug: Show what absolute file paths would be
		testFile := "src/features/auth/invalidImport.ts"
		absoluteFilePath := filepath.Join(cwd, testFile)
		t.Logf("Absolute file path for %s would be: %s", testFile, absoluteFilePath)
		t.Logf("Does %s start with %s? %v", absoluteFilePath, absoluteRulePath, strings.HasPrefix(absoluteFilePath, absoluteRulePath))
	}

	// Check if the files that should be included are actually there
	expectedFiles := []string{
		"src/features/auth/invalidImport.ts",
		"src/features/auth/validImport.ts",
		"src/features/users/invalidCrossDomain.ts",
		"src/features/users/validCrossDomain.ts",
	}

	for _, expectedFile := range expectedFiles {
		if _, exists := ruleResult.DependencyTree[expectedFile]; exists {
			t.Logf("Expected file found: %s", expectedFile)
		} else {
			t.Logf("Expected file NOT found: %s", expectedFile)
		}
	}

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

	// Debug: Print enabled checks
	t.Logf("Enabled checks: %v", ruleResult.EnabledChecks)

	// Debug: Print import details for a few files
	testFiles := []string{
		"src/features/auth/invalidImport.ts",
		"src/features/users/invalidCrossDomain.ts",
	}

	for _, testFile := range testFiles {
		if imports, exists := ruleResult.DependencyTree[testFile]; exists {
			t.Logf("File %s has %d imports:", testFile, len(imports))
			for i, imp := range imports {
				if i < 3 { // Limit output
					t.Logf("  Import %d: Request=%s, Type=%v", i, imp.Request, imp.ResolvedType)
					if imp.ID != nil {
						t.Logf("    Resolved to: %s", *imp.ID)
					}
				}
			}
		}
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
