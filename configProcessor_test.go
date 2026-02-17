package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// containsString checks if a slice of strings contains a specific string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// containsStringSlice checks if a slice of string slices contains a specific string
func containsStringSlice(slices [][]string, item string) bool {
	for _, slice := range slices {
		for _, s := range slice {
			if s == item {
				return true
			}
		}
	}
	return false
}

func TestConfigProcessor_CircularDependencies(t *testing.T) {
	// Ensure clean state
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/configProcessorProject")

	config, err := LoadConfig(testCwd)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	result, err := ProcessConfig(&config[0], testCwd, "package.json", "tsconfig.json", false)
	if err != nil {
		t.Fatalf("Failed to process config: %v", err)
	}

	// Should detect circular dependencies between featureA and featureB
	foundExpectedCircular := false
	for _, ruleResult := range result.RuleResults {
		for _, circularDep := range ruleResult.CircularDependencies {
			// Check if this matches our expected circular dependency (order may vary)
			// Look for the key files regardless of full path
			hasFeatureA := false
			hasFeatureB := false
			for _, file := range circularDep {
				relPath, _ := filepath.Rel(testCwd, file)
				if filepath.Base(relPath) == "featureA.ts" {
					hasFeatureA = true
				}
				if filepath.Base(relPath) == "featureB.ts" {
					hasFeatureB = true
				}
			}
			if hasFeatureA && hasFeatureB && len(circularDep) >= 3 {
				foundExpectedCircular = true
				t.Logf("Found expected circular dependency: %v", circularDep)
				break
			}
		}
		if foundExpectedCircular {
			break
		}
	}

	if !foundExpectedCircular {
		t.Errorf("Expected to find circular dependencies between featureA and featureB, but got: %v", result.RuleResults[0].CircularDependencies)
		for _, ruleResult := range result.RuleResults {
			t.Logf("Rule result circular deps: %v", ruleResult.CircularDependencies)
		}
	}
}

func TestConfigProcessor_OrphanFiles(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/configProcessorProject")

	config, err := LoadConfig(testCwd)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	result, err := ProcessConfig(&config[0], testCwd, "package.json", "tsconfig.json", false)
	if err != nil {
		t.Fatalf("Failed to process config: %v", err)
	}

	// Should detect specific orphan files
	expectedOrphanFiles := []string{
		"src/utils/orphan.ts",
		"src/boundary/public.ts",
		"src/index.ts",
		"packages/subpkg/src/index.ts",
		"packages/subpkg/src/orphan.ts",
		"packages/subpkg/src/broken-import.ts",
	}

	var foundOrphanFiles []string
	for _, ruleResult := range result.RuleResults {
		foundOrphanFiles = append(foundOrphanFiles, ruleResult.OrphanFiles...)
	}

	// Convert found files to relative paths for comparison
	foundOrphanRelative := []string{}
	for _, file := range foundOrphanFiles {
		relPath, _ := filepath.Rel(testCwd, file)
		foundOrphanRelative = append(foundOrphanRelative, relPath)
	}

	// Check that all expected orphan files are found
	for _, expected := range expectedOrphanFiles {
		if !containsString(foundOrphanRelative, expected) {
			t.Errorf("Expected orphan file %s not found in results: %v", expected, foundOrphanRelative)
		}
	}

}

func TestConfigProcessor_ModuleBoundaries(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/configProcessorProject")

	config, err := LoadConfig(testCwd)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	result, err := ProcessConfig(&config[0], testCwd, "package.json", "tsconfig.json", false)
	if err != nil {
		t.Fatalf("Failed to process config: %v", err)
	}

	// Should find specific module boundary violations
	foundExpectedViolation := false
	for _, ruleResult := range result.RuleResults {
		for _, violation := range ruleResult.ModuleBoundaryViolations {
			// Convert to relative paths for comparison
			relFilePath, _ := filepath.Rel(testCwd, violation.FilePath)
			relImportPath, _ := filepath.Rel(testCwd, violation.ImportPath)

			// Look for violation regardless of full path
			if filepath.Base(relFilePath) == "public.ts" &&
				filepath.Base(relImportPath) == "private.ts" &&
				violation.RuleName == "public-boundary" &&
				violation.ViolationType == "denied" {
				foundExpectedViolation = true
				t.Logf("Found expected violation: %s -> %s", relFilePath, relImportPath)
				break
			}
		}
		if foundExpectedViolation {
			break
		}
	}

	if !foundExpectedViolation {
		t.Errorf("Expected to find boundary violation with public.ts importing private.ts, but got: %v", result.RuleResults[0].ModuleBoundaryViolations)
	}
}

