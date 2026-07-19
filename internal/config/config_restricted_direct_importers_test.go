package config

import (
	"encoding/json"
	"testing"
)

func TestParseConfig_RestrictedDirectImportersDetection(t *testing.T) {
	t.Run("valid allowImporters config", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.11",
			"workspaces": [{
				"path": ".",
				"restrictedDirectImportersDetection": {
					"enabled": true,
					"files": ["config/**"],
					"allowImporters": ["src/config/**"],
					"ignoreMatches": ["src/known.ts"],
					"ignoreTypeImports": true
				}
			}]
		}`

		cfg, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		detections := cfg.Rules[0].RestrictedDirectImportersDetections
		if len(detections) != 1 || detections[0] == nil || !detections[0].Enabled {
			t.Fatalf("expected restrictedDirectImportersDetection to be enabled")
		}
		if len(detections[0].AllowImporters) != 1 || detections[0].AllowImporters[0] != "src/config/**" {
			t.Fatalf("unexpected allowImporters: %+v", detections[0].AllowImporters)
		}
	})

	t.Run("valid denyImporters + modules config", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.11",
			"workspaces": [{
				"path": ".",
				"restrictedDirectImportersDetection": {
					"enabled": true,
					"modules": ["axios", "@legacy/*"],
					"denyImporters": ["src/public/**"]
				}
			}]
		}`

		if _, err := ParseConfig([]byte(configJSON)); err != nil {
			t.Fatalf("modules + denyImporters should be valid, got: %v", err)
		}
	})

	t.Run("files and modules are mutually exclusive", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.11",
			"workspaces": [{
				"path": ".",
				"restrictedDirectImportersDetection": {
					"enabled": true,
					"files": ["config/**"],
					"modules": ["axios"],
					"denyImporters": ["src/**"]
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error when both files and modules are set")
		}
		if !contains(err.Error(), "'files' and 'modules' are mutually exclusive") {
			t.Fatalf("expected files/modules XOR error, got: %v", err)
		}
	})

	t.Run("allowImporters and denyImporters are mutually exclusive", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.11",
			"workspaces": [{
				"path": ".",
				"restrictedDirectImportersDetection": {
					"enabled": true,
					"files": ["config/**"],
					"allowImporters": ["src/a/**"],
					"denyImporters": ["src/b/**"]
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error when both allowImporters and denyImporters are set")
		}
		if !contains(err.Error(), "'allowImporters' and 'denyImporters' are mutually exclusive") {
			t.Fatalf("expected allow/deny XOR error, got: %v", err)
		}
	})

	t.Run("missing files and modules when enabled", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.11",
			"workspaces": [{
				"path": ".",
				"restrictedDirectImportersDetection": {"enabled": true, "denyImporters": ["src/**"]}
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

	t.Run("missing allowImporters and denyImporters when enabled", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.11",
			"workspaces": [{
				"path": ".",
				"restrictedDirectImportersDetection": {"enabled": true, "files": ["config/**"]}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "either allowImporters or denyImporters") {
			t.Fatalf("expected allow/deny validation error, got: %v", err)
		}
	})

	t.Run("array form parses multiple detections", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.11",
			"workspaces": [{
				"path": ".",
				"restrictedDirectImportersDetection": [
					{"enabled": true, "files": ["config/**"], "denyImporters": ["src/public/**"]},
					{"enabled": true, "modules": ["axios"], "allowImporters": ["src/net/**"]}
				]
			}]
		}`

		cfg, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(cfg.Rules[0].RestrictedDirectImportersDetections) != 2 {
			t.Fatalf("expected 2 detections, got %d", len(cfg.Rules[0].RestrictedDirectImportersDetections))
		}
	})

	t.Run("disabled detector skips validation", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.11",
			"workspaces": [{
				"path": ".",
				"restrictedDirectImportersDetection": {"enabled": false}
			}]
		}`

		if _, err := ParseConfig([]byte(configJSON)); err != nil {
			t.Fatalf("disabled detector should not error, got %v", err)
		}
	})

	t.Run("round-trips through marshal", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.11",
			"workspaces": [{
				"path": ".",
				"restrictedDirectImportersDetection": {"enabled": true, "files": ["config/**"], "denyImporters": ["src/public/**"]}
			}]
		}`

		cfg, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		out, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !contains(string(out), "restrictedDirectImportersDetection") {
			t.Fatalf("marshalled config missing detector key: %s", out)
		}
	})
}
