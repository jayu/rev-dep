package cli

import (
	"os"
	"path/filepath"
	"testing"

	"rev-dep-go/internal/config"
)

func firstDetectionOrNil[T any](items []*T) *T {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

func TestInitConfigFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-init-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test basic init
	configPath, _, _, err := initConfigFileCore(tempDir)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Check that config file was created
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("Expected config file to exist at %s", configPath)
	}

	// Test that init fails when config already exists
	_, _, _, err = initConfigFileCore(tempDir)
	if err == nil {
		t.Errorf("Expected error when config already exists, got nil")
	}

	// Remove the config and test with different variants
	_ = os.Remove(configPath)

	// Test with .rev-dep.config.json
	hiddenConfigPath := filepath.Join(tempDir, ".rev-dep.config.json")
	_ = os.WriteFile(hiddenConfigPath, []byte(`{"configVersion": "1.0"}`), 0644)
	_, _, _, err = initConfigFileCore(tempDir)
	if err == nil {
		t.Errorf("Expected error when hidden config exists, got nil")
	}
	_ = os.Remove(hiddenConfigPath)

	// Test with rev-dep.config.jsonc
	jsoncConfigPath := filepath.Join(tempDir, "rev-dep.config.jsonc")
	_ = os.WriteFile(jsoncConfigPath, []byte(`{"configVersion": "1.0"}`), 0644)
	_, _, _, err = initConfigFileCore(tempDir)
	if err == nil {
		t.Errorf("Expected error when jsonc config exists, got nil")
	}
	_ = os.Remove(jsoncConfigPath)

	// Test with .rev-dep.config.jsonc
	hiddenJsoncConfigPath := filepath.Join(tempDir, ".rev-dep.config.jsonc")
	_ = os.WriteFile(hiddenJsoncConfigPath, []byte(`{"configVersion": "1.0"}`), 0644)
	_, _, _, err = initConfigFileCore(tempDir)
	if err == nil {
		t.Errorf("Expected error when hidden jsonc config exists, got nil")
	}
	_ = os.Remove(hiddenJsoncConfigPath)

	// Now test that it works when no config files exist
	configPath, _, _, err = initConfigFileCore(tempDir)
	if err != nil {
		t.Errorf("Expected no error when no config files exist, got %v", err)
	}

	// Read and verify the generated config
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	cfg, err := config.ParseConfig(content)
	if err != nil {
		t.Errorf("Failed to parse generated config: %v", err)
	}

	if cfg.ConfigVersion != "1.7" {
		t.Errorf("Expected configVersion '1.7', got '%s'", cfg.ConfigVersion)
	}

	if len(cfg.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(cfg.Rules))
	}

	if cfg.Rules[0].Path != "." {
		t.Errorf("Expected rule path '.', got '%s'", cfg.Rules[0].Path)
	}

	if firstDetectionOrNil(cfg.Rules[0].CircularImportsDetections) == nil || !firstDetectionOrNil(cfg.Rules[0].CircularImportsDetections).Enabled {
		t.Errorf("Expected circular imports detection to be enabled")
	}

	if firstDetectionOrNil(cfg.Rules[0].CircularImportsDetections).IgnoreTypeImports {
		t.Errorf("Expected circular imports detection ignoreTypeImports to be false")
	}

	if firstDetectionOrNil(cfg.Rules[0].OrphanFilesDetections) == nil || firstDetectionOrNil(cfg.Rules[0].OrphanFilesDetections).Enabled {
		t.Errorf("Expected orphan files detection to be disabled")
	}

	if firstDetectionOrNil(cfg.Rules[0].UnusedNodeModulesDetections) == nil || firstDetectionOrNil(cfg.Rules[0].UnusedNodeModulesDetections).Enabled {
		t.Errorf("Expected unused node modules detection to be disabled")
	}

	if firstDetectionOrNil(cfg.Rules[0].MissingNodeModulesDetections) == nil || firstDetectionOrNil(cfg.Rules[0].MissingNodeModulesDetections).Enabled {
		t.Errorf("Expected missing node modules detection to be disabled")
	}

	if firstDetectionOrNil(cfg.Rules[0].UnusedExportsDetections) == nil || firstDetectionOrNil(cfg.Rules[0].UnusedExportsDetections).Enabled {
		t.Errorf("Expected unused exports detection to be disabled")
	}

	if firstDetectionOrNil(cfg.Rules[0].UnresolvedImportsDetections) == nil || !firstDetectionOrNil(cfg.Rules[0].UnresolvedImportsDetections).Enabled {
		t.Errorf("Expected unresolved imports detection to be enabled")
	}

	if firstDetectionOrNil(cfg.Rules[0].DevDepsUsageOnProdDetections) == nil || firstDetectionOrNil(cfg.Rules[0].DevDepsUsageOnProdDetections).Enabled {
		t.Errorf("Expected dev deps usage detection to be disabled")
	}

	if firstDetectionOrNil(cfg.Rules[0].RestrictedImportsDetections) == nil || firstDetectionOrNil(cfg.Rules[0].RestrictedImportsDetections).Enabled {
		t.Errorf("Expected restricted imports detection to be disabled")
	}
}

