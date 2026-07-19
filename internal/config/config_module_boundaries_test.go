package config

import "testing"

func TestParseConfig_ModuleBoundariesMutuallyExclusive(t *testing.T) {
	t.Run("valid mutuallyExclusive group", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.9",
			"workspaces": [{
				"path": ".",
				"moduleBoundaries": [{
					"name": "feature-isolation",
					"mutuallyExclusive": [
						"src/modules/analytics/**",
						"src/modules/billing/**",
						"src/modules/reporting/**"
					]
				}]
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		boundaries := config.Rules[0].ModuleBoundaries
		if len(boundaries) != 1 {
			t.Fatalf("expected 1 boundary, got %d", len(boundaries))
		}
		if len(boundaries[0].MutuallyExclusive) != 3 {
			t.Fatalf("expected 3 mutuallyExclusive globs, got %+v", boundaries[0].MutuallyExclusive)
		}
		if boundaries[0].Pattern != "" {
			t.Fatalf("expected empty pattern for mutuallyExclusive boundary, got %q", boundaries[0].Pattern)
		}
	})

	t.Run("valid explicit boundary still accepted", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.9",
			"workspaces": [{
				"path": ".",
				"moduleBoundaries": [{
					"name": "client-boundary",
					"pattern": "packages/client/**",
					"deny": ["packages/server/**"]
				}]
			}]
		}`

		if _, err := ParseConfig([]byte(configJSON)); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("mutuallyExclusive cannot combine with pattern", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.9",
			"workspaces": [{
				"path": ".",
				"moduleBoundaries": [{
					"name": "mixed",
					"pattern": "src/a/**",
					"mutuallyExclusive": ["src/a/**", "src/b/**"]
				}]
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "cannot be combined") {
			t.Fatalf("expected XOR validation error, got: %v", err)
		}
	})

	t.Run("mutuallyExclusive cannot combine with deny", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.9",
			"workspaces": [{
				"path": ".",
				"moduleBoundaries": [{
					"name": "mixed",
					"deny": ["src/b/**"],
					"mutuallyExclusive": ["src/a/**", "src/b/**"]
				}]
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "cannot be combined") {
			t.Fatalf("expected XOR validation error, got: %v", err)
		}
	})

	t.Run("mutuallyExclusive requires at least 2 globs", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.9",
			"workspaces": [{
				"path": ".",
				"moduleBoundaries": [{
					"name": "too-short",
					"mutuallyExclusive": ["src/a/**"]
				}]
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "at least 2") {
			t.Fatalf("expected min-length validation error, got: %v", err)
		}
	})

	t.Run("valid denyIgnore carve-out", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.9",
			"workspaces": [{
				"path": ".",
				"moduleBoundaries": [{
					"name": "ui-no-api-internals",
					"pattern": "src/ui/**",
					"deny": ["src/api/**"],
					"denyIgnore": ["src/api/dto/**"]
				}]
			}]
		}`

		config, err := ParseConfig([]byte(configJSON))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		b := config.Rules[0].ModuleBoundaries[0]
		if len(b.DenyIgnore) != 1 || b.DenyIgnore[0] != "src/api/dto/**" {
			t.Fatalf("unexpected denyIgnore: %+v", b.DenyIgnore)
		}
	})

	t.Run("denyIgnore requires deny", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.9",
			"workspaces": [{
				"path": ".",
				"moduleBoundaries": [{
					"name": "no-deny",
					"pattern": "src/ui/**",
					"denyIgnore": ["src/api/dto/**"]
				}]
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "denyIgnore requires 'deny'") {
			t.Fatalf("expected denyIgnore-requires-deny error, got: %v", err)
		}
	})

	t.Run("denyIgnore cannot combine with mutuallyExclusive", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.9",
			"workspaces": [{
				"path": ".",
				"moduleBoundaries": [{
					"name": "mixed",
					"mutuallyExclusive": ["src/a/**", "src/b/**"],
					"denyIgnore": ["src/a/dto/**"]
				}]
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "cannot be combined") {
			t.Fatalf("expected XOR validation error, got: %v", err)
		}
	})

	t.Run("explicit boundary still requires pattern", func(t *testing.T) {
		configJSON := `{
			"configVersion": "1.9",
			"workspaces": [{
				"path": ".",
				"moduleBoundaries": [{
					"name": "no-pattern",
					"deny": ["src/b/**"]
				}]
			}]
		}`

		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !contains(err.Error(), "pattern is required") {
			t.Fatalf("expected pattern-required validation error, got: %v", err)
		}
	})
}
