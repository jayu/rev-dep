package config

import "testing"

func TestParseConfig_RestrictedImportersDetection(t *testing.T) {
	t.Run("valid allowed-entry-points config", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.10",
			"rules": [{
				"path": ".",
				"restrictedImportersDetection": {
					"enabled": true,
					"files": ["legacy/**"],
					"allowedEntryPoints": ["src/admin/main.ts"],
					"graphExclude": ["**/*.spec.ts"],
					"ignoreMatches": ["src/known.ts"],
					"ignoreTypeImports": true
				}
			}]
		}`

		cfg, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		detections := cfg.Rules[0].RestrictedImportersDetections
		if len(detections) != 1 || detections[0] == nil || !detections[0].Enabled {
			t.Fatalf("expected restrictedImportersDetection to be enabled")
		}
		if len(detections[0].AllowedEntryPoints) != 1 || detections[0].AllowedEntryPoints[0] != "src/admin/main.ts" {
			t.Fatalf("unexpected allowedEntryPoints: %+v", detections[0].AllowedEntryPoints)
		}
	})

	t.Run("requires allowedEntryPoints when enabled", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.10",
			"rules": [{
				"path": ".",
				"restrictedImportersDetection": {"enabled": true, "files": ["legacy/**"]}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error when allowedEntryPoints is not set")
		}
		if !contains(err.Error(), "allowedEntryPoints is required") {
			t.Fatalf("expected allowedEntryPoints-required error, got: %v", err)
		}
	})

	t.Run("missing files and modules when enabled", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.10",
			"rules": [{
				"path": ".",
				"restrictedImportersDetection": {"enabled": true, "allowedEntryPoints": ["src/a.ts"]}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "either files or modules") {
			t.Fatalf("expected files/modules validation error, got: %v", err)
		}
	})

	t.Run("modules alone is valid", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.10",
			"rules": [{
				"path": ".",
				"restrictedImportersDetection": {"enabled": true, "modules": ["moment"], "allowedEntryPoints": ["src/a.ts"]}
			}]
		}`

		if _, err := ParseConfig([]byte(configJSON)); err != nil {
			t.Fatalf("modules alone should be valid, got: %v", err)
		}
	})

	t.Run("disabled detector skips validation", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.10",
			"rules": [{
				"path": ".",
				"restrictedImportersDetection": {"enabled": false}
			}]
		}`

		if _, err := ParseConfig([]byte(configJSON)); err != nil {
			t.Fatalf("disabled detector should not error, got %v", err)
		}
	})
}
