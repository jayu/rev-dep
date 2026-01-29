package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigProcessor_ImportConventions_Autofix(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-autofix-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	authDir := filepath.Join(tempDir, "src", "features", "auth")
	os.MkdirAll(authDir, 0755)

	// Create a file to be imported
	utilsFile := filepath.Join(authDir, "utils.ts")
	os.WriteFile(utilsFile, []byte("export const authenticate = () => true;"), 0644)

	// Create a file with an invalid import (intra-domain alias should be relative)
	invalidImportFile := filepath.Join(authDir, "invalidImport.ts")
	originalContent := "import { authenticate } from \"@auth/utils\";\n\nexport const login = () => authenticate();\n"
	os.WriteFile(invalidImportFile, []byte(originalContent), 0644)

	// Create package.json
	packageJson := `{
		"name": "test-project",
		"dependencies": {}
	}`
	os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJson), 0644)

	// Create tsconfig.json with alias
	tsconfig := `{
		"compilerOptions": {
			"baseUrl": ".",
			"paths": {
				"@auth/*": ["src/features/auth/*"]
			}
		}
	}`
	os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(tsconfig), 0644)

	// Create rev-dep config with autofix enabled
	config := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{
			{
				Path: ".",
				ImportConventions: []ImportConventionRule{
					{
						Rule:    "relative-internal-absolute-external",
						Autofix: true,
						Domains: []ImportConventionDomain{
							{
								Path:    "src/features/auth",
								Alias:   "@auth",
								Enabled: true,
							},
						},
					},
				},
			},
		},
	}

	// Process the config with fix=true
	result, err := ProcessConfig(&config, tempDir, "package.json", "tsconfig.json", true)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	// Check that we have a violation and it was fixed
	if len(result.RuleResults) == 0 {
		t.Fatalf("Expected at least 1 rule result")
	}

	ruleResult := result.RuleResults[0]
	foundViolation := false
	for _, v := range ruleResult.ImportConventionViolations {
		if v.ViolationType == "should-be-relative" {
			foundViolation = true
			if v.Fix == nil {
				t.Error("Expected violation to have a fix")
			}
		}
	}

	if !foundViolation {
		t.Error("Expected 'should-be-relative' violation for intra-domain nested alias import")
	}

	// Read the file and verify it was changed
	fixedContent, err := os.ReadFile(invalidImportFile)
	if err != nil {
		t.Fatalf("Failed to read fixed file: %v", err)
	}

	expectedContent := "import { authenticate } from \"./utils\";\n\nexport const login = () => authenticate();\n"
	if string(fixedContent) != expectedContent {
		t.Errorf("Autofix failed.\nExpected: %q\nGot:      %q", expectedContent, string(fixedContent))
	}
}

func TestConfigProcessor_ImportConventions_Autofix_Aliasing(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-autofix-alias-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	authDir := filepath.Join(tempDir, "src", "features", "auth")
	usersDir := filepath.Join(tempDir, "src", "features", "users")
	os.MkdirAll(authDir, 0755)
	os.MkdirAll(usersDir, 0755)

	// Create a file to be imported in auth domain
	utilsFile := filepath.Join(authDir, "utils.ts")
	os.WriteFile(utilsFile, []byte("export const authenticate = () => true;"), 0644)

	// Create a file in users domain with an invalid relative import to auth (should be aliased)
	usersFile := filepath.Join(usersDir, "controller.ts")
	originalContent := "import { authenticate } from \"../auth/utils\";\n\nexport const login = () => authenticate();\n"
	os.WriteFile(usersFile, []byte(originalContent), 0644)

	// Create package.json
	packageJson := `{
		"name": "test-project",
		"dependencies": {}
	}`
	os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJson), 0644)

	// Create tsconfig.json with alias
	tsconfig := `{
		"compilerOptions": {
			"baseUrl": ".",
			"paths": {
				"@auth/*": ["src/features/auth/*"]
			}
		}
	}`
	os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(tsconfig), 0644)

	// Create rev-dep config with autofix enabled
	config := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{
			{
				Path: ".",
				ImportConventions: []ImportConventionRule{
					{
						Rule:    "relative-internal-absolute-external",
						Autofix: true,
						Domains: []ImportConventionDomain{
							{
								Path:    "src/features/auth",
								Alias:   "@auth",
								Enabled: true,
							},
							{
								Path:    "src/features/users",
								Alias:   "@users",
								Enabled: true,
							},
						},
					},
				},
			},
		},
	}

	// Process the config with fix=true
	result, err := ProcessConfig(&config, tempDir, "package.json", "tsconfig.json", true)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	// Check that we have a violation and it was fixed
	ruleResult := result.RuleResults[0]
	foundViolation := false
	for _, v := range ruleResult.ImportConventionViolations {
		if v.ViolationType == "should-be-aliased" {
			foundViolation = true
		}
	}

	if !foundViolation {
		t.Error("Expected 'should-be-aliased' violation for inter-domain relative import")
	}

	// Read the file and verify it was changed to use alias
	fixedContent, err := os.ReadFile(usersFile)
	if err != nil {
		t.Fatalf("Failed to read fixed file: %v", err)
	}

	expectedContent := "import { authenticate } from \"@auth/utils\";\n\nexport const login = () => authenticate();\n"
	if string(fixedContent) != expectedContent {
		t.Errorf("Autofix failed.\nExpected: %q\nGot:      %q", expectedContent, string(fixedContent))
	}
}

