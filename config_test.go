package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfig_ValidMinimalConfig(t *testing.T) {
	configJSON := `{
		"configVersion": "1.0.0",
		"rules": [
			{
				"path": "./src"
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
	if config.ConfigVersion != "1.0.0" {
		t.Errorf("Expected configVersion '1.0.0', got '%s'", config.ConfigVersion)
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
		"configVersion": "1.0.0",
		"conditionNames": ["node", "imports"],
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

	configs, err := ParseConfig([]byte(configJSON))
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	config := configs[0]

	// Check global settings
	if len(config.ConditionNames) != 2 || config.ConditionNames[0] != "node" || config.ConditionNames[1] != "imports" {
		t.Errorf("Unexpected conditionNames: %v", config.ConditionNames)
	}

	if len(config.IgnoreFiles) != 2 || config.IgnoreFiles[0] != "dist/**/*" || config.IgnoreFiles[1] != "build/**/*" {
		t.Errorf("Unexpected ignoreFiles: %v", config.IgnoreFiles)
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
	if rule.CircularImportsDetection == nil || !rule.CircularImportsDetection.Enabled || !rule.CircularImportsDetection.IgnoreTypeImports {
		t.Errorf("Circular imports detection not properly configured")
	}

	if rule.OrphanFilesDetection == nil || !rule.OrphanFilesDetection.Enabled || rule.OrphanFilesDetection.IgnoreTypeImports {
		t.Errorf("Orphan files detection not properly configured")
	}

	if rule.UnusedNodeModulesDetection == nil || !rule.UnusedNodeModulesDetection.Enabled || rule.UnusedNodeModulesDetection.OutputType != "groupByModule" {
		t.Errorf("Unused node modules detection not properly configured")
	}

	if rule.MissingNodeModulesDetection == nil || !rule.MissingNodeModulesDetection.Enabled || rule.MissingNodeModulesDetection.OutputType != "groupByFile" {
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
				"configVersion": "1.0.0",
				"rules": [{}]
			}`,
			expectedErr: "rules[0].path is required",
		},
		{
			name: "missing boundary name",
			configJSON: `{
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
				"unknownField": "value",
				"rules": [{"path": "./src"}]
			}`,
			expectedErr: "unknown field 'unknownField' in config root",
		},
		{
			name: "unknown rule field",
			configJSON: `{
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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

func TestParseConfig_InvalidTypes(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		expectedErr string
	}{
		{
			name: "rules not array",
			configJSON: `{
				"configVersion": "1.0.0",
				"rules": {}
			}`,
			expectedErr: "rules must be an array",
		},
		{
			name: "rule not object",
			configJSON: `{
				"configVersion": "1.0.0",
				"rules": ["invalid"]
			}`,
			expectedErr: "rules[0] must be an object",
		},
		{
			name: "rule path not string",
			configJSON: `{
				"configVersion": "1.0.0",
				"rules": [{"path": 123}]
			}`,
			expectedErr: "rules[0].path must be a string",
		},
		{
			name: "rule path null",
			configJSON: `{
				"configVersion": "1.0.0",
				"rules": [{"path": null}]
			}`,
			expectedErr: "rules[0].path must be a string",
		},
		{
			name: "moduleBoundaries not array",
			configJSON: `{
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
		"configVersion": "1.0.0",
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
	configs, err := ParseConfig([]byte(configJSON))
	if err != nil {
		t.Errorf("Expected no error for disabled options, got %v", err)
	}

	config := configs[0]
	rule := config.Rules[0]

	// Verify options are parsed but validation is skipped when disabled
	if rule.CircularImportsDetection == nil || rule.CircularImportsDetection.Enabled {
		t.Error("Circular imports detection should be disabled")
	}

	if rule.OrphanFilesDetection == nil || rule.OrphanFilesDetection.Enabled {
		t.Error("Orphan files detection should be disabled")
	}

	if rule.UnusedNodeModulesDetection == nil || rule.UnusedNodeModulesDetection.Enabled {
		t.Error("Unused node modules detection should be disabled")
	}

	if rule.MissingNodeModulesDetection == nil || rule.MissingNodeModulesDetection.Enabled {
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
				"rules": []
			}`,
			shouldError: false,
		},
		{
			name: "empty arrays in optional fields",
			configJSON: `{
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
				"rules": [{
					"path": "./src"
				}]
			}`,
			shouldError: false,
		},
		{
			name: "trailing commas with comments",
			configJSON: `{
				"configVersion": "1.0.0", // Version comment
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				"configVersion": "1.0.0",
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
				ConfigVersion: "1.0.0",
				Rules: []Rule{
					{
						Path: "src/**/*",
						CircularImportsDetection: &CircularImportsOptions{
							Enabled: true,
						},
					},
				},
			},
		},
		{
			name: "valid jsonc with block comments",
			content: `{
				/* This is a block comment */
				"configVersion": "1.0.0",
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
				ConfigVersion: "1.0.0",
				Rules: []Rule{
					{
						Path: "src/**/*",
						OrphanFilesDetection: &OrphanFilesOptions{
							Enabled:          true,
							ValidEntryPoints: []string{"index.ts"},
						},
					},
				},
			},
		},
		{
			name: "valid jsonc with mixed comments",
			content: `{
				// Configuration file
				"configVersion": "1.0.0", /* version */
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
				ConfigVersion:  "1.0.0",
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
				"configVersion": "1.0.0",
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
			configs, err := ParseConfig([]byte(tt.content))

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

			if len(configs) != 1 {
				t.Errorf("Expected 1 config, got %d", len(configs))
				return
			}

			config := configs[0]
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
				".rev-dep.config.json": `{"configVersion": "1.0.0", "rules": []}`,
			},
			expectedFile: ".rev-dep.config.json",
			shouldError:  false,
		},
		{
			name: "regular config only",
			createFiles: map[string]string{
				"rev-dep.config.json": `{"configVersion": "1.0.0", "rules": []}`,
			},
			expectedFile: "rev-dep.config.json",
			shouldError:  false,
		},
		{
			name: "hidden jsonc config only",
			createFiles: map[string]string{
				".rev-dep.config.jsonc": `{"configVersion": "1.0.0", "rules": []}`,
			},
			expectedFile: ".rev-dep.config.jsonc",
			shouldError:  false,
		},
		{
			name: "regular jsonc config only",
			createFiles: map[string]string{
				"rev-dep.config.jsonc": `{"configVersion": "1.0.0", "rules": []}`,
			},
			expectedFile: "rev-dep.config.jsonc",
			shouldError:  false,
		},
		{
			name: "both configs present (should error)",
			createFiles: map[string]string{
				".rev-dep.config.json": `{"configVersion": "1.0.0", "rules": []}`,
				"rev-dep.config.json":  `{"configVersion": "2.0.0", "rules": []}`,
			},
			expectedFile: "",
			shouldError:  true,
		},
		{
			name: "both jsonc configs present (should error)",
			createFiles: map[string]string{
				".rev-dep.config.jsonc": `{"configVersion": "1.0.0", "rules": []}`,
				"rev-dep.config.jsonc":  `{"configVersion": "2.0.0", "rules": []}`,
			},
			expectedFile: "",
			shouldError:  true,
		},
		{
			name: "mixed json and jsonc configs present (should error)",
			createFiles: map[string]string{
				".rev-dep.config.json": `{"configVersion": "1.0.0", "rules": []}`,
				"rev-dep.config.jsonc": `{"configVersion": "2.0.0", "rules": []}`,
			},
			expectedFile: "",
			shouldError:  true,
		},
		{
			name: "all four config variants present (should error)",
			createFiles: map[string]string{
				".rev-dep.config.json":  `{"configVersion": "1.0.0", "rules": []}`,
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
