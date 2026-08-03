package cli

import (
	"testing"

	"github.com/spf13/pflag"

	"rev-dep-go/internal/dupcode"
)

// TestDuplicatedCodeOptionsCarriesEveryFlag guards against a flag that is registered,
// parsed and threaded through as a parameter while never being assigned into the options
// the detector actually reads.
//
// That is exactly what happened to --skip-objects: it worked in the library, had passing
// unit tests, and did nothing on the command line, because one assignment was missing and
// nothing compared the two ends.
func TestDuplicatedCodeOptionsCarriesEveryFlag(t *testing.T) {
	got := duplicatedCodeOptions(
		"/work",
		dupcode.Blinding{Identifiers: true, Strings: true, Numbers: true},
		11, 22, 33, 44, 55,
		true,
		[]string{"ignored/**"},
		[]string{"processed/**"},
		"snap.json",
		"human",
	)

	checks := []struct {
		flag string
		ok   bool
	}{
		{"--cwd", got.Cwd == "/work"},
		{"--blind-identifiers", got.Blinding.Identifiers},
		{"--blind-strings", got.Blinding.Strings},
		{"--blind-numbers", got.Blinding.Numbers},
		{"--min-tokens", got.MinTokens == 11},
		{"--min-lines", got.MinLines == 22},
		{"--min-depth", got.MinDepth == 33},
		{"--min-statements", got.MinStatements == 44},
		{"--min-duplicates", got.MinDuplicates == 55},
		{"--skip-objects", got.SkipObjects},
		{"--ignore-files", len(got.IgnoreFiles) == 1 && got.IgnoreFiles[0] == "ignored/**"},
		{"--process-ignored-files", len(got.ProcessIgnoredFiles) == 1 && got.ProcessIgnoredFiles[0] == "processed/**"},
		{"--snapshot implies hashing", got.NeedHashes},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("%s does not reach the detector: %+v", c.flag, got)
		}
	}
}

func TestDuplicatedCodeOptionsSkipsHashingWithoutASnapshot(t *testing.T) {
	got := duplicatedCodeOptions("/w", dupcode.Blinding{}, 0, 0, 0, 0, 0, false, nil, nil, "", "human")
	if got.NeedHashes {
		t.Error("hashing every reported block is wasted work when nothing will read it")
	}
	// JSON publishes the digest as the identity a comparison keys on, so it must be
	// computed there even without a snapshot.
	if j := duplicatedCodeOptions("/w", dupcode.Blinding{}, 0, 0, 0, 0, 0, false, nil, nil, "", "json"); !j.NeedHashes {
		t.Error("--format json must compute hashes; they are the comparison key")
	}
	if got.SkipObjects {
		t.Error("SkipObjects defaulted to true")
	}
}

// TestDuplicatedCodeFlagsAreAllAccountedFor fails when a new flag is added without being
// considered here.
func TestDuplicatedCodeFlagsAreAllAccountedFor(t *testing.T) {
	// Flags that steer the command rather than the detector.
	commandOnly := map[string]bool{
		"snapshot": true, "update-snapshot": true, "help": true,
		"format": true, "json-snippets": true,
	}
	detectorFlags := map[string]bool{
		"cwd": true, "blind-identifiers": true, "blind-strings": true, "blind-numbers": true,
		"min-tokens": true, "min-lines": true, "min-depth": true,
		"min-statements": true, "min-duplicates": true, "skip-objects": true,
		"ignore-files": true, "process-ignored-files": true,
	}

	duplicatedCodeCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if commandOnly[f.Name] || detectorFlags[f.Name] {
			return
		}
		t.Errorf("flag --%s is registered but not covered by "+
			"TestDuplicatedCodeOptionsCarriesEveryFlag; wire it or mark it command-only", f.Name)
	})
}
