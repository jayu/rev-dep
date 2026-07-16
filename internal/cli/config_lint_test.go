package cli

import (
	"testing"

	"rev-dep-go/internal/config"
)

func TestOptionLabel(t *testing.T) {
	cases := []struct {
		detectorType  string
		boundaryIndex int
		optionKey     string
		want          string
	}{
		{"", -1, "ignoreFiles", "ignoreFiles"},
		{"orphanFilesDetection", -1, "validEntryPoints", "orphanFilesDetection.validEntryPoints"},
		{"moduleBoundaries", 2, "allow", "moduleBoundaries[2].allow"},
	}
	for _, c := range cases {
		if got := optionLabel(c.detectorType, c.boundaryIndex, c.optionKey); got != c.want {
			t.Errorf("optionLabel(%q,%d,%q)=%q, want %q", c.detectorType, c.boundaryIndex, c.optionKey, got, c.want)
		}
	}
}

func TestKindSuffix(t *testing.T) {
	if kindSuffix(config.KindModule) == kindSuffix(config.KindFile) {
		t.Error("module and file suffixes should differ")
	}
	if kindSuffix(config.KindDir) == "" {
		t.Error("dir suffix should be non-empty")
	}
}

func TestConfigLintCommandRegistered(t *testing.T) {
	found := false
	for _, c := range configCmd.Commands() {
		if c.Name() == "lint" {
			found = true
			if !c.Flags().HasAvailableFlags() {
				t.Error("config lint should have flags")
			}
			if c.Flags().Lookup("fix") == nil {
				t.Error("config lint should expose --fix")
			}
		}
	}
	if !found {
		t.Error("config lint command not registered under config")
	}
}

func TestCountLintFindings(t *testing.T) {
	res := &config.LintResult{
		DeadPatterns: []config.DeadPattern{
			{Severity: config.SeverityError, Removable: true},  // removable error
			{Severity: config.SeverityError, Removable: false}, // report-only error
			{Severity: config.SeverityWarning},                 // negation warning
		},
		Overlaps:           make([]config.OverlapFinding, 3),
		TrailingCommaCount: 5,
		CompactableCount:   2,
	}
	// Without --fix: both errors count; warnings = negation(1) + overlaps(3) + commas(5) + compact(2) = 11.
	if e, w := countLintFindings(res, false); e != 2 || w != 11 {
		t.Errorf("no-fix: got errors=%d warnings=%d, want 2 and 11", e, w)
	}
	// With --fix: removable error is cleared (1 error left); auto-fixable warnings excluded → 1 negation + 3 overlaps = 4.
	if e, w := countLintFindings(res, true); e != 1 || w != 4 {
		t.Errorf("fix: got errors=%d warnings=%d, want 1 and 4", e, w)
	}
}

func TestConfigRunLintFlagsRegistered(t *testing.T) {
	if configRunCmd.Flags().Lookup("lint") == nil {
		t.Error("config run should expose --lint")
	}
	if configRunCmd.Flags().Lookup("lint-rules") == nil {
		t.Error("config run should expose --lint-rules")
	}
}
