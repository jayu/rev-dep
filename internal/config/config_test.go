package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"rev-dep-go/internal/model"
)

func firstDetectionOrNil[T any](items []*T) *T {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

func TestParseConfig_SchemaField(t *testing.T) {
	configJSON := `{
		"$schema": "./config-schema/1.0.schema.json",
		"configVersion": "1.0",
		"rules": [
			{
				"path": ".",
				"circularImportsDetection": {"enabled": true}
			}
		]
	}`

	config, err := ParseConfig([]byte(configJSON))
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if config.ConfigVersion != "1.0" {
		t.Errorf("Expected configVersion '1.0', got '%s'", config.ConfigVersion)
	}

	if len(config.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(config.Rules))
	}

	if config.Rules[0].Path != "." {
		t.Errorf("Expected rule path '.', got '%s'", config.Rules[0].Path)
	}
}

func TestParseConfig_ValidMinimalConfig(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [
			{
				"path": "./src"
			}
		]
	}`

	config, err := ParseConfig([]byte(configJSON))
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if config.ConfigVersion != "1.0" {
		t.Errorf("Expected configVersion '1.0', got '%s'", config.ConfigVersion)
	}

	if len(config.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(config.Rules))
	}

	if config.Rules[0].Path != "./src" {
		t.Errorf("Expected rule path './src', got '%s'", config.Rules[0].Path)
	}
}

func TestParseConfig_ValidCompleteConfig(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"conditionNames": ["node", "imports"],
		"customAssetExtensions": ["glb", "mp3"],
		"ignoreFiles": ["dist/**/*", "build/**/*"],
		"rules": [
			{
				"path": "./src",
				"moduleBoundaries": [
					{
						"name": "UI Components",
						"pattern": "src/components/**",
						"allow": ["src/utils/**", "src/types/**"],
						"deny": ["src/api/**"]
					}
				],
				"circularImportsDetection": {
					"enabled": true,
					"ignoreTypeImports": true
				},
				"orphanFilesDetection": {
					"enabled": true,
					"validEntryPoints": ["src/index.ts"],
					"ignoreTypeImports": false,
					"graphExclude": ["**/*.test.ts"]
				},
				"unusedNodeModulesDetection": {
					"enabled": true,
					"includeModules": ["@myorg/**"],
					"excludeModules": ["@types/**"],
					"pkgJsonFieldsWithBinaries": ["bin"],
					"filesWithBinaries": ["scripts/**"],
					"filesWithModules": ["config/**"],
					"outputType": "groupByModule"
				},
				"missingNodeModulesDetection": {
					"enabled": true,
					"includeModules": ["lodash/**"],
					"excludeModules": ["@types/**"],
					"outputType": "groupByFile"
				}
			}
		]
	}`

	config, err := ParseConfig([]byte(configJSON))
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Check global settings
	if len(config.ConditionNames) != 2 || config.ConditionNames[0] != "node" || config.ConditionNames[1] != "imports" {
		t.Errorf("Unexpected conditionNames: %v", config.ConditionNames)
	}

	if len(config.IgnoreFiles) != 2 || config.IgnoreFiles[0] != "dist/**/*" || config.IgnoreFiles[1] != "build/**/*" {
		t.Errorf("Unexpected ignoreFiles: %v", config.IgnoreFiles)
	}
	if len(config.CustomAssetExtensions) != 2 || config.CustomAssetExtensions[0] != "glb" || config.CustomAssetExtensions[1] != "mp3" {
		t.Errorf("Unexpected customAssetExtensions: %v", config.CustomAssetExtensions)
	}

	rule := config.Rules[0]

	// Check module boundaries
	if len(rule.ModuleBoundaries) != 1 {
		t.Errorf("Expected 1 module boundary, got %d", len(rule.ModuleBoundaries))
	}

	boundary := rule.ModuleBoundaries[0]
	if boundary.Name != "UI Components" {
		t.Errorf("Expected boundary name 'UI Components', got '%s'", boundary.Name)
	}

	// Check detection options are properly configured
	if firstDetectionOrNil(rule.CircularImportsDetections) == nil || !firstDetectionOrNil(rule.CircularImportsDetections).Enabled || !firstDetectionOrNil(rule.CircularImportsDetections).IgnoreTypeImports {
		t.Errorf("Circular imports detection not properly configured")
	}

	if firstDetectionOrNil(rule.OrphanFilesDetections) == nil || !firstDetectionOrNil(rule.OrphanFilesDetections).Enabled || firstDetectionOrNil(rule.OrphanFilesDetections).IgnoreTypeImports {
		t.Errorf("Orphan files detection not properly configured")
	}

	if firstDetectionOrNil(rule.UnusedNodeModulesDetections) == nil || !firstDetectionOrNil(rule.UnusedNodeModulesDetections).Enabled || firstDetectionOrNil(rule.UnusedNodeModulesDetections).OutputType != "groupByModule" {
		t.Errorf("Unused node modules detection not properly configured")
	}

	if firstDetectionOrNil(rule.MissingNodeModulesDetections) == nil || !firstDetectionOrNil(rule.MissingNodeModulesDetections).Enabled || firstDetectionOrNil(rule.MissingNodeModulesDetections).OutputType != "groupByFile" {
		t.Errorf("Missing node modules detection not properly configured")
	}
}

func TestParseConfig_RequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		expectedErr string
	}{
		{
			name: "missing configVersion",
			configJSON: `{
				"rules": [{"path": "./src"}]
			}`,
			expectedErr: "configVersion is required",
		},
		{
			name: "missing rule path",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{}]
			}`,
			expectedErr: "rules[0].path is required",
		},
		{
			name: "missing boundary name",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{"pattern": "src/**"}]
				}]
			}`,
			expectedErr: "rules[0].moduleBoundaries[0].name is required",
		},
		{
			name: "missing boundary pattern",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{"name": "Test"}]
				}]
			}`,
			expectedErr: "rules[0].moduleBoundaries[0].pattern is required",
		},
		{
			name: "missing enabled field in detection options",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"circularImportsDetection": {}
				}]
			}`,
			expectedErr: "rules[0].circularImportsDetection.enabled is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tt.configJSON))
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			} else if err.Error() != tt.expectedErr {
				t.Errorf("Expected error '%s', got '%s'", tt.expectedErr, err.Error())
			}
		})
	}
}

