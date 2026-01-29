package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInferAliasForDomain_CatchAll(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-repro-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	os.MkdirAll(filepath.Join(tempDir, "src", "utils"), 0755)

	// Create test tsconfig.json with catch-all alias
	tsconfigContent := `{
		"compilerOptions": {
			"baseUrl": ".",
			"paths": {
				"*": ["./*"]
			}
		}
	}`
	tsconfigPath := filepath.Join(tempDir, "tsconfig.json")
	err = os.WriteFile(tsconfigPath, []byte(tsconfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write tsconfig.json: %v", err)
	}

	// Parse tsconfig
	tsconfigBytes, err := ParseTsConfig(tsconfigPath)
	if err != nil {
		t.Fatalf("Failed to parse tsconfig: %v", err)
	}

	tsconfigParsed := ParseTsConfigContent(tsconfigBytes)

	// Test 1: Infer Alias
	domainPath := "src/utils"
	alias := InferAliasForDomain(domainPath, tsconfigParsed, nil)

	if alias != "*" {
		t.Errorf("Expected alias '*', got '%s'", alias)
	}

	t.Logf("Inferred alias for %s: '%s'", domainPath, alias)

	// Test 2: Validation of absolute import
	// Mock CompiledDomain
	targetDomain := &CompiledDomain{
		Path:          "src/utils",
		Alias:         "*",
		AbsolutePath:  filepath.Join(tempDir, "src", "utils"),
		AliasExplicit: false, // Should be false for inferred aliases like "*"
	}

	requestValid := "src/utils/foo"
	isValid := ValidateImportUsesCorrectAlias(requestValid, targetDomain, nil, nil)
	if !isValid {
		t.Errorf("ValidateImportUsesCorrectAlias should return true for '%s' with alias '*'", requestValid)
	}

	requestInvalid := "other/utils/foo"
	// "src/utils" path. Request "other/utils/foo". NO.
	isValid2 := ValidateImportUsesCorrectAlias(requestInvalid, targetDomain, nil, nil)
	if isValid2 {
		t.Errorf("ValidateImportUsesCorrectAlias should return false for '%s' with alias '*'", requestInvalid)
	}

	// Test 3: Fix Generation
	// Mimic logical flow in checkImportForViolation
	// Case: Relative import across domains -> should be aliased.
	// Import "../utils/foo" from "src/auth/bar"

	sourceFilePath := filepath.Join(tempDir, "src", "auth", "bar.ts")
	dep := MinimalDependency{
		Request:      "../utils/foo",
		RequestStart: 10,
		RequestEnd:   22,
		ResolvedType: UserModule,
	}

	resolvedPath := filepath.Join(tempDir, "src", "utils", "foo.ts")

	violation := checkImportForViolation(
		sourceFilePath,
		dep,
		&CompiledDomain{Path: "src/auth", AbsolutePath: filepath.Join(tempDir, "src", "auth")},   // source
		&CompiledDomain{Path: "src/utils", AbsolutePath: filepath.Join(tempDir, "src", "utils")}, // target
		resolvedPath,
		0,       // importIndex
		true,    // autofix
		tempDir, // cwd
		tsconfigParsed,
		true, // hasExplicitCatchAll
		nil,  // packageJsonImports
	)

	if violation == nil {
		t.Fatalf("Expected violation for relative import")
	}

	if violation.Fix == nil {
		t.Fatalf("Expected fix for relative import")
	}

	if violation.Fix.Text != "src/utils/foo" {
		t.Errorf("Expected fix 'src/utils/foo', got '%s'", violation.Fix.Text)
	}
}
