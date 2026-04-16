package monorepo_test

import (
	"testing"

	monorepo "rev-dep-go/internal/monorepo"
)

func TestGetDevDependenciesFromConfig(t *testing.T) {
	// Test with valid config
	config := &monorepo.PackageJsonConfig{
		DevDependencies: map[string]string{
			"lodash": "^4.0.0",
			"eslint": "^8.0.0",
		},
	}

	devDeps := monorepo.GetDevDependenciesFromConfig(config)

	expected := map[string]bool{
		"lodash": true,
		"eslint": true,
	}

	if len(devDeps) != len(expected) {
		t.Errorf("Expected %d dev dependencies, got %d", len(expected), len(devDeps))
	}

	for dep := range expected {
		if !devDeps[dep] {
			t.Errorf("Expected dev dependency %s to be present", dep)
		}
	}
}

func TestGetDevDependenciesFromConfig_Nil(t *testing.T) {
	devDeps := monorepo.GetDevDependenciesFromConfig(nil)

	if len(devDeps) != 0 {
		t.Errorf("Expected 0 dev dependencies with nil config, got %d", len(devDeps))
	}
}

func TestGetDevDependenciesFromConfig_Empty(t *testing.T) {
	config := &monorepo.PackageJsonConfig{
		DevDependencies: map[string]string{},
	}

	devDeps := monorepo.GetDevDependenciesFromConfig(config)

	if len(devDeps) != 0 {
		t.Errorf("Expected 0 dev dependencies with empty config, got %d", len(devDeps))
	}
}

func TestGetProductionDependenciesFromConfig(t *testing.T) {
	// Test with valid config
	config := &monorepo.PackageJsonConfig{
		Dependencies: map[string]string{
			"react": "^18.0.0",
			"axios": "^1.0.0",
		},
	}

	prodDeps := monorepo.GetProductionDependenciesFromConfig(config)

	expected := map[string]bool{
		"react": true,
		"axios": true,
	}

	if len(prodDeps) != len(expected) {
		t.Errorf("Expected %d production dependencies, got %d", len(expected), len(prodDeps))
	}

	for dep := range expected {
		if !prodDeps[dep] {
			t.Errorf("Expected production dependency %s to be present", dep)
		}
	}
}

func TestGetProductionDependenciesFromConfig_Nil(t *testing.T) {
	prodDeps := monorepo.GetProductionDependenciesFromConfig(nil)

	if len(prodDeps) != 0 {
		t.Errorf("Expected 0 production dependencies with nil config, got %d", len(prodDeps))
	}
}

func TestGetProductionDependenciesFromConfig_Empty(t *testing.T) {
	config := &monorepo.PackageJsonConfig{
		Dependencies: map[string]string{},
	}

	prodDeps := monorepo.GetProductionDependenciesFromConfig(config)

	if len(prodDeps) != 0 {
		t.Errorf("Expected 0 production dependencies with empty config, got %d", len(prodDeps))
	}
}