func TestParseConfig_UnknownFields(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		expectedErr string
	}{
		{
			name: "unknown root field",
			configJSON: `{
				"configVersion": "1.0",
				"unknownField": "value",
				"rules": [{"path": "./src"}]
			}`,
			expectedErr: "unknown field 'unknownField' in config root",
		},
		{
			name: "unknown rule field",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"unknownField": "value"
				}]
			}`,
			expectedErr: "rules[0]: unknown field 'unknownField'",
		},
		{
			name: "unknown boundary field",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{
						"name": "Test",
						"pattern": "src/**",
						"unknownField": "value"
					}]
				}]
			}`,
			expectedErr: "rules[0].moduleBoundaries[0]: unknown field 'unknownField'",
		},
		{
			name: "unknown detection options field",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"circularImportsDetection": {
						"enabled": true,
						"unknownField": "value"
					}
				}]
			}`,
			expectedErr: "rules[0].circularImportsDetection: unknown field 'unknownField'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tt.configJSON))
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			} else if err.Error() != tt.expectedErr {
				t.Errorf("Expected error '%s', got '%s'", tt.expectedErr, err.Error())
			}
		})
	}
}

func TestParseConfig_CustomAssetExtensionsValidation(t *testing.T) {
	t.Run("wrong type for customAssetExtensions", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"customAssetExtensions": "not-an-array",
			"rules": [{"path": "."}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "customAssetExtensions must be an array") {
			t.Errorf("Expected customAssetExtensions array type error, got: %s", err.Error())
		}
	})

	t.Run("empty customAssetExtensions entry is rejected", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"customAssetExtensions": [""],
			"rules": [{"path": "."}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "customAssetExtensions[0] cannot be empty") {
			t.Errorf("Expected empty customAssetExtensions entry error, got: %s", err.Error())
		}
	})

	t.Run("dot-prefixed customAssetExtensions entry is rejected", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"customAssetExtensions": [".mp3"],
			"rules": [{"path": "."}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "customAssetExtensions[0] must not start with '.'") {
			t.Errorf("Expected dot-prefix customAssetExtensions entry error, got: %s", err.Error())
		}
	})

	t.Run("extension containing dot is accepted", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"customAssetExtensions": ["d.ts"],
			"rules": [{"path": "."}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if len(config.CustomAssetExtensions) != 1 || config.CustomAssetExtensions[0] != "d.ts" {
			t.Fatalf("Expected customAssetExtensions to contain d.ts, got: %v", config.CustomAssetExtensions)
		}
	})

	t.Run("extension with leading or trailing spaces is rejected", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"customAssetExtensions": ["  mp3  "],
			"rules": [{"path": "."}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "customAssetExtensions[0] must not have leading or trailing spaces") {
			t.Errorf("Expected spacing validation error, got: %s", err.Error())
		}
	})
}

func TestParseConfig_InvalidTypes(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		expectedErr string
	}{
		{
			name: "rules not array",
			configJSON: `{
				"configVersion": "1.0",
				"rules": {}
			}`,
			expectedErr: "rules must be an array",
		},
		{
			name: "rule not object",
			configJSON: `{
				"configVersion": "1.0",
				"rules": ["invalid"]
			}`,
			expectedErr: "rules[0] must be an object",
		},
		{
			name: "rule path not string",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{"path": 123}]
			}`,
			expectedErr: "rules[0].path must be a string",
		},
		{
			name: "rule path null",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{"path": null}]
			}`,
			expectedErr: "rules[0].path must be a string",
		},
		{
			name: "moduleBoundaries not array",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": {}
				}]
			}`,
			expectedErr: "moduleBoundaries must be an array",
		},
		{
			name: "boundary not object",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": ["invalid"]
				}]
			}`,
			expectedErr: "moduleBoundaries[0] must be an object",
		},
		{
			name: "boundary name not string",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{
						"name": 123,
						"pattern": "src/**"
					}]
				}]
			}`,
			expectedErr: "moduleBoundaries[0].name must be a string",
		},
		{
			name: "boundary allow not array",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{
						"name": "Test",
						"pattern": "src/**",
						"allow": "not-array"
					}]
				}]
			}`,
			expectedErr: "moduleBoundaries[0].allow must be an array",
		},
		{
			name: "detection options not object",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"circularImportsDetection": "not-object"
				}]
			}`,
			expectedErr: "circularImportsDetection must be an object",
		},
		{
			name: "enabled field not boolean",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"circularImportsDetection": {
						"enabled": "not-boolean"
					}
				}]
			}`,
			expectedErr: "enabled must be a boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tt.configJSON))
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			} else if !contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectedErr, err.Error())
			}
		})
	}
}

func TestParseConfig_InvalidPatterns(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		expectedErr string
	}{
		{
			name: "invalid boundary pattern",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{
						"name": "Test",
						"pattern": "./src/**",
						"allow": [],
						"deny": []
					}]
				}]
			}`,
			expectedErr: "pattern './src/**' starts with './'",
		},
		{
			name: "invalid allow pattern",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{
						"name": "Test",
						"pattern": "src/**",
						"allow": ["./utils/**"],
						"deny": []
					}]
				}]
			}`,
			expectedErr: "pattern './utils/**' starts with './'",
		},
		{
			name: "invalid deny pattern",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{
						"name": "Test",
						"pattern": "src/**",
						"allow": [],
						"deny": ["../external/**"]
					}]
				}]
			}`,
			expectedErr: "pattern '../external/**' starts with '../'",
		},
		{
			name: "invalid graph exclude pattern",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"orphanFilesDetection": {
						"enabled": true,
						"graphExclude": ["./invalid/**"]
					}
				}]
			}`,
			expectedErr: "pattern './invalid/**' starts with './'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tt.configJSON))
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			} else if !contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectedErr, err.Error())
			}
		})
	}
}

func TestParseConfig_OutputTypes(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		shouldError bool
		expectedErr string
	}{
		{
			name: "valid output types",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"unusedNodeModulesDetection": {
						"enabled": true,
						"outputType": "groupByModule"
					},
					"missingNodeModulesDetection": {
						"enabled": true,
						"outputType": "groupByFile"
					}
				}]
			}`,
			shouldError: false,
		},
		{
			name: "empty output type",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"unusedNodeModulesDetection": {
						"enabled": true,
						"outputType": ""
					}
				}]
			}`,
			shouldError: false,
		},
		{
			name: "invalid output type",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"unusedNodeModulesDetection": {
						"enabled": true,
						"outputType": "invalidType"
					}
				}]
			}`,
			shouldError: true,
			expectedErr: "must be one of 'list', 'groupByModule', 'groupByFile'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tt.configJSON))
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.name)
				} else if !contains(err.Error(), tt.expectedErr) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedErr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s, got %v", tt.name, err)
				}
			}
		})
	}
}

func TestParseConfig_NullFields(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		shouldError bool
		expectedErr string
	}{
		{
			name: "null boundary name",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{
						"name": null,
						"pattern": "src/**"
					}]
				}]
			}`,
			shouldError: true,
			expectedErr: "name cannot be null",
		},
		{
			name: "null boundary pattern",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{
						"name": "Test",
						"pattern": null
					}]
				}]
			}`,
			shouldError: true,
			expectedErr: "pattern cannot be null",
		},
		{
			name: "null enabled field",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"circularImportsDetection": {
						"enabled": null
					}
				}]
			}`,
			shouldError: true,
			expectedErr: "enabled cannot be null",
		},
		{
			name: "null optional fields allowed",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{
						"name": "Test",
						"pattern": "src/**",
						"allow": null,
						"deny": null
					}],
					"orphanFilesDetection": {
						"enabled": true,
						"validEntryPoints": null,
						"ignoreTypeImports": null,
						"graphExclude": null
					}
				}]
			}`,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tt.configJSON))
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.name)
				} else if !contains(err.Error(), tt.expectedErr) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedErr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s, got %v", tt.name, err)
				}
			}
		})
	}
}

