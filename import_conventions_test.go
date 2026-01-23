package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDomainForFile(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-domain-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	os.MkdirAll(filepath.Join(tempDir, "src", "auth"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "src", "users"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "src", "shared", "ui"), 0755)

	// Create compiled domains
	compiledDomains := []CompiledDomain{
		{Path: "src/auth", AbsolutePath: filepath.Join(tempDir, "src", "auth")},
		{Path: "src/users", AbsolutePath: filepath.Join(tempDir, "src", "users")},
		{Path: "src/shared/ui", AbsolutePath: filepath.Join(tempDir, "src", "shared", "ui")},
	}

	tests := []struct {
		name           string
		filePath       string
		expectedDomain *string
	}{
		{
			name:           "File in auth domain",
			filePath:       filepath.Join(tempDir, "src", "auth", "service.ts"),
			expectedDomain: stringPtr("src/auth"),
		},
		{
			name:           "File in users domain",
			filePath:       filepath.Join(tempDir, "src", "users", "controller.ts"),
			expectedDomain: stringPtr("src/users"),
		},
		{
			name:           "File in shared/ui domain",
			filePath:       filepath.Join(tempDir, "src", "shared", "ui", "Button.ts"),
			expectedDomain: stringPtr("src/shared/ui"),
		},
		{
			name:           "File not in any domain",
			filePath:       filepath.Join(tempDir, "node_modules", "lodash", "index.js"),
			expectedDomain: nil,
		},
		{
			name:           "File in parent directory",
			filePath:       filepath.Join(tempDir, "index.ts"),
			expectedDomain: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain := ResolveDomainForFile(tt.filePath, compiledDomains)

			if tt.expectedDomain == nil {
				if domain != nil {
					t.Errorf("Expected no domain, got %s", domain.Path)
				}
			} else {
				if domain == nil {
					t.Errorf("Expected domain %s, got nil", *tt.expectedDomain)
				} else if domain.Path != *tt.expectedDomain {
					t.Errorf("Expected domain %s, got %s", *tt.expectedDomain, domain.Path)
				}
			}
		})
	}
}

