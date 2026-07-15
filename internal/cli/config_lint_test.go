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
