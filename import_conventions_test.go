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
				{Path: "src/auth", Alias: "@auth", AbsolutePath: filepath.Join(tempDir, "src", "auth"), AliasExplicit: true},
				{Path: "src/users", Alias: "@users", AbsolutePath: filepath.Join(tempDir, "src", "users"), AliasExplicit: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CompileDomains(tt.domains, tempDir, nil, nil)
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

func TestIsRelativeImport(t *testing.T) {
	tests := []struct {
		name     string
		request  string
		expected bool
	}{
		{
			name:     "Relative import with dot slash",
			request:  "./utils",
			expected: true,
		},
		{
			name:     "Relative import with parent directory",
			request:  "../auth/service",
			expected: true,
		},
		{
			name:     "Relative import with current directory",
			request:  ".",
			expected: true,
		},
		{
			name:     "Relative import with parent directory only",
			request:  "..",
			expected: true,
		},
		{
			name:     "Absolute import with alias",
			request:  "@auth/utils",
			expected: false,
		},
		{
			name:     "Absolute import with package.json import",
			request:  "#utils/helper",
			expected: false,
		},
		{
			name:     "Node module import",
			request:  "lodash",
			expected: false,
		},
		{
			name:     "Node module import with path",
			request:  "lodash/fp",
			expected: false,
		},
		{
			name:     "Absolute path",
			request:  "/src/utils",
			expected: false,
		},
		{
			name:     "Windows-style absolute path",
			request:  "C:\\src\\utils",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRelativeImport(tt.request)
			if result != tt.expected {
				t.Errorf("IsRelativeImport(%q) = %v, want %v", tt.request, result, tt.expected)
			}
		})
	}
}