func TestParseConfig_DisabledOptions(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0",
		"rules": [{
			"path": "./src",
			"circularImportsDetection": {
				"enabled": false,
				"ignoreTypeImports": true
			},
			"orphanFilesDetection": {
				"enabled": false,
				"validEntryPoints": [""],
				"ignoreTypeImports": false,
				"graphExclude": ["./invalid/**"]
			},
			"unusedNodeModulesDetection": {
				"enabled": false,
				"outputType": "invalidType"
			},
			"missingNodeModulesDetection": {
				"enabled": false,
				"outputType": "invalidType"
			}
		}]
	}`

	// Should pass validation because all options are disabled
	config, err := ParseConfig([]byte(configJSON))
	if err != nil {
		t.Errorf("Expected no error for disabled options, got %v", err)
	}
	rule := config.Rules[0]

	// Verify options are parsed but validation is skipped when disabled
	if firstDetectionOrNil(rule.CircularImportsDetections) == nil || firstDetectionOrNil(rule.CircularImportsDetections).Enabled {
		t.Error("Circular imports detection should be disabled")
	}

	if firstDetectionOrNil(rule.OrphanFilesDetections) == nil || firstDetectionOrNil(rule.OrphanFilesDetections).Enabled {
		t.Error("Orphan files detection should be disabled")
	}

	if firstDetectionOrNil(rule.UnusedNodeModulesDetections) == nil || firstDetectionOrNil(rule.UnusedNodeModulesDetections).Enabled {
		t.Error("Unused node modules detection should be disabled")
	}

	if firstDetectionOrNil(rule.MissingNodeModulesDetections) == nil || firstDetectionOrNil(rule.MissingNodeModulesDetections).Enabled {
		t.Error("Missing node modules detection should be disabled")
	}
}

func TestParseConfig_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		shouldError bool
		expectedErr string
	}{
		{
			name: "invalid JSON",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [
					{
						"path": "./src",
			}`,
			shouldError: true,
			expectedErr: "failed to parse config",
		},
		{
			name: "multiple rules",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [
					{"path": "./src"},
					{"path": "./tests"}
				]
			}`,
			shouldError: false,
		},
		{
			name: "comment support",
			configJSON: `{
				// This is a comment
				"configVersion": "1.0",
				"rules": [
					{
						"path": "./src" /* inline comment */
					}
				]
			}`,
			shouldError: false,
		},
		{
			name: "empty rules array",
			configJSON: `{
				"configVersion": "1.0",
				"rules": []
			}`,
			shouldError: false,
		},
		{
			name: "empty arrays in optional fields",
			configJSON: `{
				"configVersion": "1.0",
				"conditionNames": [],
				"ignoreFiles": [],
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{
						"name": "Test",
						"pattern": "src/**",
						"allow": [],
						"deny": []
					}],
					"orphanFilesDetection": {
						"enabled": true,
						"validEntryPoints": [],
						"graphExclude": []
					}
				}]
			}`,
			shouldError: false,
		},
		{
			name: "unicode characters",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{
						"name": "测试边界",
						"pattern": "src/组件/**",
						"allow": ["src/工具/**"],
						"deny": []
					}]
				}]
			}`,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tt.configJSON))
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.name)
				} else if !contains(err.Error(), tt.expectedErr) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedErr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s, got %v", tt.name, err)
				}
			}
		})
	}
}

func TestParseConfig_MultipleErrors(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		expectedErr string
	}{
		{
			name: "multiple unknown fields",
			configJSON: `{
				"configVersion": "1.0",
				"unknownField1": "value1",
				"unknownField2": "value2",
				"rules": [{
					"path": "./src",
					"unknownField3": "value3",
					"moduleBoundaries": [{
						"name": "Test",
						"pattern": "src/**",
						"unknownField4": "value4"
					}]
				}]
			}`,
			expectedErr: "unknown field", // Should catch the first unknown field
		},
		{
			name: "mixed type and pattern errors",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": 123,
					"moduleBoundaries": [{
						"name": "Test",
						"pattern": "./invalid/**",
						"allow": [],
						"deny": []
					}]
				}]
			}`,
			expectedErr: "must be a string", // Type error should be caught first
		},
		{
			name: "multiple detection options errors",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"circularImportsDetection": {
						"enabled": "not-boolean"
					},
					"orphanFilesDetection": {
						"enabled": null
					}
				}]
			}`,
			expectedErr: "must be a boolean", // First error should be caught
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tt.configJSON))
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			} else if !contains(err.Error(), tt.expectedErr) {
				t.Errorf("Expected error containing '%s', got '%s'", tt.expectedErr, err.Error())
			}
		})
	}
}

func TestParseConfig_CommentEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		shouldError bool
	}{
		{
			name: "multiline comments",
			configJSON: `{
				/* This is a
				   multiline comment */
				"configVersion": "1.0",
				"rules": [{
					"path": "./src"
				}]
			}`,
			shouldError: false,
		},
		{
			name: "comments with special characters",
			configJSON: `{
				// Comment with @#$%^&*() characters
				"configVersion": "1.0",
				"rules": [{
					"path": "./src" /* Comment with "quotes" and 'apostrophes' */
				}]
			}`,
			shouldError: false,
		},
		{
			name: "nested comments",
			configJSON: `{
				/* Outer comment */
				"configVersion": "1.0",
				"rules": [{
					"path": "./src"
				}]
			}`,
			shouldError: false,
		},
		{
			name: "trailing commas with comments",
			configJSON: `{
				"configVersion": "1.0", // Version comment
				"rules": [{
					"path": "./src", // Path comment
				},], // Rules array comment
			}`,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tt.configJSON))
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s, got %v", tt.name, err)
				}
			}
		})
	}
}

func TestParseConfig_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		shouldError bool
	}{
		{
			name: "minimal production config",
			configJSON: `{
				"configVersion": "1.0",
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [{
						"name": "Core",
						"pattern": "src/core/**",
						"allow": ["src/utils/**"],
						"deny": ["src/ui/**"]
					}],
					"circularImportsDetection": {
						"enabled": true,
						"ignoreTypeImports": true
					}
				}]
			}`,
			shouldError: false,
		},
		{
			name: "complex monorepo config",
			configJSON: `{
				"configVersion": "1.0",
				"conditionNames": ["node", "imports", "default"],
				"ignoreFiles": ["dist/**/*", "build/**/*", "*.min.js", "coverage/**/*"],
				"rules": [
					{
						"path": "./packages/client",
						"moduleBoundaries": [
							{
								"name": "Client Components",
								"pattern": "packages/client/src/components/**",
								"allow": ["packages/client/src/hooks/**", "packages/client/src/utils/**", "packages/shared/**"],
								"deny": ["packages/server/**"]
							},
							{
								"name": "Client Hooks",
								"pattern": "packages/client/src/hooks/**",
								"allow": ["packages/client/src/utils/**", "packages/shared/**"],
								"deny": ["packages/client/src/components/**"]
							}
						],
						"circularImportsDetection": {
							"enabled": true,
							"ignoreTypeImports": true
						},
						"orphanFilesDetection": {
							"enabled": true,
							"validEntryPoints": ["packages/client/src/index.ts", "packages/client/src/App.tsx"],
							"ignoreTypeImports": false,
							"graphExclude": ["**/*.test.ts", "**/*.spec.ts", "**/*.stories.ts"]
						},
						"unusedNodeModulesDetection": {
							"enabled": true,
							"includeModules": ["@myorg/**"],
							"excludeModules": ["@types/**", "@testing-library/**"],
							"outputType": "groupByModule"
						}
					},
					{
						"path": "./packages/server",
						"moduleBoundaries": [
							{
								"name": "Server API",
								"pattern": "packages/server/src/api/**",
								"allow": ["packages/server/src/services/**", "packages/server/src/types/**", "packages/shared/**"],
								"deny": ["packages/client/**"]
							}
						],
						"circularImportsDetection": {
							"enabled": true,
							"ignoreTypeImports": false
						},
						"missingNodeModulesDetection": {
							"enabled": true,
							"includeModules": ["express/**", "lodash/**"],
							"excludeModules": ["@types/**"],
							"outputType": "groupByFile"
						}
					}
				]
			}`,
			shouldError: false,
		},
		{
			name: "all features enabled",
			configJSON: `{
				"configVersion": "1.0",
				"conditionNames": ["node", "imports", "default", "browser", "worker"],
				"ignoreFiles": ["dist/**/*", "build/**/*", "*.min.js", "coverage/**/*", "*.d.ts"],
				"rules": [{
					"path": "./src",
					"moduleBoundaries": [
						{
							"name": "UI Layer",
							"pattern": "src/ui/**",
							"allow": ["src/utils/**", "src/types/**", "src/hooks/**"],
							"deny": ["src/api/**", "src/store/**"]
						},
						{
							"name": "API Layer",
							"pattern": "src/api/**",
							"allow": ["src/types/**", "src/utils/**", "src/config/**"],
							"deny": ["src/ui/**", "src/components/**"]
						}
					],
					"circularImportsDetection": {
						"enabled": true,
						"ignoreTypeImports": true
					},
					"orphanFilesDetection": {
						"enabled": true,
						"validEntryPoints": ["src/index.ts", "src/main.tsx", "src/App.ts"],
						"ignoreTypeImports": false,
						"graphExclude": ["**/*.test.ts", "**/*.spec.ts", "**/*.stories.ts", "**/*.mock.ts"]
					},
					"unusedNodeModulesDetection": {
						"enabled": true,
						"includeModules": ["@myorg/**", "@design-system/**"],
						"excludeModules": ["@types/**", "@testing/**", "@storybook/**"],
						"pkgJsonFieldsWithBinaries": ["bin", "scripts", "devScripts"],
						"filesWithBinaries": ["scripts/**/*", "tools/**/*"],
						"filesWithModules": ["config/**/*", "webpack/**/*"],
						"outputType": "groupByModule"
					},
					"missingNodeModulesDetection": {
						"enabled": true,
						"includeModules": ["lodash/**", "axios/**", "moment/**"],
						"excludeModules": ["@types/**"],
						"outputType": "groupByFile"
					}
				}]
			}`,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig([]byte(tt.configJSON))
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s, got %v", tt.name, err)
				}
			}
		})
	}
}