func TestConfigProcessor_ImportConventions_StylePreservation(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-autofix-style-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	authDir := filepath.Join(tempDir, "src", "features", "auth")
	os.MkdirAll(authDir, 0755)

	// Files for testing
	os.WriteFile(filepath.Join(authDir, "utils.ts"), []byte("export const a = 1;"), 0644)
	os.WriteFile(filepath.Join(authDir, "index.ts"), []byte("export const b = 2;"), 0644)

	// test.ts has multiple imports to fix
	testFile := filepath.Join(authDir, "test.ts")
	originalContent := "import { a } from \"@auth/utils.ts\";\nimport { b } from \"@auth/index.ts\";\nimport { c } from \"@auth/index\";\n"
	os.WriteFile(testFile, []byte(originalContent), 0644)

	// Setup config
	config := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{
			{
				Path: ".",
				ImportConventions: []ImportConventionRule{
					{
						Rule:    "relative-internal-absolute-external",
						Autofix: true,
						Domains: []ImportConventionDomain{
							{Path: "src/features/auth", Alias: "@auth", Enabled: true},
						},
					},
				},
			},
		},
	}

	packageJson := `{"name": "test", "dependencies": {}}`
	os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJson), 0644)
	tsconfig := `{"compilerOptions": {"baseUrl": ".", "paths": {"@auth/*": ["src/features/auth/*"]}}}`
	os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(tsconfig), 0644)

	// Run autofix
	_, err = ProcessConfig(&config, tempDir, "package.json", "tsconfig.json", true)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	// Verify content
	fixedContent, _ := os.ReadFile(testFile)
	expectedContent := "import { a } from \"./utils.ts\";\nimport { b } from \"./index.ts\";\nimport { c } from \"./index\";\n"
	if string(fixedContent) != expectedContent {
		t.Errorf("Style preservation failed.\nExpected: %q\nGot:      %q", expectedContent, string(fixedContent))
	}
}

func TestConfigProcessor_ImportConventions_UnfixableAliasing(t *testing.T) {
	// Setup where auth imports from common, but common is not a domain
	tempDir, err := os.MkdirTemp("", "rev-dep-autofix-unfixable")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	authDir := filepath.Join(tempDir, "src", "auth")
	commonDir := filepath.Join(tempDir, "src", "common")
	os.MkdirAll(authDir, 0755)
	os.MkdirAll(commonDir, 0755)

	// Utils in common
	os.WriteFile(filepath.Join(commonDir, "utils.ts"), []byte("export const a = 1;"), 0644)

	// AuthService in auth imports from common via relative path
	serviceFile := filepath.Join(authDir, "service.ts")
	originalContent := "import { a } from \"../common/utils\";\n"
	os.WriteFile(serviceFile, []byte(originalContent), 0644)

	// Config ONLY defines auth as a domain
	config := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{
			{
				Path: ".",
				ImportConventions: []ImportConventionRule{
					{
						Rule:    "relative-internal-absolute-external",
						Autofix: true,
						Domains: []ImportConventionDomain{
							{Path: "src/auth", Alias: "@auth", Enabled: true},
							// src/common is NOT defined as a domain here
						},
					},
				},
			},
		},
	}

	packageJson := `{"name": "test", "dependencies": {}}`
	os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJson), 0644)

	// Run autofix
	result, err := ProcessConfig(&config, tempDir, "package.json", "", true)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	// Verify violation exists but has no Fix
	ruleResult := result.RuleResults[0]
	found := false
	for _, v := range ruleResult.ImportConventionViolations {
		if v.ViolationType == "should-be-aliased" {
			found = true
			if v.Fix != nil {
				t.Error("Expected unfixable violation to have nil Fix, but it has one")
			}
		}
	}
	if !found {
		t.Error("Expected should-be-aliased violation to be found")
	}

	// Verify file content is UNCHANGED
	fixedContent, _ := os.ReadFile(serviceFile)
	if string(fixedContent) != originalContent {
		t.Errorf("File should not have been changed.\nExpected: %q\nGot:      %q", originalContent, string(fixedContent))
	}
}