func TestConfigProcessor_UnusedNodeModules(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/configProcessorProject")

	config, err := LoadConfig(testCwd)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	result, err := ProcessConfig(&config[0], testCwd, "package.json", "tsconfig.json", false)
	if err != nil {
		t.Fatalf("Failed to process config: %v", err)
	}

	// Should find specific unused node modules
	expectedUnusedModules := []string{
		"@types/lodash",
		"lodash",
		"moment",
		"typescript",
	}

	var foundUnusedModules []string
	for _, ruleResult := range result.RuleResults {
		foundUnusedModules = append(foundUnusedModules, ruleResult.UnusedNodeModules...)
	}

	// Check that all expected unused modules are found
	for _, expected := range expectedUnusedModules {
		if !containsString(foundUnusedModules, expected) {
			t.Errorf("Expected unused module %s not found in results: %v", expected, foundUnusedModules)
		}
	}

	// Specifically check that moment is unused (it's in package.json but not imported)
	if !containsString(foundUnusedModules, "moment") {
		t.Error("Expected to find moment as an unused node module")
	}
}

func TestConfigProcessor_MissingNodeModules(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/configProcessorProject")

	config, err := LoadConfig(testCwd)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	result, err := ProcessConfig(&config[0], testCwd, "package.json", "tsconfig.json", false)
	if err != nil {
		t.Fatalf("Failed to process config: %v", err)
	}

	// Should find missing node modules (we'll add an import to a non-existent module)
	foundNonExistentModule := false
	foundNonExistentPkg := false
	for _, ruleResult := range result.RuleResults {
		if len(ruleResult.MissingNodeModules) > 0 {
			// Check if non-existent-module is found
			for _, missing := range ruleResult.MissingNodeModules {
				if missing.ModuleName == "non-existent-module" {
					foundNonExistentModule = true
					t.Logf("Found expected missing module: %s imported from: %v", missing.ModuleName, missing.ImportedFrom)
				}
				if missing.ModuleName == "non-existent-pkg" {
					foundNonExistentPkg = true
					t.Logf("Found expected missing module: %s imported from: %v", missing.ModuleName, missing.ImportedFrom)
				}
			}
		}
	}

	if !foundNonExistentModule {
		t.Errorf("Expected to find non-existent-module as missing node module, but got: %v", result.RuleResults)
	}

	if !foundNonExistentPkg {
		t.Errorf("Expected to find non-existent-pkg as missing node module, but got: %v", result.RuleResults)
	}

}

func TestFilterFilesForRule_FollowMonorepoPackagesSelective(t *testing.T) {
	cwd := "/repo"
	rulePath := "packages/consumer"
	consumerFile := "/repo/packages/consumer/src/index.ts"
	allowedFile := "/repo/packages/allowed/src/index.ts"
	disallowedFile := "/repo/packages/disallowed/src/index.ts"

	fullTree := MinimalDependencyTree{
		consumerFile: {
			{ID: allowedFile},
			{ID: disallowedFile},
		},
		allowedFile:    {},
		disallowedFile: {},
	}

	resolverManager := &ResolverManager{
		monorepoContext: &MonorepoContext{
			PackageToPath: map[string]string{
				"@scope/allowed":    "/repo/packages/allowed",
				"@scope/disallowed": "/repo/packages/disallowed",
			},
		},
	}

	ruleFiles, ruleTree := filterFilesForRule(
		fullTree,
		rulePath,
		cwd,
		FollowMonorepoPackagesValue{Packages: map[string]bool{"@scope/allowed": true}},
		resolverManager,
	)

	if !containsString(ruleFiles, consumerFile) {
		t.Fatalf("expected consumer file to be present in ruleFiles: %v", ruleFiles)
	}
	if !containsString(ruleFiles, allowedFile) {
		t.Fatalf("expected allowed package file to be present in ruleFiles: %v", ruleFiles)
	}
	if containsString(ruleFiles, disallowedFile) {
		t.Fatalf("expected disallowed package file to be excluded from ruleFiles: %v", ruleFiles)
	}

	if _, ok := ruleTree[disallowedFile]; ok {
		t.Fatalf("expected disallowed package file to be excluded from ruleTree")
	}
}