func TestParseConfigWithComments(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		shouldError bool
		expected    RevDepConfig
	}{
		{
			name: "valid jsonc with line comments",
			content: `{
				// This is a comment
				"configVersion": "1.0",
				"rules": [
					{
						"path": "src/**/*",
						"circularImportsDetection": {
							"enabled": true // Enable circular import detection
						}
					}
				]
			}`,
			shouldError: false,
			expected: RevDepConfig{
				ConfigVersion: "1.0",
				Rules: []Rule{
					{
						Path: "src/**/*",
						CircularImportsDetections: []*CircularImportsOptions{{
							Enabled: true,
						}},
					},
				},
			},
		},
		{
			name: "valid jsonc with block comments",
			content: `{
				/* This is a block comment */
				"configVersion": "1.0",
				"rules": [
					{
						"path": "src/**/*",
						"orphanFilesDetection": {
							"enabled": true,
							"validEntryPoints": [
								"index.ts" /* main entry point */
							]
						}
					}
				]
			}`,
			shouldError: false,
			expected: RevDepConfig{
				ConfigVersion: "1.0",
				Rules: []Rule{
					{
						Path: "src/**/*",
						OrphanFilesDetections: []*OrphanFilesOptions{{
							Enabled:          true,
							ValidEntryPoints: []string{"index.ts"},
						}},
					},
				},
			},
		},
		{
			name: "valid jsonc with mixed comments",
			content: `{
				// Configuration file
				"configVersion": "1.0", /* version */
				"conditionNames": ["production"], // environment
				"rules": [
					{
						"path": "src/**/*",
						"moduleBoundaries": [
							{
								"name": "ui", /* UI components */
								"pattern": "src/ui/**/*",
								"allow": ["src/ui/**/*"] // allow internal imports
							}
						]
					}
				]
			}`,
			shouldError: false,
			expected: RevDepConfig{
				ConfigVersion:  "1.0",
				ConditionNames: []string{"production"},
				Rules: []Rule{
					{
						Path: "src/**/*",
						ModuleBoundaries: []BoundaryRule{
							{
								Name:    "ui",
								Pattern: "src/ui/**/*",
								Allow:   []string{"src/ui/**/*"},
							},
						},
					},
				},
			},
		},
		{
			name: "invalid jsonc syntax",
			content: `{
				"configVersion": "1.0",
				"rules": [
					{
						"path": "src/**/*"
						// Missing comma here makes it invalid JSON
						"circularImportsDetection": {
							"enabled": true
						}
					}
				]
			}`,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseConfig([]byte(tt.content))

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if config.ConfigVersion != tt.expected.ConfigVersion {
				t.Errorf("Expected configVersion %s, got %s", tt.expected.ConfigVersion, config.ConfigVersion)
			}

			if !equalStringSlices(config.ConditionNames, tt.expected.ConditionNames) {
				t.Errorf("Expected conditionNames %v, got %v", tt.expected.ConditionNames, config.ConditionNames)
			}

			if len(config.Rules) != len(tt.expected.Rules) {
				t.Errorf("Expected %d rules, got %d", len(tt.expected.Rules), len(config.Rules))
				return
			}

			// Compare first rule
			if len(config.Rules) > 0 {
				expectedRule := tt.expected.Rules[0]
				actualRule := config.Rules[0]

				if actualRule.Path != expectedRule.Path {
					t.Errorf("Expected rule path %s, got %s", expectedRule.Path, actualRule.Path)
				}
			}
		})
	}
}

// Helper function to compare string slices
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if b[i] != v {
			return false
		}
	}
	return true
}

func TestFindConfigFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "rev-dep-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name         string
		createFiles  map[string]string
		expectedFile string
		shouldError  bool
	}{
		{
			name: "hidden config only",
			createFiles: map[string]string{
				".rev-dep.config.json": `{"configVersion": "1.0", "rules": []}`,
			},
			expectedFile: ".rev-dep.config.json",
			shouldError:  false,
		},
		{
			name: "regular config only",
			createFiles: map[string]string{
				"rev-dep.config.json": `{"configVersion": "1.0", "rules": []}`,
			},
			expectedFile: "rev-dep.config.json",
			shouldError:  false,
		},
		{
			name: "hidden jsonc config only",
			createFiles: map[string]string{
				".rev-dep.config.jsonc": `{"configVersion": "1.0", "rules": []}`,
			},
			expectedFile: ".rev-dep.config.jsonc",
			shouldError:  false,
		},
		{
			name: "regular jsonc config only",
			createFiles: map[string]string{
				"rev-dep.config.jsonc": `{"configVersion": "1.0", "rules": []}`,
			},
			expectedFile: "rev-dep.config.jsonc",
			shouldError:  false,
		},
		{
			name: "both configs present (should error)",
			createFiles: map[string]string{
				".rev-dep.config.json": `{"configVersion": "1.0", "rules": []}`,
				"rev-dep.config.json":  `{"configVersion": "2.0.0", "rules": []}`,
			},
			expectedFile: "",
			shouldError:  true,
		},
		{
			name: "both jsonc configs present (should error)",
			createFiles: map[string]string{
				".rev-dep.config.jsonc": `{"configVersion": "1.0", "rules": []}`,
				"rev-dep.config.jsonc":  `{"configVersion": "2.0.0", "rules": []}`,
			},
			expectedFile: "",
			shouldError:  true,
		},
		{
			name: "mixed json and jsonc configs present (should error)",
			createFiles: map[string]string{
				".rev-dep.config.json": `{"configVersion": "1.0", "rules": []}`,
				"rev-dep.config.jsonc": `{"configVersion": "2.0.0", "rules": []}`,
			},
			expectedFile: "",
			shouldError:  true,
		},
		{
			name: "all four config variants present (should error)",
			createFiles: map[string]string{
				".rev-dep.config.json":  `{"configVersion": "1.0", "rules": []}`,
				"rev-dep.config.json":   `{"configVersion": "2.0.0", "rules": []}`,
				".rev-dep.config.jsonc": `{"configVersion": "3.0.0", "rules": []}`,
				"rev-dep.config.jsonc":  `{"configVersion": "4.0.0", "rules": []}`,
			},
			expectedFile: "",
			shouldError:  true,
		},
		{
			name:         "no config files",
			createFiles:  map[string]string{},
			expectedFile: "",
			shouldError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing files first
			for fileName := range tt.createFiles {
				os.Remove(filepath.Join(tempDir, fileName))
			}
			// Also remove the other config files if they're not in this test
			configFiles := []string{".rev-dep.config.json", "rev-dep.config.json", ".rev-dep.config.jsonc", "rev-dep.config.jsonc"}
			for _, configFile := range configFiles {
				if tt.createFiles[configFile] == "" {
					os.Remove(filepath.Join(tempDir, configFile))
				}
			}

			// Create test files
			for fileName, content := range tt.createFiles {
				filePath := filepath.Join(tempDir, fileName)
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create file %s: %v", fileName, err)
				}
			}

			// Test findConfigFile
			foundPath, err := findConfigFile(tempDir)
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s, got %v", tt.name, err)
				}
				expectedPath := filepath.Join(tempDir, tt.expectedFile)
				if foundPath != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, foundPath)
				}
			}

			// Clean up files after test
			for fileName := range tt.createFiles {
				os.Remove(filepath.Join(tempDir, fileName))
			}
		})
	}
}

func TestValidateRulePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid relative path",
			path:    "apps/web",
			wantErr: false,
		},
		{
			name:    "valid relative path with dot",
			path:    "./apps/web",
			wantErr: false,
		},
		{
			name:    "valid root path",
			path:    ".",
			wantErr: false,
		},
		{
			name:    "valid root path with dot slash",
			path:    "./",
			wantErr: false,
		},
		{
			name:    "invalid path with parent directory",
			path:    "../apps/web",
			wantErr: true,
			errMsg:  "contains '../'",
		},
		{
			name:    "invalid empty path",
			path:    "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "invalid path with parent directory in middle",
			path:    "apps/../web",
			wantErr: true,
			errMsg:  "contains '../'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRulePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRulePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("validateRulePath() error = %v, expected to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestNormalizeRulePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "relative path without dot",
			path:     "apps/web",
			expected: "apps/web",
		},
		{
			name:     "relative path with dot",
			path:     "./apps/web",
			expected: "apps/web",
		},
		{
			name:     "root path",
			path:     ".",
			expected: ".",
		},
		{
			name:     "root path with dot slash",
			path:     "./",
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeRulePath(tt.path)
			if result != tt.expected {
				t.Errorf("normalizeRulePath() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestParseConfig_UnusedExportsDetection(t *testing.T) {
	t.Run("valid config with all options", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": true,
					"validEntryPoints": ["src/index.ts", "src/public-api.ts"],
					"ignoreTypeExports": true,
					"graphExclude": ["**/*.test.ts", "**/*.spec.ts"],
					"ignore": {
						"src/types.ts": "B*"
					},
					"ignoreFiles": ["**/*.generated.ts"],
					"ignoreExports": ["default", "unused*"],
					"autofix": true
				}
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		rule := config.Rules[0]
		if firstDetectionOrNil(rule.UnusedExportsDetections) == nil {
			t.Fatal("Expected unusedExportsDetection to be non-nil")
		}
		if !firstDetectionOrNil(rule.UnusedExportsDetections).Enabled {
			t.Error("Expected enabled to be true")
		}
		if len(firstDetectionOrNil(rule.UnusedExportsDetections).ValidEntryPoints) != 2 {
			t.Errorf("Expected 2 entry points, got %d", len(firstDetectionOrNil(rule.UnusedExportsDetections).ValidEntryPoints))
		}
		if !firstDetectionOrNil(rule.UnusedExportsDetections).IgnoreTypeExports {
			t.Error("Expected ignoreTypeExports to be true")
		}
		if len(firstDetectionOrNil(rule.UnusedExportsDetections).GraphExclude) != 2 {
			t.Errorf("Expected 2 graphExclude patterns, got %d", len(firstDetectionOrNil(rule.UnusedExportsDetections).GraphExclude))
		}
		if got := firstDetectionOrNil(rule.UnusedExportsDetections).Ignore["src/types.ts"]; len(got) != 1 || got[0] != "B*" {
			t.Errorf("Expected normalized ignore entry for src/types.ts, got %#v", got)
		}
		if len(firstDetectionOrNil(rule.UnusedExportsDetections).IgnoreFiles) != 1 {
			t.Errorf("Expected 1 ignoreFiles pattern, got %d", len(firstDetectionOrNil(rule.UnusedExportsDetections).IgnoreFiles))
		}
		if len(firstDetectionOrNil(rule.UnusedExportsDetections).IgnoreExports) != 2 {
			t.Errorf("Expected 2 ignoreExports entries, got %d", len(firstDetectionOrNil(rule.UnusedExportsDetections).IgnoreExports))
		}
		if !firstDetectionOrNil(rule.UnusedExportsDetections).Autofix {
			t.Error("Expected autofix to be true")
		}
	})

	t.Run("minimal config", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": true
				}
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		rule := config.Rules[0]
		if firstDetectionOrNil(rule.UnusedExportsDetections) == nil || !firstDetectionOrNil(rule.UnusedExportsDetections).Enabled {
			t.Error("Expected unusedExportsDetection to be enabled")
		}
	})

	t.Run("missing enabled field", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "unusedExportsDetection.enabled is required") {
			t.Errorf("Expected 'enabled is required' error, got: %s", err.Error())
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": true,
					"unknownField": "value"
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "unknown field 'unknownField'") {
			t.Errorf("Expected unknown field error, got: %s", err.Error())
		}
	})

	t.Run("wrong type for enabled", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": "yes"
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "must be a boolean") {
			t.Errorf("Expected boolean type error, got: %s", err.Error())
		}
	})

	t.Run("wrong type for validEntryPoints", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": true,
					"validEntryPoints": "not-an-array"
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "must be an array") {
			t.Errorf("Expected array type error, got: %s", err.Error())
		}
	})

	t.Run("wrong type for ignoreTypeExports", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": true,
					"ignoreTypeExports": "yes"
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "must be a boolean") {
			t.Errorf("Expected boolean type error, got: %s", err.Error())
		}
	})

	t.Run("invalid graphExclude pattern", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": true,
					"graphExclude": ["./invalid/**"]
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "starts with './'") {
			t.Errorf("Expected pattern validation error, got: %s", err.Error())
		}
	})

	t.Run("empty validEntryPoints string", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": true,
					"validEntryPoints": [""]
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "cannot be empty") {
			t.Errorf("Expected empty entry point error, got: %s", err.Error())
		}
	})

	t.Run("null enabled", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": null
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "cannot be null") {
			t.Errorf("Expected null error, got: %s", err.Error())
		}
	})

	t.Run("not an object", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": "not-object"
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "must be an object") {
			t.Errorf("Expected object type error, got: %s", err.Error())
		}
	})

	t.Run("autofix validation", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": true,
					"autofix": "yes"
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "autofix must be a boolean") {
			t.Errorf("Expected autofix boolean type error, got: %s", err.Error())
		}
	})

	t.Run("wrong type for ignore", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": true,
					"ignore": "not-an-object"
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "ignore must be an object") {
			t.Errorf("Expected ignore object type error, got: %s", err.Error())
		}
	})

	t.Run("wrong type for ignoreFiles", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": true,
					"ignoreFiles": "not-an-array"
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "ignoreFiles must be an array") {
			t.Errorf("Expected ignoreFiles array type error, got: %s", err.Error())
		}
	})

	t.Run("wrong type for ignoreExports", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": true,
					"ignoreExports": "not-an-array"
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "ignoreExports must be an array") {
			t.Errorf("Expected ignoreExports array type error, got: %s", err.Error())
		}
	})

	t.Run("invalid ignoreFiles pattern", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": "./src",
				"unusedExportsDetection": {
					"enabled": true,
					"ignoreFiles": ["./invalid/**"]
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "starts with './'") {
			t.Errorf("Expected pattern validation error, got: %s", err.Error())
		}
	})
}

