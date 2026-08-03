package dupcode

import (
	"testing"

	"rev-dep-go/internal/fs"
	"rev-dep-go/internal/parser"
)

// collectViaParse drives a Collector the way the config run does: blocks come out of the
// import-parsing pass rather than from a traversal of the detector's own.
func collectViaParse(t *testing.T, cwd string, specs []CollectorSpec) (*Collector, []string) {
	t.Helper()
	files := fs.GetFiles(cwd, []string{}, fs.FindAndProcessGitIgnoreFilesUpToRepoRoot(cwd), nil)
	c := NewCollector(specs, len(files))
	parser.ParseImportsFromFilesWithBlocks(files, false, parser.ParseModeBasic, c)
	return c, files
}

// TestCollectorMatchesStandaloneDetect is the contract the single-pass integration rests
// on: blocks harvested from the shared parse must yield exactly what a standalone run
// yields. If these ever diverge, a config run and the CLI would disagree about the same
// code, which is worse than either being slow.
func TestCollectorMatchesStandaloneDetect(t *testing.T) {
	for _, blinding := range []Blinding{Blinding{}, Blinding{Identifiers: true}} {
		for _, opts := range []Options{
			{MinTokens: 5, MinLines: 1},
			{MinTokens: 5, MinLines: 1, MinDepth: 2},
			{MinTokens: 5, MinLines: 1, MinStatements: 2},
			{MinTokens: 5, MinLines: 1, MinDuplicates: 3},
			{MinTokens: 50, MinLines: 1},
		} {
			opts.Cwd = fixtureProject
			opts.Blinding = blinding
			opts.CountOnly = true

			standalone, _, err := Detect(opts)
			if err != nil {
				t.Fatal(err)
			}

			c, files := collectViaParse(t, fixtureProject, []CollectorSpec{{
				Blinding: blinding, MinTokens: opts.MinTokens, MinDepth: opts.MinDepth, MinStatements: opts.MinStatements,
			}})
			shared, _, err := c.Detect(opts, files)
			if err != nil {
				t.Fatal(err)
			}

			if len(shared) != len(standalone) {
				t.Fatalf("[%s %+v] collector found %d duplications, standalone found %d",
					blinding, opts, len(shared), len(standalone))
			}
			for i := range standalone {
				if len(shared[i].Occurrences) != len(standalone[i].Occurrences) {
					t.Errorf("[%s] #%d: %d occurrences vs %d",
						blinding, i+1, len(shared[i].Occurrences), len(standalone[i].Occurrences))
				}
				if shared[i].NormLen != standalone[i].NormLen ||
					shared[i].Depth != standalone[i].Depth ||
					shared[i].Statements != standalone[i].Statements {
					t.Errorf("[%s] #%d metrics differ: %+v vs %+v", blinding, i+1,
						shared[i], standalone[i])
				}
				for j := range standalone[i].Occurrences {
					a, b := shared[i].Occurrences[j], standalone[i].Occurrences[j]
					if a.Start != b.Start || a.End != b.End {
						t.Errorf("[%s] #%d occurrence %d: offsets %d-%d vs %d-%d",
							blinding, i+1, j, a.Start, a.End, b.Start, b.End)
					}
				}
			}
		}
	}
}

// TestCollectorSharesOneScanAcrossStricterRules covers the merging: rules that agree on
// blinding but differ in strictness are served from a single scan taken at the loosest floor,
// and each still gets its own answer.
func TestCollectorSharesOneScanAcrossStricterRules(t *testing.T) {
	loose := Options{Cwd: fixtureProject, Blinding: Blinding{}, MinTokens: 10, MinLines: 1, CountOnly: true}
	strict := Options{Cwd: fixtureProject, Blinding: Blinding{}, MinTokens: 50, MinLines: 1, CountOnly: true}

	c, files := collectViaParse(t, fixtureProject, []CollectorSpec{
		{Blinding: Blinding{}, MinTokens: 10},
		{Blinding: Blinding{}, MinTokens: 50},
	})
	if len(c.specs) != 1 {
		t.Fatalf("two literal specs should merge into one scan, got %d", len(c.specs))
	}
	if c.specs[0].minTokens != 10 {
		t.Errorf("merged scan should use the loosest floor, got %d", c.specs[0].minTokens)
	}

	gotLoose, _, _ := c.Detect(loose, files)
	gotStrict, _, _ := c.Detect(strict, files)

	wantLoose, _, _ := Detect(loose)
	wantStrict, _, _ := Detect(strict)

	if len(gotLoose) != len(wantLoose) {
		t.Errorf("loose rule: %d vs %d", len(gotLoose), len(wantLoose))
	}
	if len(gotStrict) != len(wantStrict) {
		t.Errorf("strict rule: %d vs %d", len(gotStrict), len(wantStrict))
	}
	if len(gotStrict) >= len(gotLoose) {
		t.Errorf("the strict rule should see fewer duplications: %d vs %d", len(gotStrict), len(gotLoose))
	}
}