func TestInitConfigFile_MonorepoSubpackage(t *testing.T) {
	// Create a temporary monorepo structure
	tempDir, err := os.MkdirTemp("", "rev-dep-monorepo-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// workspace root package.json with workspaces
	rootPkg := filepath.Join(tempDir, "package.json")
	rootContent := `{"name":"root","workspaces":["packages/*"]}`
	if err := os.WriteFile(rootPkg, []byte(rootContent), 0644); err != nil {
		t.Fatalf("Failed to write root package.json: %v", err)
	}

	// create a package inside packages/pkg1
	pkgDir := filepath.Join(tempDir, "packages", "pkg1")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("Failed to create package dir: %v", err)
	}
	pkgJson := filepath.Join(pkgDir, "package.json")
	pkgContent := `{"name":"@myorg/pkg1"}`
	if err := os.WriteFile(pkgJson, []byte(pkgContent), 0644); err != nil {
		t.Fatalf("Failed to write package.json for pkg1: %v", err)
	}

	// Run init in the sub-package directory
	configPath, rules, createdForSubPackage, err := initConfigFileCore(pkgDir)
	if err != nil {
		t.Fatalf("initConfigFileCore failed: %v", err)
	}
	if !createdForSubPackage {
		t.Fatalf("Expected createdForSubPackage to be true when running inside a workspace package")
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("Expected config file to exist at %s", configPath)
	}

	// Parse generated config and validate single rule
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read generated config: %v", err)
	}
	cfg, err := config.ParseConfig(content)
	if err != nil {
		t.Fatalf("Failed to parse generated config: %v", err)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("Expected 1 rule for sub-package config, got %d (rules: %v)", len(cfg.Rules), rules)
	}
	if cfg.Rules[0].Path != "." {
		t.Fatalf("Expected rule path '.' for sub-package config, got '%s'", cfg.Rules[0].Path)
	}
	if firstDetectionOrNil(cfg.Rules[0].UnresolvedImportsDetections) == nil || !firstDetectionOrNil(cfg.Rules[0].UnresolvedImportsDetections).Enabled {
		t.Fatalf("Expected unresolvedImportsDetection enabled in generated sub-package config")
	}
	if firstDetectionOrNil(cfg.Rules[0].DevDepsUsageOnProdDetections) == nil || firstDetectionOrNil(cfg.Rules[0].DevDepsUsageOnProdDetections).Enabled {
		t.Fatalf("Expected devDepsUsageOnProdDetection disabled in generated sub-package config")
	}
	if firstDetectionOrNil(cfg.Rules[0].RestrictedImportsDetections) == nil || firstDetectionOrNil(cfg.Rules[0].RestrictedImportsDetections).Enabled {
		t.Fatalf("Expected restrictedImportsDetection disabled in generated sub-package config")
	}

	// Now run init at the workspace root and expect multiple rules
	// Remove config created in package
	_ = os.Remove(configPath)

	rootConfigPath, rootRules, createdForRoot, err := initConfigFileCore(tempDir)
	if err != nil {
		t.Fatalf("initConfigFileCore failed at root: %v", err)
	}
	if createdForRoot {
		t.Fatalf("Expected createdForMonorepoSubPackage=false when running at monorepo root")
	}
	if _, err := os.Stat(rootConfigPath); err != nil {
		t.Fatalf("Expected root config file to exist at %s", rootConfigPath)
	}
	// rootRules should contain at least the root + discovered package
	if len(rootRules) < 2 {
		t.Fatalf("Expected >=2 rules for monorepo root config, got %d", len(rootRules))
	}

	rootConfigContent, err := os.ReadFile(rootConfigPath)
	if err != nil {
		t.Fatalf("Failed to read generated root config: %v", err)
	}
	rootConfig, err := config.ParseConfig(rootConfigContent)
	if err != nil {
		t.Fatalf("Failed to parse generated root config: %v", err)
	}
	if len(rootConfig.Rules) == 0 {
		t.Fatalf("Expected parsed root config with at least one rule")
	}
}
