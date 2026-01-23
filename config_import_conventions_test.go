package main

import (
	"testing"
)

func TestParseConfig_ImportConventions_SimplifiedMode(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": [
					{
						"rule": "relative-internal-absolute-external",
						"domains": ["src/*"]
					}
				]
			}
		]
	}`

	configs, err := ParseConfig([]byte(configJSON))
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(configs) != 1 {
		t.Errorf("Expected 1 config, got %d", len(configs))
	}

	config := configs[0]
	if len(config.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(config.Rules))
	}

	rule := config.Rules[0]
	if len(rule.ImportConventions) != 1 {
		t.Errorf("Expected 1 import convention, got %d", len(rule.ImportConventions))
	}

	convention := rule.ImportConventions[0]
	if convention.Rule != "relative-internal-absolute-external" {
		t.Errorf("Expected rule 'relative-internal-absolute-external', got '%s'", convention.Rule)
	}

	// Check that domains is interface{} and can be processed
	if convention.Domains == nil {
		t.Error("Expected domains to be set, got nil")
	}
}

func TestParseConfig_ImportConventions_AdvancedMode(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": [
					{
						"rule": "relative-internal-absolute-external",
						"domains": [
							{ "path": "src/features/domain1", "alias": "@domain1" },
							{ "path": "src/shared/ui", "alias": "@ui-kit" }
						]
					}
				]
			}
		]
	}`

	configs, err := ParseConfig([]byte(configJSON))
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(configs) != 1 {
		t.Errorf("Expected 1 config, got %d", len(configs))
	}

	config := configs[0]
	if len(config.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(config.Rules))
	}

	rule := config.Rules[0]
	if len(rule.ImportConventions) != 1 {
		t.Errorf("Expected 1 import convention, got %d", len(rule.ImportConventions))
	}

	convention := rule.ImportConventions[0]
	if convention.Rule != "relative-internal-absolute-external" {
		t.Errorf("Expected rule 'relative-internal-absolute-external', got '%s'", convention.Rule)
	}

	// Check that domains is interface{} and can be processed
	if convention.Domains == nil {
		t.Error("Expected domains to be set, got nil")
	}
}

func TestParseConfig_ImportConventions_InvalidRuleName(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": [
					{
						"rule": "invalid-rule",
						"domains": ["src/*"]
					}
				]
			}
		]
	}`

	_, err := ParseConfig([]byte(configJSON))
	if err == nil {
		t.Error("Expected error for invalid rule name, got nil")
	}

	expectedError := "unknown rule 'invalid-rule'"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestParseConfig_ImportConventions_MixedDomainsRejected(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": [
					{
						"rule": "relative-internal-absolute-external",
						"domains": [
							"src/*",
							{ "path": "src/shared/ui", "alias": "@ui-kit" }
						]
					}
				]
			}
		]
	}`

	_, err := ParseConfig([]byte(configJSON))
	if err == nil {
		t.Error("Expected error for mixed domains, got nil")
	}

	expectedError := "cannot mix strings and objects"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestParseConfig_ImportConventions_EmptyDomainsRejected(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": [
					{
						"rule": "relative-internal-absolute-external",
						"domains": []
					}
				]
			}
		]
	}`

	_, err := ParseConfig([]byte(configJSON))
	if err == nil {
		t.Error("Expected error for empty domains, got nil")
	}

	expectedError := "domains cannot be empty"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestParseConfig_ImportConventions_MissingPathRejected(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": [
					{
						"rule": "relative-internal-absolute-external",
						"domains": [
							{ "alias": "@domain1" }
						]
					}
				]
			}
		]
	}`

	_, err := ParseConfig([]byte(configJSON))
	if err == nil {
		t.Error("Expected error for missing path, got nil")
	}

	expectedError := "path is required"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestParseConfig_ImportConventions_MissingAliasRejected(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": [
					{
						"rule": "relative-internal-absolute-external",
						"domains": [
							{ "path": "src/domain1" }
						]
					}
				]
			}
		]
	}`

	// This should actually pass since alias is optional
	configs, err := ParseConfig([]byte(configJSON))
	if err != nil {
		t.Errorf("Expected no error for missing alias (optional), got %v", err)
	}

	if len(configs) != 1 {
		t.Errorf("Expected 1 config, got %d", len(configs))
	}
}

func TestParseConfig_ImportConventions_NestedDomainsRejected(t *testing.T) {
	// Test case 1: src/auth and src/auth/utils are nested
	configJSON1 := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": [
					{
						"rule": "relative-internal-absolute-external",
						"domains": ["src/auth", "src/auth/utils"]
					}
				]
			}
		]
	}`

	_, err1 := ParseConfig([]byte(configJSON1))
	if err1 == nil {
		t.Error("Expected error for nested domains (src/auth and src/auth/utils), got nil")
	}

	expectedError1 := "nested domains not allowed"
	if !contains(err1.Error(), expectedError1) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError1, err1.Error())
	}

	// Test case 2: src and src/auth are nested (src contains src/auth)
	configJSON2 := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": [
					{
						"rule": "relative-internal-absolute-external",
						"domains": ["src/auth", "src"]
					}
				]
			}
		]
	}`

	_, err2 := ParseConfig([]byte(configJSON2))
	if err2 == nil {
		t.Error("Expected error for nested domains (src and src/auth), got nil")
	}

	if !contains(err2.Error(), expectedError1) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError1, err2.Error())
	}
}

func TestParseConfig_ImportConventions_UnknownField(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": [
					{
						"rule": "relative-internal-absolute-external",
						"domains": ["src/*"],
						"unknownField": "value"
					}
				]
			}
		]
	}`

	_, err := ParseConfig([]byte(configJSON))
	if err == nil {
		t.Error("Expected error for unknown field, got nil")
	}

	expectedError := "unknown field 'unknownField'"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestParseConfig_ImportConventions_MissingRule(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": [
					{
						"domains": ["src/*"]
					}
				]
			}
		]
	}`

	_, err := ParseConfig([]byte(configJSON))
	if err == nil {
		t.Error("Expected error for missing rule, got nil")
	}

	expectedError := "rule is required"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestParseConfig_ImportConventions_MissingDomains(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": [
					{
						"rule": "relative-internal-absolute-external"
					}
				]
			}
		]
	}`

	_, err := ParseConfig([]byte(configJSON))
	if err == nil {
		t.Error("Expected error for missing domains, got nil")
	}

	expectedError := "domains is required"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestParseConfig_ImportConventions_EmptyImportConventions(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"importConventions": []
			}
		]
	}`

	_, err := ParseConfig([]byte(configJSON))
	if err == nil {
		t.Error("Expected error for empty import conventions, got nil")
	}

	expectedError := "importConventions cannot be empty"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}