func TestFilterFilesForRule_WindowsStyleRootPathDot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific regression test")
	}

	cwd := `C:\repo\`
	rulePath := `.\`
	fileA := `C:\repo\src\index.ts`
	fileB := `C:\repo\src\feature\a.ts`
	fileOutside := `C:\another-repo\src\outside.ts`

	fullTree := MinimalDependencyTree{
		fileA:       {},
		fileB:       {},
		fileOutside: {},
	}

	ruleFiles, ruleTree := filterFilesForRule(
		fullTree,
		rulePath,
		cwd,
		FollowMonorepoPackagesValue{},
		nil,
	)

	if !containsString(ruleFiles, fileA) {
		t.Fatalf("expected fileA to be included for rule path '.', got: %v", ruleFiles)
	}
	if !containsString(ruleFiles, fileB) {
		t.Fatalf("expected fileB to be included for rule path '.', got: %v", ruleFiles)
	}
	if containsString(ruleFiles, fileOutside) {
		t.Fatalf("expected outside file to be excluded, got: %v", ruleFiles)
	}

	if _, ok := ruleTree[fileA]; !ok {
		t.Fatalf("expected fileA to be present in ruleTree")
	}
	if _, ok := ruleTree[fileB]; !ok {
		t.Fatalf("expected fileB to be present in ruleTree")
	}
	if _, ok := ruleTree[fileOutside]; ok {
		t.Fatalf("expected outside file to be excluded from ruleTree")
	}
}

func TestFilterFilesForRule_RootPathDot(t *testing.T) {
	cwd := "/repo"
	rulePath := "."
	fileA := "/repo/src/index.ts"
	fileB := "/repo/src/feature/a.ts"
	fileOutside := "/another-repo/src/outside.ts"

	fullTree := MinimalDependencyTree{
		fileA:       {},
		fileB:       {},
		fileOutside: {},
	}

	ruleFiles, ruleTree := filterFilesForRule(
		fullTree,
		rulePath,
		cwd,
		FollowMonorepoPackagesValue{},
		nil,
	)

	if !containsString(ruleFiles, fileA) {
		t.Fatalf("expected fileA to be included for rule path '.', got: %v", ruleFiles)
	}
	if !containsString(ruleFiles, fileB) {
		t.Fatalf("expected fileB to be included for rule path '.', got: %v", ruleFiles)
	}
	if containsString(ruleFiles, fileOutside) {
		t.Fatalf("expected outside file to be excluded, got: %v", ruleFiles)
	}

	if _, ok := ruleTree[fileA]; !ok {
		t.Fatalf("expected fileA to be present in ruleTree")
	}
	if _, ok := ruleTree[fileB]; !ok {
		t.Fatalf("expected fileB to be present in ruleTree")
	}
	if _, ok := ruleTree[fileOutside]; ok {
		t.Fatalf("expected outside file to be excluded from ruleTree")
	}
}

func TestConfigProcessor_MultipleRules(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/configProcessorProject")

	// Create a config with multiple rules
	multiRuleConfig := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": "src/features",
				"circularImportsDetection": {"enabled": true}
			},
			{
				"path": "src/utils",
				"orphanFilesDetection": {"enabled": true}
			}
		]
	}`

	configPath := filepath.Join(testCwd, "multi-rule-config.json")
	err := os.WriteFile(configPath, []byte(multiRuleConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write multi-rule config: %v", err)
	}
	defer os.Remove(configPath)

	configs, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load multi-rule config: %v", err)
	}

	result, err := ProcessConfig(&configs[0], testCwd, "package.json", "tsconfig.json", false)
	if err != nil {
		t.Fatalf("Failed to process multi-rule config: %v", err)
	}

	// Should have results for multiple rules
	if len(result.RuleResults) != 2 {
		t.Errorf("Expected 2 rule results, got %d", len(result.RuleResults))
	}
}

