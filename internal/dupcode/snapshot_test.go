package dupcode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rev-dep-go/internal/emoji"
)

// NeedHashes is set throughout: a snapshot is built from these runs, and the digest is
// what it records.
func snapshotOpts(dir string) Options {
	return Options{Cwd: dir, Blinding: Blinding{}, MinTokens: 5, MinLines: 1, NeedHashes: true}
}

// tripledProject is defined in ignore_files_test.go: the same body in three files.

func TestCanonicalHashIgnoresFormattingAndComments(t *testing.T) {
	a := []byte(`{
  const total = items.length;
  if (total > 0) { return total; }
  return 0;
}`)
	b := []byte(`{const total=items.length;
      /* a comment that was not there */
      if(total>0){return total;}
      return 0;}`)

	if CanonicalHash(a, Blinding{}) != CanonicalHash(b, Blinding{}) {
		t.Fatalf("reformatting changed the identity:\n a=%s\n b=%s",
			CanonicalHash(a, Blinding{}), CanonicalHash(b, Blinding{}))
	}
}

func TestCanonicalHashIgnoresRenamingOnlyInStructuralMode(t *testing.T) {
	a := []byte(`{ const total = items.length; return total * 2; }`)
	b := []byte(`{ const size = values.length; return size * 2; }`)

	if CanonicalHash(a, Blinding{}) == CanonicalHash(b, Blinding{}) {
		t.Error("literal blinding treated a renamed copy as the same code")
	}
	if CanonicalHash(a, Blinding{Identifiers: true}) != CanonicalHash(b, Blinding{Identifiers: true}) {
		t.Error("structural blinding should give a renamed copy the same identity")
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	dir := tripledProject(t)
	opts := snapshotOpts(dir)
	dups, _, err := Detect(opts)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "nested", "dup.json")
	if err := SaveSnapshot(path, BuildSnapshot(dups, opts)); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SchemaVersion != SnapshotSchemaVersion || loaded.CanonicalFormVersion != CanonicalFormVersion {
		t.Errorf("versions not round-tripped: %+v", loaded)
	}
	if len(loaded.Entries) != len(dups) {
		t.Fatalf("%d entries, want %d", len(loaded.Entries), len(dups))
	}
	if loaded.Parameters.MinTokens != 5 || loaded.Parameters.Blinding != "exact" {
		t.Errorf("parameters not recorded: %+v", loaded.Parameters)
	}
	// Defaults must be recorded as the values actually used, or an unset field would
	// later look like a changed setting.
	if loaded.Parameters.MinDuplicates != 2 {
		t.Errorf("MinDuplicates recorded as %d, want the effective default 2", loaded.Parameters.MinDuplicates)
	}
}

func TestSnapshotIsDeterministicAndSorted(t *testing.T) {
	dir := tripledProject(t)
	opts := snapshotOpts(dir)
	dups, _, _ := Detect(opts)

	first := BuildSnapshot(dups, opts)
	second := BuildSnapshot(dups, opts)

	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatal("two snapshots of the same run differ")
	}
	for i := 1; i < len(first.Entries); i++ {
		if first.Entries[i-1].Hash > first.Entries[i].Hash {
			t.Fatalf("entries are not sorted by hash: %s before %s",
				first.Entries[i-1].Hash, first.Entries[i].Hash)
		}
	}
}

func TestSnapshotFileIsReadable(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"A.tsx": "export const A = () => (<Card><Body /></Card>);\nexport function a(rawInput, kind) " + nestedLogic + "\n",
		"B.tsx": "export const B = () => (<Card><Body /></Card>);\nexport function b(rawInput, kind) " + nestedLogic + "\n",
	})
	opts := snapshotOpts(dir)
	dups, _, _ := Detect(opts)

	path := filepath.Join(t.TempDir(), "dup.json")
	if err := SaveSnapshot(path, BuildSnapshot(dups, opts)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)

	// A snapshot is reviewed in pull requests, so JSX in a label must arrive as itself
	// rather than as a \u003c escape.
	if strings.Contains(string(raw), `\u003c`) {
		t.Errorf("labels are HTML-escaped and unreadable:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"label"`) {
		t.Errorf("no labels written:\n%s", raw)
	}
	if strings.Contains(string(raw), "JSON.parse(rawInput)\\n") {
		t.Errorf("a multi-line body leaked into a label:\n%s", raw)
	}
}

func TestLoadSnapshotMissingIsDistinguishable(t *testing.T) {
	_, err := LoadSnapshot(filepath.Join(t.TempDir(), "absent.json"))
	if err == nil {
		t.Fatal("expected an error")
	}
	var missing *SnapshotMissingError
	if !asSnapshotMissing(err, &missing) {
		t.Fatalf("a missing file must be distinguishable from a broken one, got %T: %v", err, err)
	}
}

