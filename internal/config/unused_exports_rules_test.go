package config

import "testing"

func TestAnyRuleChecksForUnusedExports(t *testing.T) {
	t.Run("returns false with no unused exports", func(t *testing.T) {
		cfg := &RevDepConfig{
			Rules: []Rule{
				{Path: ".", CircularImportsDetections: []*CircularImportsOptions{{Enabled: true}}},
			},
		}
		if AnyRuleChecksForUnusedExports(cfg) {
			t.Error("Expected false")
		}
	})

	t.Run("returns true with unused exports enabled", func(t *testing.T) {
		cfg := &RevDepConfig{
			Rules: []Rule{
				{Path: ".", UnusedExportsDetections: []*UnusedExportsOptions{{Enabled: true}}},
			},
		}
		if !AnyRuleChecksForUnusedExports(cfg) {
			t.Error("Expected true")
		}
	})

	t.Run("returns false with unused exports disabled", func(t *testing.T) {
		cfg := &RevDepConfig{
			Rules: []Rule{
				{Path: ".", UnusedExportsDetections: []*UnusedExportsOptions{{Enabled: false}}},
			},
		}
		if AnyRuleChecksForUnusedExports(cfg) {
			t.Error("Expected false")
		}
	})
}