func TestCollectorHandlesBothModesInOneRun(t *testing.T) {
	c, files := collectViaParse(t, fixtureProject, []CollectorSpec{
		{Blinding: Blinding{}, MinTokens: 10},
		{Blinding: Blinding{Identifiers: true}, MinTokens: 8},
	})
	if len(c.specs) != 2 {
		t.Fatalf("two distinct modes need two scans, got %d", len(c.specs))
	}

	for _, tc := range []struct {
		blinding  Blinding
		minTokens int
	}{{Blinding{}, 40}, {Blinding{Identifiers: true}, 30}} {
		opts := Options{Cwd: fixtureProject, Blinding: tc.blinding, MinTokens: tc.minTokens, MinLines: 1, CountOnly: true}
		got, _, _ := c.Detect(opts, files)
		want, _, _ := Detect(opts)
		if len(got) != len(want) {
			t.Errorf("[%s] collector %d vs standalone %d", tc.blinding, len(got), len(want))
		}
	}
}

// TestCollectorFallsBackForUncollectedMode makes sure an unexpected blinding produces the
// right answer rather than a silent zero.
func TestCollectorFallsBackForUncollectedMode(t *testing.T) {
	c, files := collectViaParse(t, fixtureProject, []CollectorSpec{{Blinding: Blinding{}, MinTokens: 10}})

	opts := Options{Cwd: fixtureProject, Blinding: Blinding{Identifiers: true}, MinTokens: 5, MinLines: 1, CountOnly: true}
	got, _, err := c.Detect(opts, files)
	if err != nil {
		t.Fatal(err)
	}
	want, _, _ := Detect(opts)
	if len(got) != len(want) {
		t.Errorf("fallback produced %d duplications, want %d", len(got), len(want))
	}
	if len(got) == 0 {
		t.Error("fallback produced nothing at all")
	}
}

// TestCollectorRestrictsToTheRulesFiles checks the scoping: a rule must only see
// duplicates among its own files even though the collector holds the whole run.
func TestCollectorRestrictsToTheRulesFiles(t *testing.T) {
	c, files := collectViaParse(t, fixtureProject, []CollectorSpec{{Blinding: Blinding{}, MinTokens: 10}})
	opts := Options{Cwd: fixtureProject, Blinding: Blinding{}, MinTokens: 5, MinLines: 1, CountOnly: true}

	all, _, _ := c.Detect(opts, files)
	if len(all) == 0 {
		t.Fatal("no duplications across the whole fixture")
	}

	// One file on its own cannot duplicate anything the fixture duplicates across files.
	var single []string
	for _, f := range files {
		if len(f) > 0 {
			single = []string{f}
			break
		}
	}
	scoped, _, _ := c.Detect(opts, single)
	if len(scoped) >= len(all) {
		t.Errorf("scoping to one file did not narrow the result: %d vs %d", len(scoped), len(all))
	}
}

// TestParseImportsUnaffectedByBlockCollection guards the other half of the merge: turning
// block collection on must not change the imports the same pass reports.
func TestParseImportsUnaffectedByBlockCollection(t *testing.T) {
	files := fs.GetFiles(fixtureProject, []string{}, fs.FindAndProcessGitIgnoreFilesUpToRepoRoot(fixtureProject), nil)

	plain, errsA := parser.ParseImportsFromFiles(files, false, parser.ParseModeDetailed)
	c := NewCollector([]CollectorSpec{{Blinding: Blinding{}, MinTokens: 10}}, len(files))
	withBlocks, errsB := parser.ParseImportsFromFilesWithBlocks(files, false, parser.ParseModeDetailed, c)

	if errsA != errsB {
		t.Errorf("error counts differ: %d vs %d", errsA, errsB)
	}
	if len(plain) != len(withBlocks) {
		t.Fatalf("file counts differ: %d vs %d", len(plain), len(withBlocks))
	}
	for i := range plain {
		if plain[i].FilePath != withBlocks[i].FilePath {
			t.Fatalf("file order differs at %d: %s vs %s", i, plain[i].FilePath, withBlocks[i].FilePath)
		}
		if len(plain[i].Imports) != len(withBlocks[i].Imports) {
			t.Fatalf("%s: %d imports vs %d", plain[i].FilePath,
				len(plain[i].Imports), len(withBlocks[i].Imports))
		}
		for j := range plain[i].Imports {
			a, b := plain[i].Imports[j], withBlocks[i].Imports[j]
			if a.Request != b.Request || a.RequestStart != b.RequestStart || a.Kind != b.Kind {
				t.Errorf("%s import %d differs:\n  plain:  %+v\n  blocks: %+v",
					plain[i].FilePath, j, a, b)
			}
		}
	}
}

