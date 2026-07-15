package cli

import (
	"testing"

	"rev-dep-go/internal/config"
)

func TestDeadPatternLabel(t *testing.T) {
	cases := []struct {
		dp   config.DeadPattern
		want string
	}{
		{config.DeadPattern{OptionKey: "ignoreFiles"}, "ignoreFiles"},
		{config.DeadPattern{DetectorType: "orphanFilesDetection", OptionKey: "validEntryPoints"}, "orphanFilesDetection.validEntryPoints"},
		{config.DeadPattern{DetectorType: "moduleBoundaries", BoundaryIndex: 2, OptionKey: "allow"}, "moduleBoundaries[2].allow"},
	}
	for _, c := range cases {
		if got := deadPatternLabel(c.dp); got != c.want {
			t.Errorf("deadPatternLabel(%+v)=%q, want %q", c.dp, got, c.want)
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
