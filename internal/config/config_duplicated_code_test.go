package config

import (
	"strings"
	"testing"
)

func parseRuleJSON(t *testing.T, ruleBody string) RevDepConfig {
	t.Helper()
	raw := `{"configVersion":"1.0","workspaces":[{"path":".",` + ruleBody + `}]}`
	cfg, err := ParseConfig([]byte(raw))
	if err != nil {
		t.Fatalf("parse failed for %s: %v", ruleBody, err)
	}
	return cfg
}

func TestDuplicatedCodeDetectionParsesObjectForm(t *testing.T) {
	cfg := parseRuleJSON(t, `"duplicatedCodeDetection":{"enabled":true,"blindIdentifiers":true,"minTokens":22,"minLines":4,"minDepth":1,"minStatements":2,"minDuplicates":3}`)

	got := cfg.Rules[0].getDuplicatedCodeDetections()
	if len(got) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(got))
	}
	d := got[0]
	if !d.Enabled || !d.BlindIdentifiers || d.MinTokens != 22 || d.MinLines != 4 ||
		d.MinDepth != 1 || d.MinStatements != 2 || d.MinDuplicates != 3 {
		t.Fatalf("options not parsed as written: %+v", d)
	}
}

func TestDuplicatedCodeDetectionParsesIgnoreFiles(t *testing.T) {
	cfg := parseRuleJSON(t, `"duplicatedCodeDetection":{"enabled":true,"ignoreFiles":["**/*.test.ts","src/generated/**"]}`)
	got := cfg.Rules[0].getDuplicatedCodeDetections()
	if len(got) != 1 {
		t.Fatalf("expected 1 detection, got %d", len(got))
	}
	if len(got[0].IgnoreFiles) != 2 ||
		got[0].IgnoreFiles[0] != "**/*.test.ts" || got[0].IgnoreFiles[1] != "src/generated/**" {
		t.Fatalf("ignoreFiles not parsed as written: %#v", got[0].IgnoreFiles)
	}
}

func TestDuplicatedCodeDetectionParsesBooleanShorthand(t *testing.T) {
	cfg := parseRuleJSON(t, `"duplicatedCodeDetection":true`)
	got := cfg.Rules[0].getDuplicatedCodeDetections()
	if len(got) != 1 || !got[0].Enabled {
		t.Fatalf("boolean shorthand did not enable the detector: %+v", got)
	}

	cfg = parseRuleJSON(t, `"duplicatedCodeDetection":false`)
	got = cfg.Rules[0].getDuplicatedCodeDetections()
	if len(got) == 1 && got[0].Enabled {
		t.Fatalf("false shorthand enabled the detector: %+v", got)
	}
}

func TestDuplicatedCodeDetectionParsesArrayForm(t *testing.T) {
	cfg := parseRuleJSON(t, `"duplicatedCodeDetection":[{"enabled":true,"blindIdentifiers":false},{"enabled":true,"blindIdentifiers":true,"minTokens":10}]`)
	got := cfg.Rules[0].getDuplicatedCodeDetections()
	if len(got) != 2 {
		t.Fatalf("expected 2 detections, got %d", len(got))
	}
	if got[0].BlindIdentifiers || !got[1].BlindIdentifiers || got[1].MinTokens != 10 {
		t.Fatalf("array form not parsed as written: %+v %+v", got[0], got[1])
	}
}

func TestDuplicatedCodeDetectionRejectsBadConfig(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"unknown field", `"duplicatedCodeDetection":{"enabled":true,"minChar":10}`, "unknown field"},
		// maxAllowed was a tolerated-count budget. It is gone: a number cannot say WHICH
		// duplication it admits, so a snapshot is the only honest way to accept what exists.
		{"maxAllowed is no longer a setting",
			`"duplicatedCodeDetection":{"enabled":true,"maxAllowed":5}`, "unknown field"},
		{"blinding not a boolean", `"duplicatedCodeDetection":{"enabled":true,"blindIdentifiers":"yes"}`, "must be a boolean"},
		{"blinding wrong type", `"duplicatedCodeDetection":{"enabled":true,"blindStrings":123}`, "must be a boolean"},
		{"threshold wrong type", `"duplicatedCodeDetection":{"enabled":true,"minTokens":"lots"}`, "must be a number"},
		{"fractional threshold", `"duplicatedCodeDetection":{"enabled":true,"minTokens":3.5}`, "whole number"},
		{"negative threshold", `"duplicatedCodeDetection":{"enabled":true,"minLines":-1}`, "negative"},
		{"minDuplicates of one", `"duplicatedCodeDetection":{"enabled":true,"minDuplicates":1}`, "at least 2"},
		{"ignoreFiles not an array", `"duplicatedCodeDetection":{"enabled":true,"ignoreFiles":"src/**"}`, "must be an array"},
		{"ignoreFiles entry not a string", `"duplicatedCodeDetection":{"enabled":true,"ignoreFiles":[1]}`, "must be a string"},
	}
	for _, tc := range cases {
		raw := `{"configVersion":"1.0","workspaces":[{"path":".",` + tc.body + `}]}`
		_, err := ParseConfig([]byte(raw))
		if err == nil {
			t.Errorf("%s: expected an error, got none", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not mention %q", tc.name, err.Error(), tc.want)
		}
	}
}

func TestDuplicatedCodeDetectionRoundTrips(t *testing.T) {
	cfg := parseRuleJSON(t, `"duplicatedCodeDetection":{"enabled":true,"blindIdentifiers":true,"minDepth":1,"minTokens":5}`)
	out, err := cfg.Rules[0].MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"duplicatedCodeDetection"`, `"blindIdentifiers":true`, `"minDepth":1`, `"minTokens":5`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("marshalled rule is missing %s:\n%s", want, out)
		}
	}
}
