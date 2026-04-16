package config

import "testing"

func TestRestrictedDevDependenciesUsageValidation(t *testing.T) {
	// Test valid configuration
	validConfig := RestrictedDevDependenciesUsageOptions{
		Enabled:         true,
		ProdEntryPoints: []string{"src/main.tsx", "src/pages/**/*.tsx"},
	}

	err := ValidateRestrictedDevDependenciesUsageOptions(&validConfig, "test")
	if err != nil {
		t.Errorf("Expected valid config to pass validation, got error: %v", err)
	}

	// Test disabled config (should pass even without entry points)
	disabledConfig := RestrictedDevDependenciesUsageOptions{
		Enabled: false,
	}

	err = ValidateRestrictedDevDependenciesUsageOptions(&disabledConfig, "test")
	if err != nil {
		t.Errorf("Expected disabled config to pass validation, got error: %v", err)
	}

	// Test invalid config with empty entry point
	invalidConfig := RestrictedDevDependenciesUsageOptions{
		Enabled:         true,
		ProdEntryPoints: []string{"", "src/main.tsx"},
	}

	err = ValidateRestrictedDevDependenciesUsageOptions(&invalidConfig, "test")
	if err == nil {
		t.Error("Expected config with empty entry point to fail validation")
	}
}
