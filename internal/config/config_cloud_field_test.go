package config

import "testing"

// The "cloud" field is accepted (and ignored) at both the config root and within
// each rule. It is intentionally not parsed, schema-validated, or documented yet;
// validation must simply not reject it so existing/forward configs keep working.
func TestParseConfig_AllowsCloudField(t *testing.T) {
	configJSON := `{
		"configVersion": "1.8",
		"cloud": { "anything": true, "nested": { "x": 1 } },
		"workspaces": [
			{
				"path": ".",
				"cloud": ["whatever", 123],
				"orphanFilesDetection": { "enabled": true }
			}
		]
	}`

	cfg, err := ParseConfig([]byte(configJSON))
	if err != nil {
		t.Fatalf("expected config with cloud field to parse without error, got: %v", err)
	}

	if len(cfg.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Path != "." {
		t.Errorf("expected rule path '.', got %q", cfg.Rules[0].Path)
	}
}

// An unrelated unknown field must still be rejected, proving "cloud" is an explicit
// allowance rather than validation being loosened across the board.
func TestParseConfig_RejectsOtherUnknownFields(t *testing.T) {
	rootUnknown := `{
		"configVersion": "1.8",
		"somethingElse": true,
		"workspaces": [{ "path": "." }]
	}`
	if _, err := ParseConfig([]byte(rootUnknown)); err == nil {
		t.Errorf("expected error for unknown root field 'somethingElse', got nil")
	}

	ruleUnknown := `{
		"configVersion": "1.8",
		"workspaces": [{ "path": ".", "somethingElse": true }]
	}`
	if _, err := ParseConfig([]byte(ruleUnknown)); err == nil {
		t.Errorf("expected error for unknown rule field 'somethingElse', got nil")
	}
}
