package config

import "testing"

func TestParseConfig_DevDepsUsageOnProdDetection_IgnoreTypeImports(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.5",
			"rules": [{
				"path": ".",
				"devDepsUsageOnProdDetection": {
					"enabled": true,
					"prodEntryPoints": ["src/server.ts"],
					"ignoreTypeImports": true
				}
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		detections := config.Rules[0].DevDepsUsageOnProdDetections
		if len(detections) == 0 || detections[0] == nil || !detections[0].Enabled {
			t.Fatalf("expected devDepsUsageOnProdDetection to be enabled")
		}
		if !detections[0].IgnoreTypeImports {
			t.Fatalf("expected ignoreTypeImports=true")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.5",
			"rules": [{
				"path": ".",
				"devDepsUsageOnProdDetection": {
					"enabled": true,
					"prodEntryPoints": ["src/server.ts"],
					"ignoreTypeImports": "yes"
				}
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "ignoreTypeImports must be a boolean") {
			t.Fatalf("expected ignoreTypeImports boolean error, got: %v", err)
		}
	})
}