// TestCollectorAppliesSkipObjectsPerDetection covers a bug that only appeared with more
// than one detection.
//
// One scan serves every detection in a run, so a filter that only some of them asked for
// cannot live in the scan: reconciling "skip objects" and "keep objects" into a single flag
// is wrong for one of them whichever way it goes. It was reconciled with AND, so a rule
// asking to skip objects silently kept them as soon as any other rule did not ask.
func TestCollectorAppliesSkipObjectsPerDetection(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.ts": "export const cfgA = " + flatObject + ";\nexport function runA(rawInput, kind) " + nestedLogic + "\n",
		"b.ts": "export const cfgB = " + flatObject + ";\nexport function runB(rawInput, kind) " + nestedLogic + "\n",
	})

	// Two detections sharing one scan, disagreeing about objects.
	c, files := collectViaParse(t, dir, []CollectorSpec{
		{Blinding: Blinding{}, MinTokens: 10},
		{Blinding: Blinding{}, MinTokens: 5, SkipObjects: true},
	})

	base := Options{Cwd: dir, Blinding: Blinding{}, MinTokens: 5, MinLines: 1, CountOnly: true}
	keeping, _, err := c.Detect(base, files)
	if err != nil {
		t.Fatal(err)
	}
	skipping, _, err := c.Detect(withSkipObjects(base), files)
	if err != nil {
		t.Fatal(err)
	}

	if len(keeping) != 2 {
		t.Fatalf("the detection that kept objects should see both: %s", describe(keeping))
	}
	if len(skipping) != 1 {
		t.Fatalf("the detection that skipped objects still saw them: %s", describe(skipping))
	}
	if skipping[0].IsObjectLiteral {
		t.Error("an object literal survived SkipObjects on the shared-scan path")
	}

	// And each still agrees with what a standalone run of the same options produces.
	for _, opts := range []Options{base, withSkipObjects(base)} {
		want, _, _ := Detect(opts)
		got, _, _ := c.Detect(opts, files)
		if len(got) != len(want) {
			t.Errorf("SkipObjects=%v: collector %d vs standalone %d",
				opts.SkipObjects, len(got), len(want))
		}
	}
}

// TestCollectorSkipObjectsAcrossModes covers the same hazard when the shared scan is the
// dual-blinding one, where the two specs are literal and structural rather than two literals.
func TestCollectorSkipObjectsAcrossModes(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.ts": "export const cfgA = " + flatObject + ";\nexport function runA(rawInput, kind) " + nestedLogic + "\n",
		"b.ts": "export const cfgB = " + flatObject + ";\nexport function runB(rawInput, kind) " + nestedLogic + "\n",
	})
	c, files := collectViaParse(t, dir, []CollectorSpec{
		{Blinding: Blinding{}, MinTokens: 10},
		{Blinding: Blinding{Identifiers: true}, MinTokens: 5, SkipObjects: true},
	})

	lit := Options{Cwd: dir, Blinding: Blinding{}, MinTokens: 5, MinLines: 1, CountOnly: true}
	str := Options{Cwd: dir, Blinding: Blinding{Identifiers: true}, MinTokens: 5, MinLines: 1, CountOnly: true, SkipObjects: true}

	gotLit, _, _ := c.Detect(lit, files)
	gotStr, _, _ := c.Detect(str, files)

	wantLit, _, _ := Detect(lit)
	wantStr, _, _ := Detect(str)

	if len(gotLit) != len(wantLit) {
		t.Errorf("literal detection: %d vs standalone %d", len(gotLit), len(wantLit))
	}
	if len(gotStr) != len(wantStr) {
		t.Errorf("structural detection: %d vs standalone %d", len(gotStr), len(wantStr))
	}
	for _, d := range gotStr {
		if d.IsObjectLiteral {
			t.Errorf("an object literal survived SkipObjects in the dual-blinding scan: %q", truncate(d.Snippet))
		}
	}
}
