package config

import "testing"

// TestParseConfig_CompactDetectorForms covers the compact detector syntaxes introduced alongside the
// `config compact` command: the boolean shorthand (`true`/`false`) and the object form with an
// optional `enabled` flag (presence of an object opts the detector in).
func TestParseConfig_CompactDetectorForms(t *testing.T) {
	t.Run("boolean true enables detector", func(t *testing.T) {
		cfg, err := ParseConfig([]byte(`{
			"configVersion": "1.11",
			"rules": [{"path": ".", "unresolvedImportsDetection": true}]
		}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := cfg.Rules[0].UnresolvedImportsDetections
		if len(got) != 1 || !got[0].IsEnabled() {
			t.Fatalf("expected one enabled detection, got %+v", got)
		}
	})

	t.Run("boolean false disables detector", func(t *testing.T) {
		cfg, err := ParseConfig([]byte(`{
			"configVersion": "1.11",
			"rules": [{"path": ".", "circularImportsDetection": false}]
		}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := cfg.Rules[0].CircularImportsDetections
		if len(got) != 1 || got[0].IsEnabled() {
			t.Fatalf("expected one disabled detection, got %+v", got)
		}
	})

	t.Run("object without enabled is treated as enabled", func(t *testing.T) {
		cfg, err := ParseConfig([]byte(`{
			"configVersion": "1.11",
			"rules": [{"path": ".", "restrictedImportsDetection": {"entryPoints": ["src/index.ts"], "denyFiles": ["src/secret.ts"]}}]
		}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := cfg.Rules[0].RestrictedImportsDetections
		if len(got) != 1 || !got[0].IsEnabled() {
			t.Fatalf("expected one enabled detection, got %+v", got)
		}
		if len(got[0].EntryPoints) != 1 || got[0].EntryPoints[0] != "src/index.ts" {
			t.Fatalf("options not parsed: %+v", got[0])
		}
	})

	t.Run("object with explicit enabled false and options is disabled", func(t *testing.T) {
		cfg, err := ParseConfig([]byte(`{
			"configVersion": "1.11",
			"rules": [{"path": ".", "orphanFilesDetection": {"enabled": false, "validEntryPoints": ["src/index.ts"]}}]
		}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := cfg.Rules[0].OrphanFilesDetections
		if len(got) != 1 || got[0].IsEnabled() {
			t.Fatalf("expected one disabled detection, got %+v", got)
		}
		if len(got[0].ValidEntryPoints) != 1 {
			t.Fatalf("options not parsed: %+v", got[0])
		}
	})

	t.Run("array mixes booleans and objects", func(t *testing.T) {
		cfg, err := ParseConfig([]byte(`{
			"configVersion": "1.11",
			"rules": [{"path": ".", "unusedNodeModulesDetection": [true, {"excludeModules": ["typescript"]}]}]
		}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := cfg.Rules[0].UnusedNodeModulesDetections
		if len(got) != 2 {
			t.Fatalf("expected two detections, got %+v", got)
		}
		if !got[0].IsEnabled() || !got[1].IsEnabled() {
			t.Fatalf("expected both detections enabled, got %+v", got)
		}
		if len(got[1].ExcludeModules) != 1 {
			t.Fatalf("options not parsed on second detection: %+v", got[1])
		}
	})

	t.Run("array item false disables that detection", func(t *testing.T) {
		cfg, err := ParseConfig([]byte(`{
			"configVersion": "1.11",
			"rules": [{"path": ".", "unusedNodeModulesDetection": [false, {"excludeModules": ["typescript"]}]}]
		}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := cfg.Rules[0].UnusedNodeModulesDetections
		if len(got) != 2 {
			t.Fatalf("expected two detections, got %+v", got)
		}
		if got[0].IsEnabled() {
			t.Errorf("expected first detection (false) to be disabled, got %+v", got[0])
		}
		if !got[1].IsEnabled() || len(got[1].ExcludeModules) != 1 {
			t.Errorf("expected second detection enabled with options, got %+v", got[1])
		}
	})
}