func TestExpandDomainGlobs(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-glob-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	os.MkdirAll(filepath.Join(tempDir, "src", "auth"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "src", "users"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "src", "shared", "ui"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "src", "shared", "utils"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "lib"), 0755)

	tests := []struct {
		name          string
		patterns      []string
		expectedPaths []string
	}{
		{
			name:     "Simple glob pattern",
			patterns: []string{"src/*"},
			expectedPaths: []string{
				filepath.Join(tempDir, "src", "auth"),
				filepath.Join(tempDir, "src", "users"),
				filepath.Join(tempDir, "src", "shared"),
			},
		},
		{
			name:     "Multiple glob patterns",
			patterns: []string{"src/*", "lib"},
			expectedPaths: []string{
				filepath.Join(tempDir, "src", "auth"),
				filepath.Join(tempDir, "src", "users"),
				filepath.Join(tempDir, "src", "shared"),
				filepath.Join(tempDir, "lib"),
			},
		},
		{
			name:     "No glob patterns",
			patterns: []string{"src/auth"},
			expectedPaths: []string{
				filepath.Join(tempDir, "src", "auth"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Change to temp directory for glob expansion
			originalCwd, _ := os.Getwd()
			defer os.Chdir(originalCwd)
			os.Chdir(tempDir)

			result, err := ExpandDomainGlobs(tt.patterns, tempDir)
			if err != nil {
				t.Fatalf("ExpandDomainGlobs failed: %v", err)
			}

			// Convert to sets for comparison
			resultSet := make(map[string]bool)
			for _, path := range result {
				resultSet[path] = true
			}

			expectedSet := make(map[string]bool)
			for _, path := range tt.expectedPaths {
				expectedSet[path] = true
			}

			// Check that all expected paths are in result
			for expectedPath := range expectedSet {
				if !resultSet[expectedPath] {
					t.Errorf("Expected path %s not found in result", expectedPath)
				}
			}

			// Check that result doesn't have extra paths
			for resultPath := range resultSet {
				if !expectedSet[resultPath] {
					t.Errorf("Unexpected path %s found in result", resultPath)
				}
			}
		})
	}
}

func TestCompileDomains(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-compile-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	os.MkdirAll(filepath.Join(tempDir, "src", "auth"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "src", "users"), 0755)

	tests := []struct {
		name             string
		domains          []ImportConventionDomain
		expectedCompiled []CompiledDomain
	}{
		{
			name: "Simple domains without aliases",
			domains: []ImportConventionDomain{
				{Path: "src/auth"},
				{Path: "src/users"},
			},
			expectedCompiled: []CompiledDomain{
				{Path: "src/auth", AbsolutePath: filepath.Join(tempDir, "src", "auth")},
				{Path: "src/users", AbsolutePath: filepath.Join(tempDir, "src", "users")},
			},
		},
		{
			name: "Domains with aliases",
			domains: []ImportConventionDomain{
				{Path: "src/auth", Alias: "@auth"},
				{Path: "src/users", Alias: "@users"},
			},
			expectedCompiled: []CompiledDomain{
				{Path: "src/auth", Alias: "@auth", AbsolutePath: filepath.Join(tempDir, "src", "auth")},
				{Path: "src/users", Alias: "@users", AbsolutePath: filepath.Join(tempDir, "src", "users")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CompileDomains(tt.domains, tempDir)
			if err != nil {
				t.Fatalf("CompileDomains failed: %v", err)
			}

			if len(result) != len(tt.expectedCompiled) {
				t.Fatalf("Expected %d compiled domains, got %d", len(tt.expectedCompiled), len(result))
			}

			for i, expected := range tt.expectedCompiled {
				if result[i].Path != expected.Path {
					t.Errorf("Expected path %s, got %s", expected.Path, result[i].Path)
				}
				if result[i].Alias != expected.Alias {
					t.Errorf("Expected alias %s, got %s", expected.Alias, result[i].Alias)
				}
				if result[i].AbsolutePath != expected.AbsolutePath {
					t.Errorf("Expected absolute path %s, got %s", expected.AbsolutePath, result[i].AbsolutePath)
				}
			}
		})
	}
}

func TestInferAliasForDomain(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-alias-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	os.MkdirAll(filepath.Join(tempDir, "src", "auth"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "src", "users"), 0755)

	// Create test tsconfig.json
	tsconfigContent := `{
		"compilerOptions": {
			"paths": {
				"@auth/*": ["src/auth/*"],
				"@users/*": ["src/users/*"],
				"@shared/*": ["src/shared/*"]
			}
		}
	}`
	tsconfigPath := filepath.Join(tempDir, "tsconfig.json")
	err = os.WriteFile(tsconfigPath, []byte(tsconfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write tsconfig.json: %v", err)
	}

	// Create test package.json
	packageJsonContent := `{
		"imports": {
			"#utils/*": "./src/utils/*",
			"#components/*": "./src/components/*"
		}
	}`
	packageJsonPath := filepath.Join(tempDir, "package.json")
	err = os.WriteFile(packageJsonPath, []byte(packageJsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	// Parse tsconfig and package.json
	tsconfigBytes, err := ParseTsConfig(tsconfigPath)
	if err != nil {
		t.Fatalf("Failed to parse tsconfig: %v", err)
	}

	tsconfigParsed := ParseTsConfigContent(tsconfigBytes)

	packageJsonImports, err := ParsePackageJsonImports(packageJsonPath)
	if err != nil {
		t.Fatalf("Failed to parse package.json imports: %v", err)
	}

	tests := []struct {
		name          string
		domainPath    string
		expectedAlias string
	}{
		{
			name:          "Domain matching tsconfig path",
			domainPath:    "src/auth",
			expectedAlias: "@auth",
		},
		{
			name:          "Domain matching tsconfig path with different pattern",
			domainPath:    "src/users",
			expectedAlias: "@users",
		},
		{
			name:          "Domain matching package.json import",
			domainPath:    "src/utils",
			expectedAlias: "#utils",
		},
		{
			name:          "Domain with no matching alias",
			domainPath:    "src/unknown",
			expectedAlias: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alias := InferAliasForDomain(tt.domainPath, tsconfigParsed, packageJsonImports)
			if alias != tt.expectedAlias {
				t.Errorf("Expected alias %s, got %s", tt.expectedAlias, alias)
			}
		})
	}
}