func TestParseConfig_UnresolvedImportsDetection(t *testing.T) {
	t.Run("valid config with all options", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": ".",
				"unresolvedImportsDetection": {
					"enabled": true,
					"ignore": {
						"./src/index.ts": "non-existent-module"
					},
					"ignoreFiles": ["**/broken-import.ts"],
					"ignoreImports": ["non-existent-pkg"]
				}
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		rule := config.Rules[0]
		if firstDetectionOrNil(rule.UnresolvedImportsDetections) == nil {
			t.Fatal("Expected unresolvedImportsDetection to be non-nil")
		}
		if !firstDetectionOrNil(rule.UnresolvedImportsDetections).Enabled {
			t.Error("Expected enabled to be true")
		}

		if got := firstDetectionOrNil(rule.UnresolvedImportsDetections).Ignore["src/index.ts"]; len(got) != 1 || got[0] != "non-existent-module" {
			t.Errorf("Expected normalized ignore entry for src/index.ts, got %#v", got)
		}
		if len(firstDetectionOrNil(rule.UnresolvedImportsDetections).IgnoreFiles) != 1 {
			t.Errorf("Expected 1 ignoreFiles pattern, got %d", len(firstDetectionOrNil(rule.UnresolvedImportsDetections).IgnoreFiles))
		}
		if len(firstDetectionOrNil(rule.UnresolvedImportsDetections).IgnoreImports) != 1 {
			t.Errorf("Expected 1 ignoreImports entry, got %d", len(firstDetectionOrNil(rule.UnresolvedImportsDetections).IgnoreImports))
		}
	})

	t.Run("missing enabled field", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": ".",
				"unresolvedImportsDetection": {}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "unresolvedImportsDetection.enabled is required") {
			t.Errorf("Expected enabled-is-required error, got: %s", err.Error())
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": ".",
				"unresolvedImportsDetection": {
					"enabled": true,
					"unknownField": true
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "unknown field 'unknownField'") {
			t.Errorf("Expected unknown field error, got: %s", err.Error())
		}
	})

	t.Run("wrong type for ignore", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": ".",
				"unresolvedImportsDetection": {
					"enabled": true,
					"ignore": "not-an-object"
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "ignore must be an object") {
			t.Errorf("Expected object type error, got: %s", err.Error())
		}
	})

	t.Run("wrong type for ignoreFiles", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": ".",
				"unresolvedImportsDetection": {
					"enabled": true,
					"ignoreFiles": "not-an-array"
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "ignoreFiles must be an array") {
			t.Errorf("Expected array type error, got: %s", err.Error())
		}
	})

	t.Run("wrong type for ignoreImports", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": ".",
				"unresolvedImportsDetection": {
					"enabled": true,
					"ignoreImports": "not-an-array"
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "ignoreImports must be an array") {
			t.Errorf("Expected array type error, got: %s", err.Error())
		}
	})

	t.Run("invalid ignoreFiles pattern", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": ".",
				"unresolvedImportsDetection": {
					"enabled": true,
					"ignoreFiles": ["./invalid/**"]
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "starts with './'") {
			t.Errorf("Expected pattern validation error, got: %s", err.Error())
		}
	})

	t.Run("empty ignore import request in map", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": ".",
				"unresolvedImportsDetection": {
					"enabled": true,
					"ignore": {
						"src/index.ts": ""
					}
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "cannot be empty") {
			t.Errorf("Expected empty value error, got: %s", err.Error())
		}
	})

	t.Run("ignore map key cannot traverse parent dirs", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.3",
			"rules": [{
				"path": ".",
				"unresolvedImportsDetection": {
					"enabled": true,
					"ignore": {
						"../src/index.ts": "non-existent-module"
					}
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !contains(err.Error(), "must not traverse parent directories") {
			t.Errorf("Expected parent directory traversal error, got: %s", err.Error())
		}
	})
}

func TestParseConfig_FollowMonorepoPackages(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expected    model.FollowMonorepoPackagesValue
		expectedErr string
	}{
		{
			name: "followMonorepoPackages not set should default to true",
			config: `{
				"configVersion": "1.0",
				"rules": [
					{
						"path": "./src"
					}
				]
			}`,
			expected: model.FollowMonorepoPackagesValue{FollowAll: true},
		},
		{
			name: "followMonorepoPackages explicitly set to true",
			config: `{
				"configVersion": "1.0",
				"rules": [
					{
						"path": "./src",
						"followMonorepoPackages": true
					}
				]
			}`,
			expected: model.FollowMonorepoPackagesValue{FollowAll: true},
		},
		{
			name: "followMonorepoPackages explicitly set to false",
			config: `{
				"configVersion": "1.0",
				"rules": [
					{
						"path": "./src",
						"followMonorepoPackages": false
					}
				]
			}`,
			expected: model.FollowMonorepoPackagesValue{},
		},
		{
			name: "followMonorepoPackages selective package list",
			config: `{
				"configVersion": "1.0",
				"rules": [
					{
						"path": "./src",
						"followMonorepoPackages": ["pkg-a", "@scope/*"]
					}
				]
			}`,
			expected: model.FollowMonorepoPackagesValue{Packages: map[string]bool{"pkg-a": true, "@scope/*": true}},
		},
		{
			name: "followMonorepoPackages rejects mixed array",
			config: `{
				"configVersion": "1.0",
				"rules": [
					{
						"path": "./src",
						"followMonorepoPackages": ["pkg-a", 1]
					}
				]
			}`,
			expectedErr: "followMonorepoPackages must be a boolean or array of strings",
		},
		{
			name: "followMonorepoPackages rejects empty array",
			config: `{
				"configVersion": "1.0",
				"rules": [
					{
						"path": "./src",
						"followMonorepoPackages": []
					}
				]
			}`,
			expectedErr: "array cannot be empty",
		},
		{
			name: "followMonorepoPackages rejects invalid type object",
			config: `{
				"configVersion": "1.0",
				"rules": [
					{
						"path": "./src",
						"followMonorepoPackages": {}
					}
				]
			}`,
			expectedErr: "must be a boolean or array of strings",
		},
		{
			name: "followMonorepoPackages rejects invalid type string",
			config: `{
				"configVersion": "1.0",
				"rules": [
					{
						"path": "./src",
						"followMonorepoPackages": "yes"
					}
				]
			}`,
			expectedErr: "must be a boolean or array of strings",
		},
		{
			name: "followMonorepoPackages rejects empty string entry",
			config: `{
				"configVersion": "1.0",
				"rules": [
					{
						"path": "./src",
						"followMonorepoPackages": ["pkg-a", " "]
					}
				]
			}`,
			expectedErr: "cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseConfig([]byte(tt.config))
			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q, got nil", tt.expectedErr)
				}
				if !contains(err.Error(), tt.expectedErr) {
					t.Fatalf("Expected error containing %q, got %q", tt.expectedErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
				return
			}

			if len(config.Rules) != 1 {
				t.Errorf("Expected 1 rule, got %d", len(config.Rules))
				return
			}

			actual := config.Rules[0].FollowMonorepoPackages
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("Expected FollowMonorepoPackages to be %v, got %v", tt.expected, actual)
			}
		})
	}

	// Separate test for multiple rules with different values
	t.Run("multiple rules with different followMonorepoPackages values", func(t *testing.T) {
		config := `{
			"configVersion": "1.0",
			"rules": [
				{
					"path": "./src",
					"followMonorepoPackages": false
				},
				{
					"path": "./lib"
				},
				{
					"path": "./tests",
					"followMonorepoPackages": true
				}
			]
		}`

		parsedConfig, err := ParseConfig([]byte(config))
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
			return
		}

		if len(parsedConfig.Rules) != 3 {
			t.Errorf("Expected 3 rules, got %d", len(parsedConfig.Rules))
			return
		}

		// First rule should be explicitly false
		if !reflect.DeepEqual(parsedConfig.Rules[0].FollowMonorepoPackages, model.FollowMonorepoPackagesValue{}) {
			t.Errorf("Expected first rule FollowMonorepoPackages to be false, got %v", parsedConfig.Rules[0].FollowMonorepoPackages)
		}

		// Second rule should default to true
		if !reflect.DeepEqual(parsedConfig.Rules[1].FollowMonorepoPackages, model.FollowMonorepoPackagesValue{FollowAll: true}) {
			t.Errorf("Expected second rule FollowMonorepoPackages to be true (default), got %v", parsedConfig.Rules[1].FollowMonorepoPackages)
		}

		// Third rule should be explicitly true
		if !reflect.DeepEqual(parsedConfig.Rules[2].FollowMonorepoPackages, model.FollowMonorepoPackagesValue{FollowAll: true}) {
			t.Errorf("Expected third rule FollowMonorepoPackages to be true, got %v", parsedConfig.Rules[2].FollowMonorepoPackages)
		}
	})
}

