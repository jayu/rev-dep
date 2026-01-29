package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportConventionsRegressionSuite(t *testing.T) {
	// This test suite covers all the regression scenarios we've fixed:
	// 1. Catch-all alias only when explicitly defined
	// 2. Wildcard alias precedence over specific aliases
	// 3. No invalid fixes when catch-all alias is removed
	// 4. Shortcut syntax with mixed alias configurations
	// 5. Explicit vs inferred alias validation

	t.Run("Scenario1_CatchAllOnlyWhenExplicit", func(t *testing.T) {
		// Test that fixes are only generated when "*" is explicitly in tsconfig
		tests := []struct {
			name              string
			tsconfigPaths     map[string]string
			shouldGenerateFix bool
		}{
			{
				name: "explicit_catchall_with_other_aliases",
				tsconfigPaths: map[string]string{
					"*":          "./*",
					"@/shared/*": "./app/shared/*",
				},
				shouldGenerateFix: true,
			},
			{
				name: "no_catchall_with_baseurl",
				tsconfigPaths: map[string]string{
					"@/shared/*": "./app/shared/*",
				},
				shouldGenerateFix: false,
			},
			{
				name: "no_catchall_no_baseurl",
				tsconfigPaths: map[string]string{
					"@/shared/*": "./app/shared/*",
				},
				shouldGenerateFix: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tempDir, err := os.MkdirTemp("", "rev-dep-regression-1")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				defer os.RemoveAll(tempDir)

				appsWebDir := filepath.Join(tempDir, "apps", "web")
				retailDir := filepath.Join(appsWebDir, "app", "retail")
				messagesTemplatesDir := filepath.Join(appsWebDir, "app", "messagesTemplates")

				os.MkdirAll(retailDir, 0755)
				os.MkdirAll(messagesTemplatesDir, 0755)

				sourceFile := filepath.Join(retailDir, "service.ts")
				sourceContent := `import { constants } from "../../../../../messagesTemplates/constants";`
				os.WriteFile(sourceFile, []byte(sourceContent), 0644)
				os.WriteFile(filepath.Join(messagesTemplatesDir, "constants.ts"), []byte("export const constants = {};"), 0644)

				// Create tsconfig
				tsconfigContent := `{
					"compilerOptions": {
						"baseUrl": ".",
						"paths": {
`
				for alias, path := range tt.tsconfigPaths {
					tsconfigContent += `							"` + alias + `": ["` + path + `"],` + "\n"
				}
				tsconfigContent += `						}
					}
				}`

				tsconfigPath := filepath.Join(appsWebDir, "tsconfig.json")
				os.WriteFile(tsconfigPath, []byte(tsconfigContent), 0644)

				tsconfigBytes, _ := ParseTsConfig(tsconfigPath)
				tsconfigParsed := ParseTsConfigContent(tsconfigBytes)

				domains := []ImportConventionDomain{
					{Path: "app/retail", Alias: "@/retail", Enabled: true},
				}

				compiledDomains, _ := CompileDomains(domains, appsWebDir, tsconfigParsed, nil)
				var retailDomain *CompiledDomain
				for _, domain := range compiledDomains {
					if domain.Path == "app/retail" {
						retailDomain = &domain
						break
					}
				}

				imports := []MinimalDependency{
					{
						Request:      "../../../../../messagesTemplates/constants",
						RequestStart: 24,
						RequestEnd:   65,
						ResolvedType: UserModule,
						ID:           func() *string { p := filepath.Join(appsWebDir, "app", "messagesTemplates", "constants.ts"); return &p }(),
					},
				}

				violations := checkFileImportConventions(
					sourceFile,
					imports,
					compiledDomains,
					retailDomain,
					true,
					appsWebDir,
					tsconfigParsed,
					nil,
				)

				hasFix := len(violations) > 0 && violations[0].Fix != nil
				if hasFix != tt.shouldGenerateFix {
					t.Errorf("Expected fix generation: %v, got: %v", tt.shouldGenerateFix, hasFix)
				}
			})
		}
	})

	t.Run("Scenario2_WildcardPrecedenceOverSpecific", func(t *testing.T) {
		// Test that specific aliases take precedence over wildcard aliases
		tempDir, err := os.MkdirTemp("", "rev-dep-regression-2")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		sharedDir := filepath.Join(tempDir, "src", "shared")
		utilsDir := filepath.Join(tempDir, "src", "utils")
		os.MkdirAll(sharedDir, 0755)
		os.MkdirAll(utilsDir, 0755)

		os.WriteFile(filepath.Join(sharedDir, "component.ts"), []byte("export const component = {};"), 0644)

		// tsconfig with both wildcard and specific aliases
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

		tsconfigBytes, _ := ParseTsConfig(tsconfigPath)
		tsconfigParsed := ParseTsConfigContent(tsconfigBytes)

		// Use shortcut syntax
		domains := []ImportConventionDomain{
			{Path: "src/*", Enabled: true},
		}

		compiledDomains, _ := CompileDomains(domains, tempDir, tsconfigParsed, nil)

		// Find the shared domain
		sharedPath := filepath.Join(tempDir, "src", "shared", "component.ts")
		targetDomain := ResolveDomainForFile(sharedPath, compiledDomains)

		if targetDomain == nil {
			t.Fatalf("Expected to find domain for shared file")
		}

		// Test cases for alias validation
		testCases := []struct {
			importPath    string
			shouldBeValid bool
			description   string
		}{
			{
				importPath:    "@/shared/component",
				shouldBeValid: true,
				description:   "Specific alias should be valid",
			},
			{
				importPath:    "src/shared/component",
				shouldBeValid: true,
				description:   "Direct path should be valid for catch-all",
			},
			{
				importPath:    "@shared/component",
				shouldBeValid: false,
				description:   "Wrong specific alias should be invalid",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				isValid := ValidateImportUsesCorrectAlias(tc.importPath, targetDomain, tsconfigParsed, nil)
				if isValid != tc.shouldBeValid {
					t.Errorf("Import '%s' validity: expected %v, got %v", tc.importPath, tc.shouldBeValid, isValid)
				}
			})
		}
	})

	t.Run("Scenario3_NoInvalidFixesWhenCatchAllRemoved", func(t *testing.T) {
		// Test that no invalid fixes are generated when catch-all is removed
		tempDir, err := os.MkdirTemp("", "rev-dep-regression-3")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		appsWebDir := filepath.Join(tempDir, "apps", "web")
		retailDir := filepath.Join(appsWebDir, "app", "retail")
		messagesTemplatesDir := filepath.Join(appsWebDir, "app", "messagesTemplates")

		os.MkdirAll(retailDir, 0755)
		os.MkdirAll(messagesTemplatesDir, 0755)

		sourceFile := filepath.Join(retailDir, "service.ts")
		sourceContent := `import { constants } from "../../../../../messagesTemplates/constants";`
		os.WriteFile(sourceFile, []byte(sourceContent), 0644)
		os.WriteFile(filepath.Join(messagesTemplatesDir, "constants.ts"), []byte("export const constants = {};"), 0644)

		// tsconfig WITHOUT catch-all but with baseUrl (auto-generates catch-all internally)
		tsconfigContent := `{
			"compilerOptions": {
				"baseUrl": ".",
				"paths": {
					"@/shared/*": ["./app/shared/*"]
				}
			}
		}`
		tsconfigPath := filepath.Join(appsWebDir, "tsconfig.json")
		os.WriteFile(tsconfigPath, []byte(tsconfigContent), 0644)

		tsconfigBytes, _ := ParseTsConfig(tsconfigPath)
		tsconfigParsed := ParseTsConfigContent(tsconfigBytes)

		domains := []ImportConventionDomain{
			{Path: "app/retail", Alias: "@/retail", Enabled: true},
		}

		compiledDomains, _ := CompileDomains(domains, appsWebDir, tsconfigParsed, nil)
		var retailDomain *CompiledDomain
		for _, domain := range compiledDomains {
			if domain.Path == "app/retail" {
				retailDomain = &domain
				break
			}
		}

		imports := []MinimalDependency{
			{
				Request:      "../../../../../messagesTemplates/constants",
				RequestStart: 24,
				RequestEnd:   65,
				ResolvedType: UserModule,
				ID:           func() *string { p := filepath.Join(appsWebDir, "app", "messagesTemplates", "constants.ts"); return &p }(),
			},
		}

		violations := checkFileImportConventions(
			sourceFile,
			imports,
			compiledDomains,
			retailDomain,
			true,
			appsWebDir,
			tsconfigParsed,
			nil,
		)

		// Should have violation but no fix (since catch-all is not explicit)
		if len(violations) != 1 {
			t.Fatalf("Expected 1 violation, got %d", len(violations))
		}

		if violations[0].Fix != nil {
			t.Errorf("Expected no fix when catch-all is not explicit, but got: '%s'", violations[0].Fix.Text)
		}
	})

	t.Run("Scenario4_ShortcutSyntaxMixedAliases", func(t *testing.T) {
		// Test shortcut syntax works correctly with mixed alias configurations
		tempDir, err := os.MkdirTemp("", "rev-dep-regression-4")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create complex directory structure
		dirs := []string{"src/auth", "src/shared/ui", "src/utils/helpers", "src/components"}
		for _, dir := range dirs {
			os.MkdirAll(filepath.Join(tempDir, dir), 0755)
		}

		// Create files
		os.WriteFile(filepath.Join(tempDir, "src/shared/ui", "Button.ts"), []byte("export const Button = {};"), 0644)
		os.WriteFile(filepath.Join(tempDir, "src/auth", "service.ts"), []byte("import { Button } from '../shared/ui/Button';"), 0644)

		// Complex tsconfig with multiple alias types
		tsconfigContent := `{
			"compilerOptions": {
				"baseUrl": ".",
				"paths": {
					"*": ["./*"],
					"@/shared/*": ["./src/shared/*"],
					"@/components/*": ["./src/components/*"],
					"@ui/*": ["./src/shared/ui/*"]
				}
			}
		}`
		tsconfigPath := filepath.Join(tempDir, "tsconfig.json")
		os.WriteFile(tsconfigPath, []byte(tsconfigContent), 0644)

		tsconfigBytes, _ := ParseTsConfig(tsconfigPath)
		tsconfigParsed := ParseTsConfigContent(tsconfigBytes)

		// Use shortcut syntax to create domains
		domains := []ImportConventionDomain{
			{Path: "src/*", Enabled: true},
		}

		compiledDomains, _ := CompileDomains(domains, tempDir, tsconfigParsed, nil)

		// Test that domains are created correctly
		expectedDomains := []string{"src/auth", "src/shared/ui", "src/utils/helpers", "src/components"}
		if len(compiledDomains) != len(expectedDomains) {
			t.Errorf("Expected %d domains, got %d", len(expectedDomains), len(compiledDomains))
		}

		// Test that shared/ui domain can be found
		sharedUIPath := filepath.Join(tempDir, "src/shared/ui", "Button.ts")
		targetDomain := ResolveDomainForFile(sharedUIPath, compiledDomains)

		if targetDomain == nil {
			t.Fatalf("Expected to find domain for shared/ui file")
		}

		// Test alias validation for mixed scenarios
		testImports := []struct {
			importPath    string
			shouldBeValid bool
		}{
			{"@/shared/ui/Button", true},   // Most specific
			{"@ui/Button", true},           // Also specific
			{"src/shared/ui/Button", true}, // Catch-all fallback
			{"@components/Button", false},  // Wrong specific alias
		}

		for _, test := range testImports {
			isValid := ValidateImportUsesCorrectAlias(test.importPath, targetDomain, tsconfigParsed, nil)
			if isValid != test.shouldBeValid {
				t.Errorf("Import '%s': expected valid=%v, got valid=%v", test.importPath, test.shouldBeValid, isValid)
			}
		}
	})

	t.Run("Scenario5_ExplicitVsInferredValidation", func(t *testing.T) {
		// Test that explicit aliases are validated differently from inferred aliases
		tempDir, err := os.MkdirTemp("", "rev-dep-regression-5")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		authDir := filepath.Join(tempDir, "src", "auth")
		sharedDir := filepath.Join(tempDir, "src", "shared")
		os.MkdirAll(authDir, 0755)
		os.MkdirAll(sharedDir, 0755)

		// tsconfig with catch-all and specific aliases
		tsconfigContent := `{
			"compilerOptions": {
				"baseUrl": ".",
				"paths": {
					"*": ["./*"],
					"@/shared/*": ["./src/shared/*"]
				}
			}
		}`
		tsconfigPath := filepath.Join(tempDir, "tsconfig.json")
		os.WriteFile(tsconfigPath, []byte(tsconfigContent), 0644)

		tsconfigBytes, _ := ParseTsConfig(tsconfigPath)
		tsconfigParsed := ParseTsConfigContent(tsconfigBytes)

		// Create domains: one explicit, one inferred
		domains := []ImportConventionDomain{
			{Path: "src/auth", Alias: "@/auth", Enabled: true}, // Explicit
			{Path: "src/shared", Enabled: true},                // Inferred (will get "*")
		}

		compiledDomains, _ := CompileDomains(domains, tempDir, tsconfigParsed, nil)

		var explicitDomain *CompiledDomain
		var inferredDomain *CompiledDomain

		for _, domain := range compiledDomains {
			if domain.Path == "src/auth" {
				explicitDomain = &domain
			} else if domain.Path == "src/shared" {
				inferredDomain = &domain
			}
		}

		// Test explicit domain validation
		if explicitDomain == nil {
			t.Fatal("Expected to find explicit domain")
		}

		explicitValid := ValidateImportUsesCorrectAlias("@/auth/utils", explicitDomain, tsconfigParsed, nil)
		explicitInvalid := ValidateImportUsesCorrectAlias("src/auth/utils", explicitDomain, tsconfigParsed, nil)

		if !explicitValid {
			t.Error("Explicit alias should be valid")
		}
		if explicitInvalid {
			t.Error("Direct path should be invalid for explicit domain")
		}

		// Test inferred domain validation
		if inferredDomain == nil {
			t.Fatal("Expected to find inferred domain")
		}

		inferredValidSpecific := ValidateImportUsesCorrectAlias("@/shared/component", inferredDomain, tsconfigParsed, nil)
		inferredValidCatchAll := ValidateImportUsesCorrectAlias("src/shared/component", inferredDomain, tsconfigParsed, nil)

		if !inferredValidSpecific {
			t.Error("Specific alias should be valid for inferred domain")
		}
		if !inferredValidCatchAll {
			t.Error("Catch-all path should be valid for inferred domain")
		}
	})

	t.Run("Scenario6_PackageJsonImportsValidation", func(t *testing.T) {
		// Test that package.json imports map aliases are properly validated
		tempDir, err := os.MkdirTemp("", "rev-dep-regression-6")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create test files
		utilsDir := filepath.Join(tempDir, "src", "utils")
		sharedDir := filepath.Join(tempDir, "src", "shared")
		os.MkdirAll(utilsDir, 0755)
		os.MkdirAll(sharedDir, 0755)

		// tsconfig with catch-all but no specific aliases for utils
		tsconfigContent := `{
			"compilerOptions": {
				"baseUrl": ".",
				"paths": {
					"*": ["./*"],
					"@/shared/*": ["./src/shared/*"]
				}
			}
		}`
		tsconfigPath := filepath.Join(tempDir, "tsconfig.json")
		os.WriteFile(tsconfigPath, []byte(tsconfigContent), 0644)

		// package.json with specific alias for utils
		packageJsonContent := `{
			"imports": {
				"#utils/*": "./src/utils/*"
			}
		}`
		packageJsonPath := filepath.Join(tempDir, "package.json")
		os.WriteFile(packageJsonPath, []byte(packageJsonContent), 0644)

		tsconfigBytes, _ := ParseTsConfig(tsconfigPath)
		packageJsonBytes, _ := os.ReadFile(packageJsonPath)

		resolver := NewImportsResolver(tempDir, tsconfigBytes, packageJsonBytes, []string{}, []string{}, nil)
		tsconfigParsed := resolver.tsConfigParsed
		packageJsonImports := resolver.packageJsonImports

		// Use shortcut syntax to create domains
		domains := []ImportConventionDomain{
			{Path: "src/*", Enabled: true},
		}

		compiledDomains, _ := CompileDomains(domains, tempDir, tsconfigParsed, packageJsonImports)

		// Find the utils domain
		utilsPath := filepath.Join(tempDir, "src", "utils", "helper.ts")
		targetDomain := ResolveDomainForFile(utilsPath, compiledDomains)

		if targetDomain == nil {
			t.Fatalf("Expected to find domain for utils file")
		}

		// Test validation with package.json alias
		testCases := []struct {
			importPath    string
			shouldBeValid bool
			description   string
		}{
			{
				importPath:    "#utils/helper",
				shouldBeValid: true,
				description:   "Package.json alias should be valid",
			},
			{
				importPath:    "src/utils/helper",
				shouldBeValid: true,
				description:   "Direct path should be valid for catch-all",
			},
			{
				importPath:    "@utils/helper",
				shouldBeValid: false,
				description:   "Wrong alias should be invalid",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				isValid := ValidateImportUsesCorrectAlias(tc.importPath, targetDomain, tsconfigParsed, packageJsonImports)

				if isValid != tc.shouldBeValid {
					t.Errorf("Import '%s' validity: expected %v, got %v", tc.importPath, tc.shouldBeValid, isValid)
				}
			})
		}
	})
}
