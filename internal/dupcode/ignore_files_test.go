package dupcode

import (
	"strings"
	"testing"
)

// tripledBody appears in three files below, one of which tests can ignore.
const tripledBody = `{
  const parsed = JSON.parse(rawInput);
  const cleaned = normalise(parsed, defaults);
  const checked = validate(cleaned, schemaFor(kind));
  return checked;
}`

func tripledProject(t *testing.T) string {
	t.Helper()
	return writeProject(t, map[string]string{
		"src/a.ts":              "export function a(rawInput, kind) " + tripledBody + "\n",
		"src/b.ts":              "export function b(rawInput, kind) " + tripledBody + "\n",
		"src/generated/c.ts":    "export function c(rawInput, kind) " + tripledBody + "\n",
		"src/unrelated/keep.ts": "export const keep = 1;\n",
	})
}

// TestIgnoreFilesAppliesBeforeCounting is the behaviour that makes ignoring meaningful
// rather than cosmetic. Three copies exist; one lives in an ignored file. With
// MinDuplicates at 3 the remaining two are not a reportable duplicate, so nothing is
// reported - as opposed to reporting a three-copy duplicate with one occurrence hidden.
func TestIgnoreFilesAppliesBeforeCounting(t *testing.T) {
	dir := tripledProject(t)
	base := Options{Cwd: dir, Blinding: Blinding{}, MinTokens: 5, MinLines: 1}

	// All three visible: a three-copy duplicate, which clears MinDuplicates of 3.
	withAll := base
	withAll.MinDuplicates = 3
	dups := detect(t, dir, withAll)
	if len(dups) != 1 || len(dups[0].Occurrences) != 3 {
		t.Fatalf("expected one three-copy duplication, got %#v", dups)
	}

	// Ignore one of the three. Two copies remain, which is below the threshold, so the
	// duplication drops out entirely.
	ignored := base
	ignored.MinDuplicates = 3
	ignored.IgnoreFiles = []string{"src/generated/**"}
	if dups := detect(t, dir, ignored); len(dups) != 0 {
		t.Fatalf("a two-copy duplicate was reported under MinDuplicates=3: %#v", dups)
	}

	// The same ignore with a threshold of 2 still reports it, now with two occurrences
	// and no mention of the ignored file.
	loosened := ignored
	loosened.MinDuplicates = 2
	dups = detect(t, dir, loosened)
	if len(dups) != 1 {
		t.Fatalf("expected the two-copy duplication, got %#v", dups)
	}
	if len(dups[0].Occurrences) != 2 {
		t.Errorf("expected 2 occurrences, got %d", len(dups[0].Occurrences))
	}
	for _, o := range dups[0].Occurrences {
		if strings.Contains(o.File, "generated") {
			t.Errorf("an ignored file was reported as an occurrence: %s", o.File)
		}
	}
	if dups[0].FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", dups[0].FileCount)
	}
}

func TestIgnoreFilesPatternForms(t *testing.T) {
	dir := tripledProject(t)
	base := Options{Cwd: dir, Blinding: Blinding{}, MinTokens: 5, MinLines: 1, MinDuplicates: 2}

	cases := []struct {
		name       string
		patterns   []string
		wantCopies int
	}{
		{"no patterns", nil, 3},
		{"directory glob", []string{"src/generated/**"}, 2},
		{"exact path", []string{"src/a.ts"}, 2},
		{"extension glob across tree", []string{"**/c.ts"}, 2},
		{"two patterns remove two copies", []string{"src/a.ts", "src/b.ts"}, 0},
		{"pattern matching nothing", []string{"does/not/exist/**"}, 3},
	}
	for _, tc := range cases {
		opts := base
		opts.IgnoreFiles = tc.patterns
		dups := detect(t, dir, opts)
		got := 0
		if len(dups) > 0 {
			got = len(dups[0].Occurrences)
		}
		if tc.wantCopies == 0 {
			if len(dups) != 0 {
				t.Errorf("%s: expected nothing reported, got %#v", tc.name, dups)
			}
			continue
		}
		if got != tc.wantCopies {
			t.Errorf("%s: got %d occurrences, want %d", tc.name, got, tc.wantCopies)
		}
	}
}

func TestIgnoreFilesReportsHowManyItRemoved(t *testing.T) {
	dir := tripledProject(t)
	_, stats, err := Detect(Options{
		Cwd: dir, Blinding: Blinding{}, MinTokens: 5, MinLines: 1,
		IgnoreFiles: []string{"src/generated/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.IgnoredFiles != 1 {
		t.Errorf("IgnoredFiles = %d, want 1", stats.IgnoredFiles)
	}
	if stats.Files != 3 {
		t.Errorf("Files = %d, want 3 (the ignored one is not scanned)", stats.Files)
	}
}

// TestIgnoreFilesMatchesViaCollector checks the config-run path applies the same filter,
// so a rule and the command printed beside it cannot disagree.
func TestIgnoreFilesMatchesViaCollector(t *testing.T) {
	dir := tripledProject(t)
	for _, patterns := range [][]string{nil, {"src/generated/**"}, {"src/a.ts", "src/b.ts"}} {
		for _, minDup := range []int{2, 3} {
			opts := Options{
				Cwd: dir, Blinding: Blinding{}, MinTokens: 5, MinLines: 1,
				MinDuplicates: minDup, IgnoreFiles: patterns, CountOnly: true,
			}
			standalone, _, err := Detect(opts)
			if err != nil {
				t.Fatal(err)
			}
			c, files := collectViaParse(t, dir, []CollectorSpec{{Blinding: Blinding{}, MinTokens: 10}})
			shared, _, err := c.Detect(opts, files)
			if err != nil {
				t.Fatal(err)
			}
			if len(shared) != len(standalone) {
				t.Errorf("patterns=%v minDup=%d: collector %d vs standalone %d",
					patterns, minDup, len(shared), len(standalone))
				continue
			}
			for i := range standalone {
				if len(shared[i].Occurrences) != len(standalone[i].Occurrences) {
					t.Errorf("patterns=%v minDup=%d #%d: %d occurrences vs %d",
						patterns, minDup, i+1, len(shared[i].Occurrences), len(standalone[i].Occurrences))
				}
			}
		}
	}
}