func TestConfigProcessor_RulePathFiltering(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/configProcessorProject")

	// Create a config with a specific rule path
	specificPathConfig := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": "src/features",
				"circularImportsDetection": {"enabled": true}
			}
		]
	}`

	configPath := filepath.Join(testCwd, "specific-path-config.json")
	err := os.WriteFile(configPath, []byte(specificPathConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write specific path config: %v", err)
	}
	defer os.Remove(configPath)

	configs, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load specific path config: %v", err)
	}

	result, err := ProcessConfig(&configs[0], testCwd, "package.json", "tsconfig.json", false)
	if err != nil {
		t.Fatalf("Failed to process specific path config: %v", err)
	}

	// Should only process files in src/features
	if len(result.RuleResults) != 1 {
		t.Errorf("Expected 1 rule result, got %d", len(result.RuleResults))
	}

	ruleResult := result.RuleResults[0]
	if ruleResult.RulePath != "src/features" {
		t.Errorf("Expected rule path 'src/features', got '%s'", ruleResult.RulePath)
	}
}

func TestConfigProcessor_NewFields(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/configProcessorProject")

	// Create a config with all checks enabled to test new fields
	allChecksConfig := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": "src/features",
				"circularImportsDetection": {"enabled": true},
				"orphanFilesDetection": {"enabled": true},
				"moduleBoundaries": [
					{
						"name": "test-boundary",
						"pattern": "src/features/**/*",
						"allow": ["src/utils/**/*"]
					}
				],
				"unusedNodeModulesDetection": {"enabled": true},
				"missingNodeModulesDetection": {"enabled": true}
			}
		]
	}`

	configPath := filepath.Join(testCwd, "all-checks-config.json")
	err := os.WriteFile(configPath, []byte(allChecksConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write all checks config: %v", err)
	}
	defer os.Remove(configPath)

	configs, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load all checks config: %v", err)
	}

	result, err := ProcessConfig(&configs[0], testCwd, "package.json", "tsconfig.json", false)
	if err != nil {
		t.Fatalf("Failed to process all checks config: %v", err)
	}

	// Should have 1 rule result
	if len(result.RuleResults) != 1 {
		t.Fatalf("Expected 1 rule result, got %d", len(result.RuleResults))
	}

	ruleResult := result.RuleResults[0]

	// Test new fields
	if ruleResult.FileCount < 0 {
		t.Errorf("FileCount should be non-negative, got %d", ruleResult.FileCount)
	}

	// Check that dependency tree is included
	if len(ruleResult.DependencyTree) == 0 {
		t.Errorf("DependencyTree should not be empty, got %d entries", len(ruleResult.DependencyTree))
	}

	// Should have all 5 checks enabled
	expectedChecks := []string{"circular-imports", "orphan-files", "module-boundaries", "unused-node-modules", "missing-node-modules"}
	if len(ruleResult.EnabledChecks) != len(expectedChecks) {
		t.Errorf("Expected %d enabled checks, got %d", len(expectedChecks), len(ruleResult.EnabledChecks))
	}

	for _, expectedCheck := range expectedChecks {
		if !containsString(ruleResult.EnabledChecks, expectedCheck) {
			t.Errorf("Expected enabled check '%s' not found in %v", expectedCheck, ruleResult.EnabledChecks)
		}
	}

	// Test rule path
	if ruleResult.RulePath != "src/features" {
		t.Errorf("Expected rule path 'src/features', got '%s'", ruleResult.RulePath)
	}
}