func TestImportTargetsDomain(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-target-test")
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
		importPath     string
		expectedDomain *string
	}{
		{
			name:           "Import path in auth domain",
			importPath:     filepath.Join(tempDir, "src", "auth", "service.ts"),
			expectedDomain: stringPtr("src/auth"),
		},
		{
			name:           "Import path in users domain",
			importPath:     filepath.Join(tempDir, "src", "users", "controller.ts"),
			expectedDomain: stringPtr("src/users"),
		},
		{
			name:           "Import path in shared/ui domain",
			importPath:     filepath.Join(tempDir, "src", "shared", "ui", "Button.ts"),
			expectedDomain: stringPtr("src/shared/ui"),
		},
		{
			name:           "Import path not in any domain",
			importPath:     filepath.Join(tempDir, "node_modules", "lodash", "index.js"),
			expectedDomain: nil,
		},
		{
			name:           "Import path in parent directory",
			importPath:     filepath.Join(tempDir, "index.ts"),
			expectedDomain: nil,
		},
		{
			name:           "Nested file in domain",
			importPath:     filepath.Join(tempDir, "src", "auth", "utils", "helper.ts"),
			expectedDomain: stringPtr("src/auth"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain := ResolveImportTargetDomain(tt.importPath, compiledDomains)

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

func TestValidateImportUsesCorrectAlias(t *testing.T) {
	tests := []struct {
		name         string
		request      string
		targetDomain *CompiledDomain
		expected     bool
	}{
		{
			name:         "Correct alias for domain",
			request:      "@auth/utils",
			targetDomain: &CompiledDomain{Path: "src/auth", Alias: "@auth", AliasExplicit: true},
			expected:     true,
		},
		{
			name:         "Wrong alias for domain",
			request:      "@users/utils",
			targetDomain: &CompiledDomain{Path: "src/auth", Alias: "@auth", AliasExplicit: true},
			expected:     false,
		},
		{
			name:         "Package.json import alias",
			request:      "#utils/helper",
			targetDomain: &CompiledDomain{Path: "src/utils", Alias: "#utils", AliasExplicit: true},
			expected:     true,
		},
		{
			name:         "Relative import should not match alias",
			request:      "./utils",
			targetDomain: &CompiledDomain{Path: "src/auth", Alias: "@auth", AliasExplicit: true},
			expected:     false,
		},
		{
			name:         "Node module import should not match alias",
			request:      "lodash",
			targetDomain: &CompiledDomain{Path: "src/auth", Alias: "@auth", AliasExplicit: true},
			expected:     false,
		},
		{
			name:         "Domain with no alias",
			request:      "something",
			targetDomain: &CompiledDomain{Path: "src/auth", Alias: "", AliasExplicit: false},
			expected:     false,
		},
		{
			name:         "Alias with path suffix",
			request:      "@auth/utils/helper",
			targetDomain: &CompiledDomain{Path: "src/auth", Alias: "@auth", AliasExplicit: true},
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateImportUsesCorrectAlias(tt.request, tt.targetDomain)
			if result != tt.expected {
				t.Errorf("ValidateImportUsesCorrectAlias(%q, domain with alias %q) = %v, want %v",
					tt.request, tt.targetDomain.Alias, result, tt.expected)
			}
		})
	}
}

func TestCheckImportConventions_IntraDomainAlias(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-violation-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	os.MkdirAll(filepath.Join(tempDir, "src", "auth"), 0755)

	// Create compiled domains
	compiledDomains := []CompiledDomain{
		{Path: "src/auth", AbsolutePath: filepath.Join(tempDir, "src", "auth"), Alias: "@auth", Enabled: true, AliasExplicit: true},
	}

	// Create test imports - intra-domain import using alias (should be relative)
	imports := []MinimalDependency{
		{
			ID:           stringPtr(filepath.Join(tempDir, "src", "auth", "utils.ts")),
			Request:      "@auth/utils",
			ResolvedType: UserModule,
		},
	}

	violations := checkFileImportConventions(
		filepath.Join(tempDir, "src", "auth", "service.ts"),
		imports,
		compiledDomains,
		&compiledDomains[0],
		tempDir,
	)

	if len(violations) != 1 {
		t.Fatalf("Expected 1 violation, got %d", len(violations))
	}

	violation := violations[0]
	if violation.ViolationType != "should-be-relative" {
		t.Errorf("Expected violation type 'should-be-relative', got %s", violation.ViolationType)
	}
	if violation.SourceDomain != "src/auth" {
		t.Errorf("Expected source domain 'src/auth', got %s", violation.SourceDomain)
	}
	if violation.TargetDomain != "src/auth" {
		t.Errorf("Expected target domain 'src/auth', got %s", violation.TargetDomain)
	}
}

func TestCheckImportConventions_InterDomainRelative(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-violation-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	os.MkdirAll(filepath.Join(tempDir, "src", "auth"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "src", "users"), 0755)

	// Create compiled domains
	compiledDomains := []CompiledDomain{
		{Path: "src/auth", AbsolutePath: filepath.Join(tempDir, "src", "auth"), Alias: "@auth", Enabled: true, AliasExplicit: true},
		{Path: "src/users", AbsolutePath: filepath.Join(tempDir, "src", "users"), Alias: "@users", Enabled: true, AliasExplicit: true},
	}

	// Create test imports - inter-domain import using relative path (should be aliased)
	imports := []MinimalDependency{
		{
			ID:           stringPtr(filepath.Join(tempDir, "src", "auth", "service.ts")),
			Request:      "../auth/service",
			ResolvedType: UserModule,
		},
	}

	violations := checkFileImportConventions(
		filepath.Join(tempDir, "src", "users", "controller.ts"),
		imports,
		compiledDomains,
		&compiledDomains[1], // users domain
		tempDir,
	)

	if len(violations) != 1 {
		t.Fatalf("Expected 1 violation, got %d", len(violations))
	}

	violation := violations[0]
	if violation.ViolationType != "should-be-aliased" {
		t.Errorf("Expected violation type 'should-be-aliased', got %s", violation.ViolationType)
	}
	if violation.SourceDomain != "src/users" {
		t.Errorf("Expected source domain 'src/users', got %s", violation.SourceDomain)
	}
	if violation.TargetDomain != "src/auth" {
		t.Errorf("Expected target domain 'src/auth', got %s", violation.TargetDomain)
	}
}

func TestCheckImportConventions_WrongAlias(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-violation-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	os.MkdirAll(filepath.Join(tempDir, "src", "auth"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "src", "users"), 0755)

	// Create compiled domains
	compiledDomains := []CompiledDomain{
		{Path: "src/auth", AbsolutePath: filepath.Join(tempDir, "src", "auth"), Alias: "@auth", Enabled: true, AliasExplicit: true},
		{Path: "src/users", AbsolutePath: filepath.Join(tempDir, "src", "users"), Alias: "@users", Enabled: true, AliasExplicit: true},
	}

	// Create test imports - inter-domain import using wrong alias
	imports := []MinimalDependency{
		{
			ID:           stringPtr(filepath.Join(tempDir, "src", "auth", "service.ts")),
			Request:      "@users/service", // Wrong alias - should be @auth
			ResolvedType: UserModule,
		},
	}

	violations := checkFileImportConventions(
		filepath.Join(tempDir, "src", "users", "controller.ts"),
		imports,
		compiledDomains,
		&compiledDomains[1], // users domain
		tempDir,
	)

	if len(violations) != 1 {
		t.Fatalf("Expected 1 violation, got %d", len(violations))
	}

	violation := violations[0]
	if violation.ViolationType != "wrong-alias" {
		t.Errorf("Expected violation type 'wrong-alias', got %s", violation.ViolationType)
	}
	if violation.SourceDomain != "src/users" {
		t.Errorf("Expected source domain 'src/users', got %s", violation.SourceDomain)
	}
	if violation.TargetDomain != "src/auth" {
		t.Errorf("Expected target domain 'src/auth', got %s", violation.TargetDomain)
	}
}

func TestCheckImportConventions_ValidIntraDomain(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-violation-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	os.MkdirAll(filepath.Join(tempDir, "src", "auth"), 0755)

	// Create compiled domains
	compiledDomains := []CompiledDomain{
		{Path: "src/auth", AbsolutePath: filepath.Join(tempDir, "src", "auth"), Alias: "@auth", Enabled: true},
	}

	// Create test imports - valid intra-domain relative import
	imports := []MinimalDependency{
		{
			ID:           stringPtr(filepath.Join(tempDir, "src", "auth", "utils.ts")),
			Request:      "./utils",
			ResolvedType: UserModule,
		},
	}

	violations := checkFileImportConventions(
		filepath.Join(tempDir, "src", "auth", "service.ts"),
		imports,
		compiledDomains,
		&compiledDomains[0],
		tempDir,
	)

	if len(violations) != 0 {
		t.Errorf("Expected no violations, got %d", len(violations))
	}
}

func TestCheckImportConventions_ValidInterDomain(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-violation-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	os.MkdirAll(filepath.Join(tempDir, "src", "auth"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "src", "users"), 0755)

	// Create compiled domains
	compiledDomains := []CompiledDomain{
		{Path: "src/auth", AbsolutePath: filepath.Join(tempDir, "src", "auth"), Alias: "@auth", Enabled: true, AliasExplicit: true},
		{Path: "src/users", AbsolutePath: filepath.Join(tempDir, "src", "users"), Alias: "@users", Enabled: true, AliasExplicit: true},
	}

	// Create test imports - valid inter-domain aliased import
	imports := []MinimalDependency{
		{
			ID:           stringPtr(filepath.Join(tempDir, "src", "auth", "service.ts")),
			Request:      "@auth/service",
			ResolvedType: UserModule,
		},
	}

	violations := checkFileImportConventions(
		filepath.Join(tempDir, "src", "users", "controller.ts"),
		imports,
		compiledDomains,
		&compiledDomains[1], // users domain
		tempDir,
	)

	if len(violations) != 0 {
		t.Errorf("Expected no violations, got %d", len(violations))
	}
}

func TestCheckImportConventions_EnabledField(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-enabled-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	os.MkdirAll(filepath.Join(tempDir, "src", "auth"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "src", "users"), 0755)

	// Create compiled domains with different enabled states
	compiledDomains := []CompiledDomain{
		{Path: "src/auth", AbsolutePath: filepath.Join(tempDir, "src", "auth"), Alias: "@auth", Enabled: false, AliasExplicit: true},   // Disabled
		{Path: "src/users", AbsolutePath: filepath.Join(tempDir, "src", "users"), Alias: "@users", Enabled: true, AliasExplicit: true}, // Enabled
	}

	// Test 1: Import FROM enabled domain TO disabled domain should still be checked for alias
	// This ensures aliases work even if domain checks are disabled for the domain itself
	imports := []MinimalDependency{
		{
			ID:           stringPtr(filepath.Join(tempDir, "src", "auth", "service.ts")),
			Request:      "../auth/service", // This would normally be a violation (should be aliased)
			ResolvedType: UserModule,
		},
	}

	violations := checkFileImportConventions(
		filepath.Join(tempDir, "src", "users", "controller.ts"),
		imports,
		compiledDomains,
		&compiledDomains[1], // users domain (enabled)
		tempDir,
	)

	// Should have a violation because it targets a domain that has an alias, even if targets own checks are disabled
	if len(violations) != 1 {
		t.Errorf("Expected 1 violation when target domain is disabled but has alias, got %d", len(violations))
	} else if violations[0].ViolationType != "should-be-aliased" {
		t.Errorf("Expected violation type 'should-be-aliased', got '%s'", violations[0].ViolationType)
	}

	// Test 2: Import to enabled domain should still be checked
	imports = []MinimalDependency{
		{
			ID:           stringPtr(filepath.Join(tempDir, "src", "users", "service.ts")),
			Request:      "@users/service", // This is a violation (should be relative within same domain)
			ResolvedType: UserModule,
		},
	}

	violations = checkFileImportConventions(
		filepath.Join(tempDir, "src", "users", "controller.ts"),
		imports,
		compiledDomains,
		&compiledDomains[1], // users domain (enabled)
		tempDir,
	)

	// Should have a violation because it's intra-domain with alias instead of relative
	if len(violations) != 1 {
		t.Errorf("Expected 1 violation for intra-domain alias import, got %d", len(violations))
	} else {
		if violations[0].ViolationType != "should-be-relative" {
			t.Errorf("Expected violation type 'should-be-relative', got '%s'", violations[0].ViolationType)
		}
	}

	// Test 3: Files in disabled domains should not be checked
	imports = []MinimalDependency{
		{
			ID:           stringPtr(filepath.Join(tempDir, "src", "users", "service.ts")),
			Request:      "@users/service", // This would normally be a violation (should be relative)
			ResolvedType: UserModule,
		},
	}

	violations = checkFileImportConventions(
		filepath.Join(tempDir, "src", "auth", "controller.ts"),
		imports,
		compiledDomains,
		&compiledDomains[0], // auth domain (disabled)
		tempDir,
	)

	// Should have no violations because source domain is disabled
	if len(violations) != 0 {
		t.Errorf("Expected no violations when source domain is disabled, got %d", len(violations))
	}

	// Test 4: Import FROM enabled domain TO disabled domain using WRONG alias should be caught
	imports = []MinimalDependency{
		{
			ID:           stringPtr(filepath.Join(tempDir, "src", "auth", "service.ts")),
			Request:      "@wrong-alias/service", // Wrong alias for auth domain
			ResolvedType: UserModule,
		},
	}

	violations = checkFileImportConventions(
		filepath.Join(tempDir, "src", "users", "controller.ts"),
		imports,
		compiledDomains,
		&compiledDomains[1], // users domain (enabled)
		tempDir,
	)

	// Should have a violation because it uses the wrong explicit alias for the target domain
	if len(violations) != 1 {
		t.Errorf("Expected 1 violation for wrong-alias targeting disabled domain, got %d", len(violations))
	} else if violations[0].ViolationType != "wrong-alias" {
		t.Errorf("Expected violation type 'wrong-alias', got '%s'", violations[0].ViolationType)
	}
}