func TestParseConfig_RuleLevelEntryPointsInheritance(t *testing.T) {
	t.Run("inherits merged valid entry points and prod-only entry points", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"rules": [{
				"path": ".",
				"prodEntryPoints": ["src/prod.ts", "src/shared.ts"],
				"devEntryPoints": ["src/dev.ts", "src/shared.ts"],
				"orphanFilesDetection": {
					"enabled": true
				},
				"unusedExportsDetection": {
					"enabled": true
				},
				"devDepsUsageOnProdDetection": {
					"enabled": true
				}
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		rule := config.Rules[0]
		expectedMerged := []string{"src/prod.ts", "src/shared.ts", "src/dev.ts"}
		if !reflect.DeepEqual(firstDetectionOrNil(rule.OrphanFilesDetections).ValidEntryPoints, expectedMerged) {
			t.Fatalf("unexpected orphanFilesDetection.validEntryPoints: %+v", firstDetectionOrNil(rule.OrphanFilesDetections).ValidEntryPoints)
		}
		if !reflect.DeepEqual(firstDetectionOrNil(rule.UnusedExportsDetections).ValidEntryPoints, expectedMerged) {
			t.Fatalf("unexpected unusedExportsDetection.validEntryPoints: %+v", firstDetectionOrNil(rule.UnusedExportsDetections).ValidEntryPoints)
		}
		if !reflect.DeepEqual(firstDetectionOrNil(rule.DevDepsUsageOnProdDetections).ProdEntryPoints, []string{"src/prod.ts", "src/shared.ts"}) {
			t.Fatalf("unexpected devDepsUsageOnProdDetection.prodEntryPoints: %+v", firstDetectionOrNil(rule.DevDepsUsageOnProdDetections).ProdEntryPoints)
		}
	})

	t.Run("detector-level arrays override inherited values", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"rules": [{
				"path": ".",
				"prodEntryPoints": ["src/prod.ts"],
				"devEntryPoints": ["src/dev.ts"],
				"orphanFilesDetection": {
					"enabled": true,
					"validEntryPoints": ["src/custom-orphan.ts"]
				},
				"unusedExportsDetection": {
					"enabled": true,
					"validEntryPoints": ["src/custom-unused.ts"]
				},
				"devDepsUsageOnProdDetection": {
					"enabled": true,
					"prodEntryPoints": ["src/custom-prod.ts"]
				}
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		rule := config.Rules[0]
		if !reflect.DeepEqual(firstDetectionOrNil(rule.OrphanFilesDetections).ValidEntryPoints, []string{"src/custom-orphan.ts"}) {
			t.Fatalf("unexpected orphan override: %+v", firstDetectionOrNil(rule.OrphanFilesDetections).ValidEntryPoints)
		}
		if !reflect.DeepEqual(firstDetectionOrNil(rule.UnusedExportsDetections).ValidEntryPoints, []string{"src/custom-unused.ts"}) {
			t.Fatalf("unexpected unused exports override: %+v", firstDetectionOrNil(rule.UnusedExportsDetections).ValidEntryPoints)
		}
		if !reflect.DeepEqual(firstDetectionOrNil(rule.DevDepsUsageOnProdDetections).ProdEntryPoints, []string{"src/custom-prod.ts"}) {
			t.Fatalf("unexpected prod override: %+v", firstDetectionOrNil(rule.DevDepsUsageOnProdDetections).ProdEntryPoints)
		}
	})

	t.Run("explicit empty detector arrays override and disable inheritance", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"rules": [{
				"path": ".",
				"prodEntryPoints": ["src/prod.ts"],
				"devEntryPoints": ["src/dev.ts"],
				"orphanFilesDetection": {
					"enabled": true,
					"validEntryPoints": []
				},
				"unusedExportsDetection": {
					"enabled": true,
					"validEntryPoints": []
				},
				"devDepsUsageOnProdDetection": {
					"enabled": true,
					"prodEntryPoints": []
				}
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		rule := config.Rules[0]
		if len(firstDetectionOrNil(rule.OrphanFilesDetections).ValidEntryPoints) != 0 {
			t.Fatalf("expected empty orphan validEntryPoints, got %+v", firstDetectionOrNil(rule.OrphanFilesDetections).ValidEntryPoints)
		}
		if len(firstDetectionOrNil(rule.UnusedExportsDetections).ValidEntryPoints) != 0 {
			t.Fatalf("expected empty unusedExports validEntryPoints, got %+v", firstDetectionOrNil(rule.UnusedExportsDetections).ValidEntryPoints)
		}
		if len(firstDetectionOrNil(rule.DevDepsUsageOnProdDetections).ProdEntryPoints) != 0 {
			t.Fatalf("expected empty prodEntryPoints, got %+v", firstDetectionOrNil(rule.DevDepsUsageOnProdDetections).ProdEntryPoints)
		}
	})

	t.Run("detector-level null values fallback to rule-level defaults", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"rules": [{
				"path": ".",
				"prodEntryPoints": ["src/prod.ts"],
				"devEntryPoints": ["src/dev.ts"],
				"orphanFilesDetection": {
					"enabled": true,
					"validEntryPoints": null
				},
				"unusedExportsDetection": {
					"enabled": true,
					"validEntryPoints": null
				},
				"devDepsUsageOnProdDetection": {
					"enabled": true,
					"prodEntryPoints": null
				}
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		rule := config.Rules[0]
		expectedMerged := []string{"src/prod.ts", "src/dev.ts"}
		if !reflect.DeepEqual(firstDetectionOrNil(rule.OrphanFilesDetections).ValidEntryPoints, expectedMerged) {
			t.Fatalf("unexpected orphan fallback: %+v", firstDetectionOrNil(rule.OrphanFilesDetections).ValidEntryPoints)
		}
		if !reflect.DeepEqual(firstDetectionOrNil(rule.UnusedExportsDetections).ValidEntryPoints, expectedMerged) {
			t.Fatalf("unexpected unused exports fallback: %+v", firstDetectionOrNil(rule.UnusedExportsDetections).ValidEntryPoints)
		}
		if !reflect.DeepEqual(firstDetectionOrNil(rule.DevDepsUsageOnProdDetections).ProdEntryPoints, []string{"src/prod.ts"}) {
			t.Fatalf("unexpected prod fallback: %+v", firstDetectionOrNil(rule.DevDepsUsageOnProdDetections).ProdEntryPoints)
		}
	})
}

func TestParseConfig_RuleLevelEntryPointsValidation(t *testing.T) {
	t.Run("invalid type for rule-level entry points", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"rules": [{
				"path": ".",
				"prodEntryPoints": "src/main.ts"
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "rules[0].prodEntryPoints must be an array") {
			t.Fatalf("expected prodEntryPoints type error, got: %v", err)
		}
	})

	t.Run("empty string in rule-level entry points", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"rules": [{
				"path": ".",
				"devEntryPoints": [""]
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "rules[0].devEntryPoints[0]: cannot be empty") {
			t.Fatalf("expected devEntryPoints validation error, got: %v", err)
		}
	})
}

func TestParseConfig_IgnoreMapSupportsStringOrArrayValues(t *testing.T) {
	t.Run("unused exports ignore supports array values", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"rules": [{
				"path": ".",
				"unusedExportsDetection": {
					"enabled": true,
					"ignore": {
						"src/types.ts": ["B*", "F*"]
					}
				}
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		got := firstDetectionOrNil(config.Rules[0].UnusedExportsDetections).Ignore["src/types.ts"]
		if len(got) != 2 || got[0] != "B*" || got[1] != "F*" {
			t.Fatalf("unexpected normalized ignore for unused exports: %#v", got)
		}
	})

	t.Run("unresolved imports ignore supports array values", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"rules": [{
				"path": ".",
				"unresolvedImportsDetection": {
					"enabled": true,
					"ignore": {
						"src/index.ts": ["non-existent-*", "missing-*"]
					}
				}
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		got := firstDetectionOrNil(config.Rules[0].UnresolvedImportsDetections).Ignore["src/index.ts"]
		if len(got) != 2 || got[0] != "non-existent-*" || got[1] != "missing-*" {
			t.Fatalf("unexpected normalized ignore for unresolved imports: %#v", got)
		}
	})
}

func TestParseConfig_DetectorsCanBeArrays(t *testing.T) {
	configJSON := `{
		"configVersion": "1.6",
		"rules": [{
			"path": ".",
			"circularImportsDetection": [
				{"enabled": true, "ignoreTypeImports": true},
				{"enabled": false}
			],
			"orphanFilesDetection": [
				{"enabled": true, "validEntryPoints": ["src/index.ts"]},
				{"enabled": true, "validEntryPoints": []}
			],
			"unusedNodeModulesDetection": [
				{"enabled": true, "outputType": "list"},
				{"enabled": true, "outputType": "groupByModule"}
			],
			"missingNodeModulesDetection": [
				{"enabled": true, "outputType": "list"},
				{"enabled": true, "outputType": "groupByFile"}
			],
			"unusedExportsDetection": [
				{"enabled": true, "validEntryPoints": ["src/index.ts"]},
				{"enabled": false}
			],
			"unresolvedImportsDetection": [
				{"enabled": true},
				{"enabled": false}
			],
			"devDepsUsageOnProdDetection": [
				{"enabled": true, "prodEntryPoints": ["src/main.ts"]},
				{"enabled": false}
			],
			"restrictedImportsDetection": [
				{"enabled": true, "entryPoints": ["src/main.ts"], "denyFiles": ["**/*.tsx"]},
				{"enabled": false, "entryPoints": ["src/other.ts"], "denyModules": ["react"]}
			]
		}]
	}`

	config, err := ParseConfig([]byte(configJSON))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	rule := config.Rules[0]
	if len(rule.CircularImportsDetections) != 2 {
		t.Fatalf("expected 2 circularImportsDetection entries, got %d", len(rule.CircularImportsDetections))
	}
	if len(rule.OrphanFilesDetections) != 2 {
		t.Fatalf("expected 2 orphanFilesDetection entries, got %d", len(rule.OrphanFilesDetections))
	}
	if len(rule.UnusedNodeModulesDetections) != 2 {
		t.Fatalf("expected 2 unusedNodeModulesDetection entries, got %d", len(rule.UnusedNodeModulesDetections))
	}
	if len(rule.MissingNodeModulesDetections) != 2 {
		t.Fatalf("expected 2 missingNodeModulesDetection entries, got %d", len(rule.MissingNodeModulesDetections))
	}
	if len(rule.UnusedExportsDetections) != 2 {
		t.Fatalf("expected 2 unusedExportsDetection entries, got %d", len(rule.UnusedExportsDetections))
	}
	if len(rule.UnresolvedImportsDetections) != 2 {
		t.Fatalf("expected 2 unresolvedImportsDetection entries, got %d", len(rule.UnresolvedImportsDetections))
	}
	if len(rule.DevDepsUsageOnProdDetections) != 2 {
		t.Fatalf("expected 2 devDepsUsageOnProdDetection entries, got %d", len(rule.DevDepsUsageOnProdDetections))
	}
	if len(rule.RestrictedImportsDetections) != 2 {
		t.Fatalf("expected 2 restrictedImportsDetection entries, got %d", len(rule.RestrictedImportsDetections))
	}

	// Compatibility: single pointer fields still expose first array item.
	if firstDetectionOrNil(rule.CircularImportsDetections) == nil || !firstDetectionOrNil(rule.CircularImportsDetections).IgnoreTypeImports {
		t.Fatalf("expected CircularImportsDetection pointer to point to first array item")
	}
	if firstDetectionOrNil(rule.RestrictedImportsDetections) == nil || len(firstDetectionOrNil(rule.RestrictedImportsDetections).EntryPoints) != 1 {
		t.Fatalf("expected RestrictedImportsDetection pointer to point to first array item")
	}
}
