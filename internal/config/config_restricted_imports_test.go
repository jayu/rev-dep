package config

import "testing"

func TestParseConfig_RestrictedImportsDetection(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.5",
			"rules": [{
				"path": ".",
				"restrictedImportsDetection": {
					"enabled": true,
					"entryPoints": ["src/server.ts"],
					"graphExclude": ["**/*.spec.ts"],
					"denyFiles": ["**/*.tsx"],
					"denyModules": ["react", "react-*"],
					"ignoreMatches": ["src/server/allowed.tsx"],
					"ignoreTypeImports": true
				}
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		detections := config.Rules[0].RestrictedImportsDetections
		if len(detections) == 0 || detections[0] == nil || !detections[0].Enabled {
			t.Fatalf("expected restrictedImportsDetection to be enabled")
		}
		if len(detections[0].EntryPoints) != 1 || detections[0].EntryPoints[0] != "src/server.ts" {
			t.Fatalf("unexpected entryPoints: %+v", detections[0].EntryPoints)
		}
		if len(detections[0].GraphExclude) != 1 || detections[0].GraphExclude[0] != "**/*.spec.ts" {
			t.Fatalf("unexpected graphExclude: %+v", detections[0].GraphExclude)
		}
	})

	t.Run("missing entryPoints", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.5",
			"rules": [{
				"path": ".",
				"restrictedImportsDetection": {
					"enabled": true,
					"denyFiles": ["**/*.tsx"]
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "entryPoints is required") {
			t.Fatalf("expected entryPoints validation error, got: %v", err)
		}
	})

	t.Run("missing entryPoints does not fallback to rule-level entry points", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"rules": [{
				"path": ".",
				"prodEntryPoints": ["src/main.ts"],
				"devEntryPoints": ["src/dev.ts"],
				"restrictedImportsDetection": {
					"enabled": true,
					"denyFiles": ["**/*.tsx"]
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "entryPoints is required") {
			t.Fatalf("expected entryPoints validation error, got: %v", err)
		}
	})

	t.Run("missing denyFiles and denyModules", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.5",
			"rules": [{
				"path": ".",
				"restrictedImportsDetection": {
					"enabled": true,
					"entryPoints": ["src/server.ts"]
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "either denyFiles or denyModules") {
			t.Fatalf("expected deny fields validation error, got: %v", err)
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.5",
			"rules": [{
				"path": ".",
				"restrictedImportsDetection": {
					"enabled": true,
					"entryPoints": ["src/server.ts"],
					"denyFiles": ["**/*.tsx"],
					"unknownField": true
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "unknown field 'unknownField'") {
			t.Fatalf("expected unknown field error, got: %v", err)
		}
	})

	t.Run("array support", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"rules": [{
				"path": ".",
				"restrictedImportsDetection": [
					{
						"enabled": true,
						"entryPoints": ["src/server.ts"],
						"denyFiles": ["**/*.tsx"]
					},
					{
						"enabled": true,
						"entryPoints": ["src/client.ts"],
						"denyModules": ["react"]
					}
				]
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		rule := config.Rules[0]
		if len(rule.RestrictedImportsDetections) != 2 {
			t.Fatalf("expected 2 restrictedImportsDetection entries, got %d", len(rule.RestrictedImportsDetections))
		}
		if len(rule.RestrictedImportsDetections[0].EntryPoints) != 1 || rule.RestrictedImportsDetections[0].EntryPoints[0] != "src/server.ts" {
			t.Fatalf("unexpected first restrictedImportsDetection entry points: %+v", rule.RestrictedImportsDetections[0].EntryPoints)
		}
	})

	t.Run("invalid graphExclude pattern", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.6",
			"rules": [{
				"path": ".",
				"restrictedImportsDetection": {
					"enabled": true,
					"entryPoints": ["src/server.ts"],
					"graphExclude": ["./invalid/**"],
					"denyFiles": ["**/*.tsx"]
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "graphExclude[0]") {
			t.Fatalf("expected graphExclude validation error, got: %v", err)
		}
	})
}
