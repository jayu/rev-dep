package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckImportConventionsFromTree_Comprehensive(t *testing.T) {
	// Create a comprehensive test covering multiple scenarios across multiple files
	tempDir, err := os.MkdirTemp("", "rev-dep-comprehensive-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create directory structure
	authDir := filepath.Join(tempDir, "src", "auth")
	usersDir := filepath.Join(tempDir, "src", "users")
	utilsDir := filepath.Join(tempDir, "src", "utils")
	sharedDir := filepath.Join(tempDir, "src", "shared")
	os.MkdirAll(authDir, 0755)
	os.MkdirAll(usersDir, 0755)
	os.MkdirAll(utilsDir, 0755)
	os.MkdirAll(sharedDir, 0755)

	// Create test files
	authServiceFile := filepath.Join(authDir, "authService.ts")
	usersControllerFile := filepath.Join(usersDir, "userController.ts")
	sharedValidationFile := filepath.Join(sharedDir, "validation.ts")
	utilsCryptoFile := filepath.Join(utilsDir, "crypto.ts")
	userRepoFile := filepath.Join(usersDir, "userRepository.ts")
	loggerFile := filepath.Join(utilsDir, "logger.ts")
	configFile := filepath.Join(sharedDir, "config.ts")
	formatFile := filepath.Join(utilsDir, "format.ts")
	apiFile := filepath.Join(sharedDir, "api.ts")

	// Create some basic files
	os.WriteFile(utilsCryptoFile, []byte(`export function hashPassword(password: string): string {
	return password + "-hashed";
}`), 0644)

	os.WriteFile(userRepoFile, []byte(`export class UserRepository {
	find(id: string) { return { id }; }
}`), 0644)

	os.WriteFile(loggerFile, []byte(`export class Logger {
	log(message: string) { console.log(message); }
}`), 0644)

	os.WriteFile(configFile, []byte(`export class Config {
	get() { return {}; }
}`), 0644)

	os.WriteFile(formatFile, []byte(`export function formatError(error: string): string {
	return "Error: " + error;
}`), 0644)

	os.WriteFile(apiFile, []byte(`export class ApiResponse {
	success(data: any) { return { success: true, data }; }
}`), 0644)

	os.WriteFile(sharedValidationFile, []byte(`import { Logger } from '../utils/logger';

export function validateUser(email: string): boolean {
	const logger = new Logger();
	return email.includes("@");
}`), 0644)

	// Create tsconfig.json with various aliases
	tsconfigContent := `{
		"compilerOptions": {
			"baseUrl": ".",
			"paths": {
				"*": ["./*"],
				"@/shared/*": ["./src/shared/*"],
				"@/utils/*": ["./src/utils/*"]
			}
		}
	}`
	tsconfigPath := filepath.Join(tempDir, "tsconfig.json")
	os.WriteFile(tsconfigPath, []byte(tsconfigContent), 0644)

	// Create package.json with imports map
	packageJsonContent := `{
		"name": "test-app",
		"imports": {
			"#database/*": "./src/database/*"
		}
	}`
	packageJsonPath := filepath.Join(tempDir, "package.json")
	os.WriteFile(packageJsonPath, []byte(packageJsonContent), 0644)

	// Create database directory and file
	dbDir := filepath.Join(tempDir, "src", "database")
	os.MkdirAll(dbDir, 0755)
	dbConnectionFile := filepath.Join(dbDir, "connection.ts")
	os.WriteFile(dbConnectionFile, []byte(`export class Database {
	connect() { return "connected"; }
}`), 0644)

	t.Run("ExplicitConfiguration_WithViolations", func(t *testing.T) {
		rules := []ImportConventionRule{
			{
				Rule: "relative-internal-absolute-external",
				Domains: []ImportConventionDomain{
					{Path: "src/auth", Alias: "@/auth", Enabled: true},
					{Path: "src/users", Alias: "@/users", Enabled: true},
					{Path: "src/utils", Alias: "@/utils", Enabled: true},
					{Path: "src/shared", Alias: "@/shared", Enabled: true},
				},
				Autofix: true,
			},
		}

		// Create minimal tree with various import violations
		minimalTree := make(MinimalDependencyTree)
		minimalTree[authServiceFile] = []MinimalDependency{
			{
				Request:      "../utils/crypto", // Should be @/utils/crypto
				RequestStart: 24,
				RequestEnd:   42,
				ResolvedType: UserModule,
				ID:           func() *string { p := utilsCryptoFile; return &p }(),
			},
			{
				Request:      "../shared/validation", // Should be @/shared/validation
				RequestStart: 50,
				RequestEnd:   72,
				ResolvedType: UserModule,
				ID:           func() *string { p := sharedValidationFile; return &p }(),
			},
			{
				Request:      "src/utils/logger", // Wrong alias, should be @/utils/logger
				RequestStart: 80,
				RequestEnd:   98,
				ResolvedType: UserModule,
				ID:           func() *string { p := loggerFile; return &p }(),
			},
		}

		violations := CheckImportConventionsFromTree(
			minimalTree,
			[]string{authServiceFile},
			rules,
			nil,
			tempDir,
			false,
		)

		t.Logf("Found %d violations", len(violations))
		for i, violation := range violations {
			t.Logf("Violation %d: %s -> %s (%s)", i, violation.ImportRequest, violation.ExpectedPattern, violation.ViolationType)
		}

		// Should have violations for:
		// 1. should-be-aliased for relative imports
		// 2. wrong-alias for incorrect alias usage

		if len(violations) == 0 {
			t.Errorf("Expected violations in explicit configuration, got none")
		}

		violationTypes := make(map[string]int)
		for _, violation := range violations {
			violationTypes[violation.ViolationType]++
		}

		if violationTypes["should-be-aliased"] == 0 {
			t.Error("Expected should-be-aliased violations for relative imports")
		}

		if violationTypes["wrong-alias"] == 0 {
			t.Error("Expected wrong-alias violations for incorrect aliases")
		}
	})

	t.Run("ShortcutConfiguration_WithPackageJson", func(t *testing.T) {
		rules := []ImportConventionRule{
			{
				Rule: "relative-internal-absolute-external",
				Domains: []ImportConventionDomain{
					{Path: "src/*", Enabled: true}, // Shortcut syntax
				},
				Autofix: true,
			},
		}

		// Create minimal tree with mixed alias sources
		minimalTree := make(MinimalDependencyTree)
		minimalTree[usersControllerFile] = []MinimalDependency{
			{
				Request:      "../auth/authService", // Should be aliased
				RequestStart: 24,
				RequestEnd:   46,
				ResolvedType: UserModule,
				ID:           func() *string { p := authServiceFile; return &p }(),
			},
			{
				Request:      "#database/connection", // Package.json alias (valid)
				RequestStart: 50,
				RequestEnd:   72,
				ResolvedType: UserModule,
				ID:           func() *string { p := dbConnectionFile; return &p }(),
			},
			{
				Request:      "@/shared/api", // Tsconfig alias (valid)
				RequestStart: 80,
				RequestEnd:   96,
				ResolvedType: UserModule,
				ID:           func() *string { p := apiFile; return &p }(),
			},
		}

		violations := CheckImportConventionsFromTree(
			minimalTree,
			[]string{usersControllerFile},
			rules,
			nil,
			tempDir,
			false,
		)

		t.Logf("Found %d violations", len(violations))
		for i, violation := range violations {
			t.Logf("Violation %d: %s -> %s (%s)", i, violation.ImportRequest, violation.ExpectedPattern, violation.ViolationType)
		}

		if len(violations) == 0 {
			t.Errorf("Expected violations in shortcut configuration, got none")
		}

		// Should only have violation for relative import, not for valid aliases
		relativeImportViolation := false
		for _, violation := range violations {
			if violation.ViolationType == "should-be-aliased" &&
				strings.Contains(violation.ImportRequest, "../auth/authService") {
				relativeImportViolation = true
			}
		}

		if !relativeImportViolation {
			t.Error("Expected relative import to be flagged as should-be-aliased")
		}
	})

	t.Run("AutofixGeneration", func(t *testing.T) {
		rules := []ImportConventionRule{
			{
				Rule: "relative-internal-absolute-external",
				Domains: []ImportConventionDomain{
					{Path: "src/*", Enabled: true},
				},
				Autofix: true,
			},
		}

		// Create minimal tree with fixable violations
		minimalTree := make(MinimalDependencyTree)
		minimalTree[authServiceFile] = []MinimalDependency{
			{
				Request:      "../utils/crypto",
				RequestStart: 24,
				RequestEnd:   42,
				ResolvedType: UserModule,
				ID:           func() *string { p := utilsCryptoFile; return &p }(),
			},
		}

		violations := CheckImportConventionsFromTree(
			minimalTree,
			[]string{authServiceFile},
			rules,
			nil,
			tempDir,
			true, // enable autofix
		)

		// Check that fixes are generated
		fixCount := 0
		for _, violation := range violations {
			if violation.Fix != nil {
				fixCount++
				if violation.Fix.Text == "" {
					t.Errorf("Expected non-empty fix text for violation: %s", violation.ViolationType)
				}
				t.Logf("Generated fix: %s", violation.Fix.Text)
			}
		}

		if fixCount == 0 {
			t.Error("Expected some fixes to be generated when autofix is enabled")
		}
	})

	t.Run("DisabledDomains", func(t *testing.T) {
		rules := []ImportConventionRule{
			{
				Rule: "relative-internal-absolute-external",
				Domains: []ImportConventionDomain{
					{Path: "src/auth", Enabled: false}, // Disabled
					{Path: "src/users", Enabled: true}, // Enabled
				},
				Autofix: false,
			},
		}

		// Create minimal tree for both files
		minimalTree := make(MinimalDependencyTree)
		minimalTree[authServiceFile] = []MinimalDependency{
			{
				Request:      "../users/userRepository",
				RequestStart: 24,
				RequestEnd:   48,
				ResolvedType: UserModule,
				ID:           func() *string { p := userRepoFile; return &p }(),
			},
		}
		minimalTree[usersControllerFile] = []MinimalDependency{
			{
				Request:      "../auth/authService",
				RequestStart: 24,
				RequestEnd:   46,
				ResolvedType: UserModule,
				ID:           func() *string { p := authServiceFile; return &p }(),
			},
		}

		violations := CheckImportConventionsFromTree(
			minimalTree,
			[]string{authServiceFile, usersControllerFile},
			rules,
			nil,
			tempDir,
			false,
		)

		t.Logf("Found %d violations", len(violations))
		for i, violation := range violations {
			t.Logf("Violation %d: %s -> %s (%s) from %s", i, violation.ImportRequest, violation.ExpectedPattern, violation.ViolationType, violation.SourceDomain)
		}

		// Should have fewer violations because auth domain is disabled
		authViolations := 0
		usersViolations := 0

		for _, violation := range violations {
			if strings.Contains(violation.SourceDomain, "auth") {
				authViolations++
			}
			if strings.Contains(violation.SourceDomain, "users") {
				usersViolations++
			}
		}

		// Auth should have no violations (domain disabled)
		if authViolations > 0 {
			t.Errorf("Expected no violations from disabled auth domain, got %d", authViolations)
		}

		// Users should still have violations
		if usersViolations == 0 {
			t.Error("Expected violations from enabled users domain")
		}
	})

	t.Run("IntraDomainRelativeImports", func(t *testing.T) {
		rules := []ImportConventionRule{
			{
				Rule: "relative-internal-absolute-external",
				Domains: []ImportConventionDomain{
					{Path: "src/auth", Alias: "@/auth", Enabled: true},
				},
				Autofix: false,
			},
		}

		// Create another file in auth domain
		authUtilsFile := filepath.Join(authDir, "authUtils.ts")
		os.WriteFile(authUtilsFile, []byte(`export function validateAuth() {
	return true;
}`), 0644)

		// Create minimal tree with intra-domain relative import
		minimalTree := make(MinimalDependencyTree)
		minimalTree[authServiceFile] = []MinimalDependency{
			{
				Request:      "./authUtils",
				RequestStart: 24,
				RequestEnd:   36,
				ResolvedType: UserModule,
				ID:           func() *string { p := authUtilsFile; return &p }(),
			},
		}

		violations := CheckImportConventionsFromTree(
			minimalTree,
			[]string{authServiceFile},
			rules,
			nil,
			tempDir,
			false,
		)

		t.Logf("Found %d violations", len(violations))
		for i, violation := range violations {
			t.Logf("Violation %d: %s -> %s (%s)", i, violation.ImportRequest, violation.ExpectedPattern, violation.ViolationType)
		}

		// Should have no violations - intra-domain relative imports are allowed
		if len(violations) > 0 {
			t.Errorf("Expected no violations for intra-domain relative imports, got %d", len(violations))
		}
	})
}
