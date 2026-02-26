package main

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
					"denyFiles": ["**/*.tsx"],
					"denyModules": ["react", "react-*"],
					"ignoreMatches": ["src/server/allowed.tsx"],
					"ignoreTypeImports": true
				}
			}]
		}`

		configs, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		opts := configs[0].Rules[0].RestrictedImportsDetection
		if opts == nil || !opts.Enabled {
			t.Fatalf("expected restrictedImportsDetection to be enabled")
		}
		if len(opts.EntryPoints) != 1 || opts.EntryPoints[0] != "src/server.ts" {
			t.Fatalf("unexpected entryPoints: %+v", opts.EntryPoints)
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
}