func TestLoadSnapshotRejectsCorruptAndWrongSchema(t *testing.T) {
	dir := t.TempDir()

	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("{not json"), 0o644)
	if _, err := LoadSnapshot(bad); err == nil {
		t.Error("corrupt snapshot accepted")
	}

	future := filepath.Join(dir, "future.json")
	os.WriteFile(future, []byte(`{"schemaVersion": "99.0", "duplications": []}`), 0o644)
	_, err := LoadSnapshot(future)
	if err == nil || !strings.Contains(err.Error(), "schema version") {
		t.Errorf("a future schema should be rejected clearly, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// delta
// ---------------------------------------------------------------------------

func deltaFor(t *testing.T, dir string, opts Options, snap *Snapshot) *Delta {
	t.Helper()
	opts.Cwd = dir
	dups, _, err := Detect(opts)
	if err != nil {
		t.Fatal(err)
	}
	return CompareToSnapshot(dups, nil, snap, opts)
}

func TestDeltaCleanWhenNothingChanged(t *testing.T) {
	dir := tripledProject(t)
	opts := snapshotOpts(dir)
	dups, _, _ := Detect(opts)
	snap := BuildSnapshot(dups, opts)

	delta := deltaFor(t, dir, opts, snap)
	if !delta.IsClean() {
		t.Fatalf("unchanged run produced a delta: %+v", delta)
	}
	if delta.Unchanged != len(dups) {
		t.Errorf("Unchanged = %d, want %d", delta.Unchanged, len(dups))
	}
}

func TestDeltaSurvivesReformatting(t *testing.T) {
	dir := tripledProject(t)
	opts := snapshotOpts(dir)
	dups, _, _ := Detect(opts)
	snap := BuildSnapshot(dups, opts)

	// Reformat one copy and add comments: same code, different bytes on disk.
	path := filepath.Join(dir, "src", "a.ts")
	original, _ := os.ReadFile(path)
	reformatted := strings.ReplaceAll(string(original), "\n", "\n   ")
	reformatted = strings.Replace(reformatted, "export function", "// a new comment\nexport function", 1)
	os.WriteFile(path, []byte(reformatted), 0o644)

	delta := deltaFor(t, dir, opts, snap)
	if !delta.IsClean() {
		t.Fatalf("reformatting broke the snapshot: %s", DeltaSummary(delta))
	}
}

// Raising --min-duplicates creates a gap the report cannot see into: a pattern with two
// copies and a threshold of three is real duplication that simply does not qualify. Deleting
// one of three copies lands exactly there, and calling it "resolved" would tell the reader
// they finished a job they are halfway through.
func TestDeltaSeparatesBelowThresholdFromResolved(t *testing.T) {
	dir := tripledProject(t)
	opts := snapshotOpts(dir)
	opts.MinDuplicates = 3
	dups, _, err := Detect(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(dups) != 1 {
		t.Fatalf("fixture should hold one three-copy duplication, got %d", len(dups))
	}
	snap := BuildSnapshot(dups, opts)

	// One of the three copies goes away. Two are still there.
	if err := os.Remove(filepath.Join(dir, "src", "generated", "c.ts")); err != nil {
		t.Fatal(err)
	}

	opts.Cwd = dir
	after, below, _, err := DetectWithBelowThreshold(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Fatalf("two copies must not qualify at min-duplicates 3, got %d findings", len(after))
	}
	if len(below) != 1 {
		t.Fatalf("the two surviving copies should be tracked below the threshold, got %d", len(below))
	}

	delta := CompareToSnapshot(after, below, snap, opts)

	if len(delta.Resolved) != 0 {
		t.Errorf("code that is still duplicated twice was reported as resolved: %+v", delta.Resolved)
	}
	if len(delta.Dropped) != 1 {
		t.Fatalf("expected 1 pattern below the threshold, got %d", len(delta.Dropped))
	}
	got := delta.Dropped[0]
	if got.Was.Occurrences != 3 || got.Now.Occurrences != 2 {
		t.Errorf("copy counts = %d -> %d, want 3 -> 2", got.Was.Occurrences, got.Now.Occurrences)
	}
	if len(got.FileChanges) != 1 || !strings.HasSuffix(got.FileChanges[0].Path, "c.ts") ||
		!got.FileChanges[0].IsGone() {
		t.Errorf("the file that lost its copy is not named: %+v", got.FileChanges)
	}

	// The surviving copies are what the reader has to act on, so the code and its locations
	// have to be there - which is the whole reason the run tracks them.
	if got.Finding == nil {
		t.Fatal("no live finding attached, so the delta cannot show the remaining copies")
	}
	if len(got.Finding.Occurrences) != 2 {
		t.Errorf("attached finding has %d occurrences, want 2", len(got.Finding.Occurrences))
	}
	if got.Finding.Snippet == "" {
		t.Error("attached finding carries no snippet")
	}

	// Fewer copies than acknowledged is an improvement, but the snapshot is now stale.
	if delta.IsClean() {
		t.Error("a pattern falling under the threshold is still a change")
	}
	if delta.HasRegressions() {
		t.Error("losing a copy is not a regression")
	}
	if !delta.HasImprovements() {
		t.Error("losing a copy should read as an improvement")
	}

	out := renderDelta(delta, *got.Finding)
	for _, want := range []string{
		"under-min",
		"3 -> 2 copies",
		"still duplicated 2 times - below --min-duplicates 3, not gone",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// Code that is genuinely gone still reads as resolved - the distinction above must not
// swallow the case it was carved out of.
func TestDeltaStillReportsTrulyResolved(t *testing.T) {
	dir := tripledProject(t)
	opts := snapshotOpts(dir)
	opts.MinDuplicates = 3
	dups, _, _ := Detect(opts)
	snap := BuildSnapshot(dups, opts)

	for _, name := range []string{"a.ts", "b.ts", "generated/c.ts"} {
		if err := os.Remove(filepath.Join(dir, "src", name)); err != nil {
			t.Fatal(err)
		}
	}

	opts.Cwd = dir
	after, below, _, _ := DetectWithBelowThreshold(opts)
	delta := CompareToSnapshot(after, below, snap, opts)

	if len(delta.Dropped) != 0 {
		t.Errorf("deleted code was reported as merely below the threshold: %+v", delta.Dropped)
	}
	if len(delta.Resolved) != 1 {
		t.Fatalf("expected 1 resolved pattern, got %d", len(delta.Resolved))
	}
}

// At the default threshold there is no gap between "not duplicated" and "reported", so
// nothing is tracked and nothing costs anything.
func TestNothingIsTrackedBelowTheDefaultThreshold(t *testing.T) {
	dir := tripledProject(t)
	opts := snapshotOpts(dir)
	opts.MinDuplicates = 2
	opts.Cwd = dir

	_, below, _, err := DetectWithBelowThreshold(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(below) != 0 {
		t.Errorf("min-duplicates 2 leaves nothing below the threshold, got %d", len(below))
	}
}

func TestDeltaDetectsNewSpreadMovedResolved(t *testing.T) {
	dir := tripledProject(t)
	opts := snapshotOpts(dir)
	opts.MinDuplicates = 2
	dups, _, _ := Detect(opts)
	snap := BuildSnapshot(dups, opts)
	if len(snap.Entries) != 1 {
		t.Fatalf("fixture should hold one duplication, got %d", len(snap.Entries))
	}

	t.Run("spread", func(t *testing.T) {
		d := tripledProject(t)
		body, _ := os.ReadFile(filepath.Join(d, "src", "a.ts"))
		os.WriteFile(filepath.Join(d, "src", "d.ts"), body, 0o644)
		delta := deltaFor(t, d, snapshotOpts(d), snap)
		if len(delta.Spread) != 1 {
			t.Fatalf("expected one spread, got %s", DeltaSummary(delta))
		}
		if delta.Spread[0].Was.Occurrences != 3 || delta.Spread[0].Now.Occurrences != 4 {
			t.Errorf("counts wrong: %+v", delta.Spread[0])
		}
		if !delta.HasRegressions() {
			t.Error("a spread should count as a regression")
		}
	})

	t.Run("moved", func(t *testing.T) {
		d := tripledProject(t)
		body, _ := os.ReadFile(filepath.Join(d, "src", "generated", "c.ts"))
		os.Remove(filepath.Join(d, "src", "generated", "c.ts"))
		os.WriteFile(filepath.Join(d, "src", "moved.ts"), body, 0o644)
		delta := deltaFor(t, d, snapshotOpts(d), snap)
		if len(delta.Moved) != 1 {
			t.Fatalf("expected one move, got %s", DeltaSummary(delta))
		}
		c := delta.Moved[0]
		var gained, lost []FileChange
		for _, fc := range c.FileChanges {
			if fc.IsNew() {
				gained = append(gained, fc)
			} else if fc.IsGone() {
				lost = append(lost, fc)
			}
		}
		if len(gained) != 1 || !strings.HasSuffix(gained[0].Path, "moved.ts") {
			t.Errorf("added files wrong: %+v", c.FileChanges)
		}
		if len(lost) != 1 || !strings.Contains(lost[0].Path, "generated") {
			t.Errorf("removed files wrong: %+v", c.FileChanges)
		}
		if !delta.HasRegressions() {
			t.Error("a move should count as a regression")
		}
	})

	t.Run("shrunk", func(t *testing.T) {
		d := tripledProject(t)
		os.Remove(filepath.Join(d, "src", "generated", "c.ts"))
		delta := deltaFor(t, d, snapshotOpts(d), snap)
		if len(delta.Shrunk) != 1 {
			t.Fatalf("expected one shrink, got %s", DeltaSummary(delta))
		}
		if delta.HasRegressions() {
			t.Error("fewer copies is not a regression")
		}
		if !delta.HasImprovements() || delta.IsClean() {
			t.Error("fewer copies must still leave the snapshot out of date")
		}
	})

	t.Run("resolved", func(t *testing.T) {
		d := tripledProject(t)
		for _, f := range []string{"src/b.ts", "src/generated/c.ts"} {
			os.Remove(filepath.Join(d, f))
		}
		delta := deltaFor(t, d, snapshotOpts(d), snap)
		if len(delta.Resolved) != 1 {
			t.Fatalf("expected one resolved, got %s", DeltaSummary(delta))
		}
		if delta.HasRegressions() || !delta.HasImprovements() {
			t.Errorf("resolved should be an improvement: %+v", delta)
		}
	})

	t.Run("new", func(t *testing.T) {
		d := tripledProject(t)
		other := "export function x(a) " + nestedLogic + "\n"
		os.WriteFile(filepath.Join(d, "src", "x1.ts"), []byte(other), 0o644)
		os.WriteFile(filepath.Join(d, "src", "x2.ts"), []byte(other), 0o644)
		delta := deltaFor(t, d, snapshotOpts(d), snap)
		if len(delta.New) != 1 {
			t.Fatalf("expected one new pattern, got %s", DeltaSummary(delta))
		}
		if delta.Unchanged != 1 {
			t.Errorf("the acknowledged pattern should be unchanged, got %d", delta.Unchanged)
		}
		if !delta.HasRegressions() {
			t.Error("a new pattern is a regression")
		}
	})
}

func TestDeltaReportsParameterChangesWithoutInvalidatingHashes(t *testing.T) {
	dir := tripledProject(t)
	opts := snapshotOpts(dir)
	dups, _, _ := Detect(opts)
	snap := BuildSnapshot(dups, opts)

	// Tightening a threshold must not orphan the entries that still qualify: the whole
	// point of keeping parameters out of the hash.
	tighter := opts
	tighter.MinTokens = 60
	delta := deltaFor(t, dir, tighter, snap)

	if len(delta.ParameterChanges) == 0 {
		t.Error("a changed threshold should be reported as context")
	}
	if len(delta.New) != 0 {
		t.Errorf("tightening a threshold invented new patterns: %+v", delta.New)
	}
	if delta.Unchanged == 0 && len(delta.Resolved) == 0 {
		t.Error("expected the entries to be recognised, not re-identified")
	}
}

func TestDeltaFlagsCanonicalFormChange(t *testing.T) {
	dir := tripledProject(t)
	opts := snapshotOpts(dir)
	dups, _, _ := Detect(opts)
	snap := BuildSnapshot(dups, opts)
	snap.CanonicalFormVersion = "0.9"

	delta := CompareToSnapshot(dups, nil, snap, opts)
	if !delta.CanonicalFormChanged {
		t.Error("a normaliser revision change should be flagged")
	}
	// Advisory, not a gate: the comparison still happened.
	if delta.Unchanged != len(dups) {
		t.Errorf("the delta should still be computed, got %s", DeltaSummary(delta))
	}
}

func asSnapshotMissing(err error, target **SnapshotMissingError) bool {
	if m, ok := err.(*SnapshotMissingError); ok {
		*target = m
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// delta rendering
// ---------------------------------------------------------------------------

func renderDelta(delta *Delta, dups ...Duplication) string {
	// CompareToSnapshot is what joins an entry to the finding it came from; these tests
	// build deltas by hand, so they do the same join here rather than reaching into the
	// printer.
	byHash := map[string]*Duplication{}
	for i := range dups {
		byHash[dups[i].Hash] = &dups[i]
	}
	for i := range delta.New {
		delta.New[i].Finding = byHash[delta.New[i].Hash]
	}
	for _, changes := range [][]EntryChange{delta.Spread, delta.Moved, delta.Shrunk, delta.Dropped} {
		for i := range changes {
			changes[i].Finding = byHash[changes[i].Now.Hash]
		}
	}

	var sb strings.Builder
	PrintDelta(&sb, delta, Stats{Files: 12}, Blinding{}, "dup.json")
	return sb.String()
}

// liveFinding is what the RUN holds for a hash: the code, and where each copy is. A snapshot
// holds none of that, which is why the delta reads its content from here.
func liveFinding(hash, snippet string, files ...string) Duplication {
	d := Duplication{
		Hash:             hash,
		Snippet:          snippet,
		FileCount:        len(files),
		Lines:            1 + strings.Count(snippet, "\n"),
		Depth:            2,
		Statements:       3,
		IsStatementBlock: true,
	}
	for i, f := range files {
		d.Occurrences = append(d.Occurrences, Occurrence{
			File: f, Line: 10 + i, Col: 3, EndLine: 10 + i, EndCol: 9,
		})
	}
	return d
}

func TestPrintDeltaClean(t *testing.T) {
	out := renderDelta(&Delta{Unchanged: 7})
	if !strings.Contains(out, "7 patterns acknowledged, nothing new") {
		t.Errorf("clean delta wrong:\n%s", out)
	}
	if !strings.Contains(out, "Scanned 12 files") {
		t.Errorf("scanned count missing:\n%s", out)
	}
	if strings.Contains(out, "--update-snapshot") {
		t.Errorf("a clean run should not ask for an update:\n%s", out)
	}
}

// TestPrintDeltaShowsTheCodeAndItsLocations is what a snapshot delta is for: a reader has to
// be able to see WHAT is duplicated, not just that a hash changed. The snapshot cannot
// supply that - it stores a hash, a file list and a one-line label - so every entry the run
// still has is rendered from the run.
func TestPrintDeltaShowsTheCodeAndItsLocations(t *testing.T) {
	snippet := "{\n  const total = items.reduce(sum, 0);\n  return total;\n}"
	delta := &Delta{
		New: []NewEntry{{SnapshotEntry: SnapshotEntry{
			Hash: "aaaa", Occurrences: 2, Files: map[string]int{"src/a.ts": 1, "src/b.ts": 1},
			Label: "const total = items.reduce(sum, 0);",
		}}},
		Unchanged: 4,
	}
	out := renderDelta(delta, liveFinding("aaaa", snippet, "src/a.ts", "src/b.ts"))

	for _, want := range []string{
		"new",
		// The shape of the block, exactly as the unfiltered report describes it.
		"aaaa  2 copies in 2 files  -  code block, 4 lines, depth 2, 3 statements",
		// Locations, with line and column - not just the file paths the snapshot stores.
		"src/a.ts:10:3",
		"src/b.ts:11:3",
		// And the code itself.
		"|   const total = items.reduce(sum, 0);",
		"|   return total;",
		"4 unchanged",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestPrintDeltaRegressions(t *testing.T) {
	delta := &Delta{
		New: []NewEntry{{SnapshotEntry: SnapshotEntry{
			Hash: "aaaa", Occurrences: 2, Files: map[string]int{"src/a.ts": 1, "src/b.ts": 1}, Label: "const x = 1;",
		}}},
		Spread: []EntryChange{{
			Was:         SnapshotEntry{Hash: "bbbb", Occurrences: 2},
			Now:         SnapshotEntry{Hash: "bbbb", Occurrences: 3, Label: "go()"},
			FileChanges: []FileChange{{Path: "src/c.ts", Was: 0, Now: 1}},
		}},
		Moved: []EntryChange{{
			Was: SnapshotEntry{Hash: "cccc", Occurrences: 2},
			Now: SnapshotEntry{Hash: "cccc", Occurrences: 2},
			FileChanges: []FileChange{
				{Path: "src/new.ts", Was: 0, Now: 1},
				{Path: "src/old.ts", Was: 1, Now: 0},
			},
		}},
		Unchanged: 4,
	}
	out := renderDelta(delta,
		liveFinding("aaaa", "{ const x = 1; }", "src/a.ts", "src/b.ts"),
		liveFinding("bbbb", "{ go() }", "src/a.ts", "src/b.ts", "src/c.ts"),
		liveFinding("cccc", "{ moved() }", "src/keep.ts", "src/new.ts"),
	)

	for _, want := range []string{
		"1 new, 1 spread further, 1 moved",
		"aaaa", "src/a.ts:10:3", "| { const x = 1; }",
		"bbbb  2 -> 3 copies", "spread", "moved",
		"4 unchanged",
		"Duplication increased. Fix it, or acknowledge it with --update-snapshot.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// TestPrintDeltaMarksWhichCopyIsNew: when a pattern spreads, the useful question is which
// copy is the new one. Marking the occurrence answers it in place; two file lists to diff by
// eye do not.
func TestPrintDeltaMarksWhichCopyIsNew(t *testing.T) {
	delta := &Delta{
		Spread: []EntryChange{{
			Was:         SnapshotEntry{Hash: "bbbb", Occurrences: 2},
			Now:         SnapshotEntry{Hash: "bbbb", Occurrences: 3},
			FileChanges: []FileChange{{Path: "src/c.ts", Was: 0, Now: 1}},
		}},
	}
	out := renderDelta(delta, liveFinding("bbbb", "{ go() }", "src/a.ts", "src/b.ts", "src/c.ts"))

	if !strings.Contains(out, "+ src/c.ts:12:3") {
		t.Errorf("the added copy is not marked at its location:\n%s", out)
	}
	if strings.Contains(out, "+ src/a.ts") || strings.Contains(out, "+ src/b.ts") {
		t.Errorf("an already-acknowledged copy is marked as new:\n%s", out)
	}
}

// A file a pattern LEFT has no occurrence to point at, so it is listed on its own.
func TestPrintDeltaListsFilesAPatternLeft(t *testing.T) {
	delta := &Delta{
		Moved: []EntryChange{{
			Was: SnapshotEntry{Hash: "cccc", Occurrences: 2},
			Now: SnapshotEntry{Hash: "cccc", Occurrences: 2},
			FileChanges: []FileChange{
				{Path: "src/new.ts", Was: 0, Now: 1},
				{Path: "src/old.ts", Was: 1, Now: 0},
			},
		}},
	}
	out := renderDelta(delta, liveFinding("cccc", "{ moved() }", "src/keep.ts", "src/new.ts"))

	if !strings.Contains(out, "- src/old.ts: 1 copy gone, none left here") {
		t.Errorf("the file the pattern left is not reported:\n%s", out)
	}
	if !strings.Contains(out, "+ src/new.ts:11:3") {
		t.Errorf("the file it moved into is not marked:\n%s", out)
	}
}

// A resolved entry is the one case the run cannot render: that code is gone. The snapshot's
// own record - hash, files, label - is all there is, and the wording has to admit it is past
// tense rather than claim the copies are still there.
func TestPrintDeltaResolvedEntryFallsBackToTheSnapshot(t *testing.T) {
	delta := &Delta{
		Resolved: []SnapshotEntry{{
			Hash: "dddd", Occurrences: 2, Files: map[string]int{"src/a.ts": 1, "src/b.ts": 1},
			Label: "const gone = true;",
		}},
	}
	// Deliberately no live finding for dddd: it no longer exists in the run.
	out := renderDelta(delta, liveFinding("aaaa", "{ other() }", "src/x.ts"))

	for _, want := range []string{
		"dddd  was 2 copies in 2 files",
		"src/a.ts",
		"| const gone = true;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "src/a.ts:") {
		t.Errorf("a resolved entry cannot have line numbers - that code is gone:\n%s", out)
	}
}

func TestPrintDeltaImprovementsWordedDifferently(t *testing.T) {
	delta := &Delta{
		Resolved:  []SnapshotEntry{{Hash: "dddd", Occurrences: 2, Files: map[string]int{"src/a.ts": 1}}},
		Unchanged: 1,
	}
	out := renderDelta(delta)

	if !strings.Contains(out, "resolved") {
		t.Errorf("resolved not reported:\n%s", out)
	}
	if !strings.Contains(out, "Duplication decreased, so the snapshot is out of date.") {
		t.Errorf("an improvement should not read as a regression:\n%s", out)
	}
	if strings.Contains(out, "Duplication increased") {
		t.Errorf("an improvement reported as an increase:\n%s", out)
	}
}

func TestPrintDeltaExplainsRevealedEntries(t *testing.T) {
	delta := &Delta{
		New: []NewEntry{{
			SnapshotEntry: SnapshotEntry{Hash: "aaaa", Occurrences: 2, Files: map[string]int{"src/a.ts": 1}},
			RevealedBy:    "parent01",
		}},
		Resolved: []SnapshotEntry{{Hash: "parent01", Occurrences: 2, Files: map[string]int{"src/a.ts": 1}}},
	}
	out := renderDelta(delta)
	if !strings.Contains(out, "was nested inside parent01") {
		t.Errorf("a revealed entry should say where it came from:\n%s", out)
	}
	if !strings.Contains(out, "this code did not change") {
		t.Errorf("a revealed entry should not read as newly written code:\n%s", out)
	}
}

func TestPrintDeltaContextLines(t *testing.T) {
	out := renderDelta(&Delta{
		Unchanged:            2,
		CanonicalFormChanged: true,
		SnapshotFormVersion:  "0.9",
		ParameterChanges:     []string{"minTokens 150 -> 250"},
	})
	if !strings.Contains(out, "canonical form changed") && !strings.Contains(out, "canonical form") {
		t.Errorf("a normaliser change should be explained:\n%s", out)
	}
	if !strings.Contains(out, "minTokens 150 -> 250") {
		t.Errorf("parameter changes should be reported as context:\n%s", out)
	}
}

func TestDeltaSummaryWording(t *testing.T) {
	if got := DeltaSummary(&Delta{Unchanged: 3}); !strings.Contains(got, "nothing new") {
		t.Errorf("clean summary = %q", got)
	}
	got := DeltaSummary(&Delta{
		New:       []NewEntry{{}, {}},
		Spread:    []EntryChange{{}},
		Resolved:  []SnapshotEntry{{}},
		Unchanged: 5,
	})
	for _, want := range []string{"2 new", "1 spread further", "1 resolved", "5 unchanged"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
}

// The snapshot stores a count per file, not a list of paths, and these are the two things
// that buys. Line positions are still not stored and cannot be: they move whenever anything
// above them is edited, and the copies are identical by construction so none of them is "the
// new one" anyway.

// A copy added to a file the snapshot ALREADY listed moves the total and nothing else. With
// only a path list that is invisible - the report says "2 -> 3 copies" over three locations
// and leaves the reader to work out which file gained one.
func TestDeltaNamesTheFileThatGainedACopy(t *testing.T) {
	before := SnapshotEntry{Hash: "h", Occurrences: 2, Files: map[string]int{"a.ts": 1, "b.ts": 1}}
	after := SnapshotEntry{Hash: "h", Occurrences: 3, Files: map[string]int{"a.ts": 2, "b.ts": 1}}

	changes := diffFiles(before.Files, after.Files)
	if len(changes) != 1 {
		t.Fatalf("expected one file to have changed, got %+v", changes)
	}
	got := changes[0]
	if got.Path != "a.ts" || got.Was != 1 || got.Now != 2 {
		t.Errorf("file change = %+v, want a.ts 1 -> 2", got)
	}
	// Not a new file, so no single occurrence in it can be pointed at as the new one.
	if got.IsNew() || got.IsGone() {
		t.Errorf("a file that gained a copy is neither new nor gone: %+v", got)
	}

	out := renderDelta(&Delta{Spread: []EntryChange{{Was: before, Now: after, FileChanges: changes}}},
		liveFinding("h", "{ go() }", "a.ts", "a.ts", "b.ts"))
	if !strings.Contains(out, "+ a.ts: 1 -> 2 copies here") {
		t.Errorf("the file that gained a copy is not named:\n%s", out)
	}
	// Marking a line would be a guess: both copies are identical and either could be the
	// one that was there before. The summary line above names the file; no LOCATION in it
	// may carry the marker.
	for _, loc := range []string{"+ a.ts:10:3", "+ a.ts:11:3"} {
		if strings.Contains(out, loc) {
			t.Errorf("an arbitrary occurrence was marked as the new one:\n%s", out)
		}
	}
}

// A file that held several copies and lost all of them says how many, rather than reporting
// the same "no longer here" a single copy would.
func TestDeltaSaysHowManyCopiesAFileLost(t *testing.T) {
	before := SnapshotEntry{Hash: "h", Occurrences: 4, Files: map[string]int{"keep.ts": 2, "gone.ts": 2}}
	after := SnapshotEntry{Hash: "h", Occurrences: 2, Files: map[string]int{"keep.ts": 2}}

	out := renderDelta(
		&Delta{Shrunk: []EntryChange{{Was: before, Now: after, FileChanges: diffFiles(before.Files, after.Files)}}},
		liveFinding("h", "{ go() }", "keep.ts", "keep.ts"))

	if !strings.Contains(out, "- gone.ts: 2 copies gone, none left here") {
		t.Errorf("the number of copies lost is not stated:\n%s", out)
	}
}

// A file the snapshot never listed is the one case where every occurrence in it IS new, so
// those are still marked exactly, at the line.
func TestDeltaStillMarksOccurrencesInAWhollyNewFile(t *testing.T) {
	before := SnapshotEntry{Hash: "h", Occurrences: 1, Files: map[string]int{"a.ts": 1}}
	after := SnapshotEntry{Hash: "h", Occurrences: 2, Files: map[string]int{"a.ts": 1, "new.ts": 1}}

	out := renderDelta(
		&Delta{Spread: []EntryChange{{Was: before, Now: after, FileChanges: diffFiles(before.Files, after.Files)}}},
		liveFinding("h", "{ go() }", "a.ts", "new.ts"))

	if !strings.Contains(out, "+ new.ts:11:3") {
		t.Errorf("a copy in a wholly new file must be pointed at directly:\n%s", out)
	}
	// And not repeated as a per-file summary, which would say the same thing twice.
	if strings.Contains(out, "new.ts: 0 -> 1") {
		t.Errorf("a new file was reported twice:\n%s", out)
	}
}

// A file whose count did not move is not a change and must not be marked.
func TestDeltaLeavesUnchangedFilesAlone(t *testing.T) {
	same := map[string]int{"a.ts": 2, "b.ts": 1}
	if changes := diffFiles(same, map[string]int{"a.ts": 2, "b.ts": 1}); len(changes) != 0 {
		t.Errorf("unchanged files were reported as changes: %+v", changes)
	}
}

// ---------------------------------------------------------------------------
// verdicts
// ---------------------------------------------------------------------------

// The emoji are not decoration - each one states which way duplication moved, and the report
// is read fastest by scanning for them. So they have to follow the verdict exactly: a green
// tick on a run that got worse is worse than no tick at all.

func newEntry(hash string, files map[string]int) NewEntry {
	total := 0
	for _, n := range files {
		total += n
	}
	return NewEntry{SnapshotEntry: SnapshotEntry{Hash: hash, Occurrences: total, Files: files}}
}

func TestDeltaVerdictFollowsWhichWayDuplicationMoved(t *testing.T) {
	oneFile := map[string]int{"a.ts": 1}
	cases := []struct {
		name     string
		delta    *Delta
		want     string
		mustNot  []string
		contains []string
	}{
		{
			name:     "clean",
			delta:    &Delta{Unchanged: 3},
			want:     emoji.Success,
			contains: []string{"matches the snapshot"},
			mustNot:  []string{emoji.Error, emoji.Warning},
		},
		{
			name:     "only more",
			delta:    &Delta{New: []NewEntry{newEntry("a", oneFile)}},
			want:     emoji.Error,
			contains: []string{"Duplication increased"},
			mustNot:  []string{"Duplication decreased", "moved to other files"},
		},
		{
			name:     "only less",
			delta:    &Delta{Resolved: []SnapshotEntry{{Hash: "b", Occurrences: 2, Files: oneFile}}},
			want:     emoji.Success,
			contains: []string{"Duplication decreased"},
			mustNot:  []string{"Duplication increased"},
		},
		{
			// The shape a real commit usually has: some fixed, some added. Reporting only
			// the increase hides work that was done and reads as a rejection of it.
			name: "both directions",
			delta: &Delta{
				New:      []NewEntry{newEntry("a", oneFile)},
				Resolved: []SnapshotEntry{{Hash: "b", Occurrences: 2, Files: oneFile}},
			},
			want:     emoji.Error,
			contains: []string{"Duplication increased", "Some was also resolved"},
		},
		{
			// Same amount of duplication, different files. Calling that an increase names a
			// change that did not happen.
			name: "moved only",
			delta: &Delta{Moved: []EntryChange{{
				Was: SnapshotEntry{Hash: "c", Occurrences: 2, Files: map[string]int{"old.ts": 2}},
				Now: SnapshotEntry{Hash: "c", Occurrences: 2, Files: map[string]int{"new.ts": 2}},
			}}},
			want:     emoji.Warning,
			contains: []string{"moved to other files", "Nothing was added"},
			mustNot:  []string{"Duplication increased"},
		},
	}

	for _, tc := range cases {
		out := renderDelta(tc.delta)
		verdict := lastVerdictLine(out)
		if !strings.Contains(verdict, tc.want) {
			t.Errorf("%s: verdict %q does not carry %s\n%s", tc.name, verdict, tc.want, out)
		}
		for _, want := range tc.contains {
			if !strings.Contains(out, want) {
				t.Errorf("%s: missing %q in:\n%s", tc.name, want, out)
			}
		}
		for _, unwanted := range tc.mustNot {
			if strings.Contains(out, unwanted) {
				t.Errorf("%s: says %q when it should not:\n%s", tc.name, unwanted, out)
			}
		}
	}
}

// lastVerdictLine returns the first line of the closing verdict, which is the one a reader
// looks at to decide whether anything is wrong.
func lastVerdictLine(out string) string {
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.Contains(line, "Duplication increased"),
			strings.Contains(line, "Duplication decreased"),
			strings.Contains(line, "moved to other files"),
			strings.Contains(line, "matches the snapshot"):
			return line
		}
	}
	return ""
}

// A move fails the check - a baseline naming the wrong files cannot be reviewed - but it is
// not more duplication, and the two questions have to stay separable.
func TestMovedFailsWithoutCountingAsMoreDuplication(t *testing.T) {
	delta := &Delta{Moved: []EntryChange{{
		Was: SnapshotEntry{Hash: "c", Occurrences: 2},
		Now: SnapshotEntry{Hash: "c", Occurrences: 2},
	}}}
	if !delta.HasRegressions() {
		t.Error("a move must still fail the check")
	}
	if delta.HasMoreDuplication() {
		t.Error("a move is not more duplication")
	}
	if delta.IsClean() {
		t.Error("a move is not a clean run")
	}
}

// Every entry names its own kind and carries its own verdict, so the list can be read
// top-down without a heading above each group repeating what the line already says.
func TestDeltaEntriesCarryTheirKindAndVerdict(t *testing.T) {
	oneFile := map[string]int{"a.ts": 1}
	out := renderDelta(&Delta{
		New:      []NewEntry{newEntry("a", oneFile)},
		Resolved: []SnapshotEntry{{Hash: "b", Occurrences: 2, Files: oneFile}},
	})
	for _, want := range []string{
		emoji.Error + "    new        a  ",
		emoji.Success + "    resolved   b  ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing entry line %q in:\n%s", want, out)
		}
	}
	// The old per-category headings are gone; the summary at the top says the same thing
	// once.
	if strings.Contains(out, "NOT in the snapshot:") {
		t.Errorf("a category heading survived the flattening:\n%s", out)
	}
}

// Worst first, so the list can be abandoned at the point the reader stops caring.
func TestDeltaEntriesAreOrderedWorstFirst(t *testing.T) {
	oneFile := map[string]int{"a.ts": 1}
	out := renderDelta(&Delta{
		Resolved: []SnapshotEntry{{Hash: "rrrr", Occurrences: 2, Files: oneFile}},
		New:      []NewEntry{newEntry("nnnn", oneFile)},
		Moved: []EntryChange{{
			Was: SnapshotEntry{Hash: "mmmm", Occurrences: 2, Files: map[string]int{"old.ts": 2}},
			Now: SnapshotEntry{Hash: "mmmm", Occurrences: 2, Files: map[string]int{"new.ts": 2}},
		}},
	})
	order := []string{"new", "moved", "resolved"}
	at := make([]int, len(order))
	for i, kind := range order {
		at[i] = strings.Index(out, "  "+kind+" ")
		if at[i] < 0 {
			t.Fatalf("%q is not in the report:\n%s", kind, out)
		}
	}
	for i := 1; i < len(at); i++ {
		if at[i] < at[i-1] {
			t.Errorf("%q comes before %q; the list is not ordered worst first:\n%s",
				order[i], order[i-1], out)
		}
	}
}

// An entry's own verdict is not the section's. A pattern filed under "copied into more
// places" on its net count can have left a file entirely on the way - part worse, part
// better - and a reader deciding what to do next needs both halves. The file lines carry
// them, but only for someone who reads every one; the entry header is what gets scanned.
func TestEntryMarkerReflectsTheEntryNotItsSection(t *testing.T) {
	cases := []struct {
		name    string
		changes []FileChange
		want    string
		mustNot string
	}{
		{
			name:    "gained only",
			changes: []FileChange{{Path: "new.ts", Was: 0, Now: 1}},
			want:    emoji.Error,
			mustNot: emoji.Success,
		},
		{
			name:    "lost only",
			changes: []FileChange{{Path: "gone.ts", Was: 2, Now: 0}},
			want:    emoji.Success,
			mustNot: emoji.Error,
		},
		{
			// The case that was invisible: net worse, but a file was cleaned out.
			name: "gained in one file, lost another",
			changes: []FileChange{
				{Path: "new.ts", Was: 0, Now: 2},
				{Path: "gone.ts", Was: 1, Now: 0},
			},
			want: emoji.Error + emoji.Success,
		},
		{
			// A count moving inside files that were already listed counts too.
			name:    "same files, more copies",
			changes: []FileChange{{Path: "a.ts", Was: 1, Now: 3}},
			want:    emoji.Error,
			mustNot: emoji.Success,
		},
	}
	for _, tc := range cases {
		got := entryMarker(tc.changes)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: marker %q is missing %s", tc.name, got, tc.want)
		}
		if tc.mustNot != "" && strings.Contains(got, tc.mustNot) {
			t.Errorf("%s: marker %q wrongly carries %s", tc.name, got, tc.mustNot)
		}
	}
}

// Markers of different widths sit in one column, so the hashes after them have to line up or
// the report stops being scannable - which was the only reason to put them there.
func TestEntryMarkersAreColumnAligned(t *testing.T) {
	widths := map[string]int{}
	for _, m := range []string{
		entryMarker([]FileChange{{Path: "a", Was: 0, Now: 1}}),                              // one glyph
		entryMarker([]FileChange{{Path: "a", Was: 1, Now: 0}}),                              // one glyph
		entryMarker([]FileChange{{Path: "a", Was: 0, Now: 1}, {Path: "b", Was: 1, Now: 0}}), // two
		entryMarker(nil), // none
	} {
		// Emoji are two cells wide; padding makes up the difference.
		cells := 0
		for _, r := range m {
			if r == ' ' {
				cells++
				continue
			}
			// Variation selectors and zero-width joiners add no width.
			if r == '️' || r == '‍' {
				continue
			}
			cells += 2
		}
		widths[m] = cells
	}
	var want int
	for m, cells := range widths {
		if want == 0 {
			want = cells
			continue
		}
		if cells != want {
			t.Errorf("marker %q is %d cells wide, others are %d - the hash column will not line up",
				m, cells, want)
		}
	}
}

// The mixed marker has to survive into the rendered report, not just the helper.
func TestMixedEntryIsMarkedInTheReport(t *testing.T) {
	before := SnapshotEntry{Hash: "h", Occurrences: 3, Files: map[string]int{"gone.ts": 2, "keep.ts": 1}}
	after := SnapshotEntry{Hash: "h", Occurrences: 4, Files: map[string]int{"new.ts": 3, "keep.ts": 1}}

	out := renderDelta(
		&Delta{Spread: []EntryChange{{Was: before, Now: after, FileChanges: diffFiles(before.Files, after.Files)}}},
		liveFinding("h", "{ go() }", "keep.ts", "new.ts", "new.ts", "new.ts"))

	if !strings.Contains(out, emoji.Error+emoji.Success+"  spread     h") {
		t.Errorf("an entry that both gained and lost is not marked as both:\n%s", out)
	}
}
