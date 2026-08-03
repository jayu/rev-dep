package dupcode

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// renderJSON runs a detection and returns the report parsed back, which is how a consumer
// sees it rather than how it was built.
func renderJSON(t *testing.T, opts Options, snippets bool) JSONReport {
	t.Helper()
	dups, stats, err := Detect(opts)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, dups, stats, opts, snippets); err != nil {
		t.Fatal(err)
	}
	var got JSONReport
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	return got
}

// TestJSONSettingsRecordEveryOptionThatChangesTheResult is the guard that --skip-objects
// needed and did not have: a setting can be added, threaded through and honoured by the
// detector while never appearing in the report, and then two runs that produced different
// numbers look identical to whatever is reading them.
//
// It fails when a field is added to Options without a decision about the report.
func TestJSONSettingsRecordEveryOptionThatChangesTheResult(t *testing.T) {
	// Fields that steer the run rather than decide what qualifies as a duplication.
	notASetting := map[string]string{
		"Cwd":        "published at the top level",
		"Files":      "the caller's file list, not a threshold",
		"NeedHashes": "decides what is computed, not what qualifies",
		"CountOnly":  "drops presentation, not findings",
		"trackBelowThreshold": "returns what did NOT qualify, on a second result JSON " +
			"does not publish; it cannot change the findings themselves",
		"SnapshotPath": "where the baseline lives, not what qualifies as a finding",
	}

	recorded := map[string]bool{}
	st := reflect.TypeOf(JSONSettings{})
	for i := 0; i < st.NumField(); i++ {
		recorded[st.Field(i).Name] = true
	}
	// ConfigLevel is published as its own object, since its patterns are relative to the
	// config rather than to cwd.
	recorded["ConfigLevel"] = true
	// Blinding is published as a label plus one boolean per axis.
	recorded["Blinding"] = recorded["BlindIdentifiers"] && recorded["BlindStrings"] &&
		recorded["BlindNumbers"] && recorded["Blinding"]

	ot := reflect.TypeOf(Options{})
	for i := 0; i < ot.NumField(); i++ {
		name := ot.Field(i).Name
		if _, ok := notASetting[name]; ok || recorded[name] {
			continue
		}
		t.Errorf("Options.%s changes what is reported but does not appear in "+
			"JSONSettings; record it or add it to notASetting with a reason", name)
	}
}

func TestJSONSettingsMirrorTheOptionsUsed(t *testing.T) {
	got := renderJSON(t, Options{
		Cwd:                 fixtureProject,
		Blinding:            Blinding{Identifiers: true, Numbers: true},
		MinTokens:           10,
		MinLines:            1,
		MinDepth:            2,
		MinStatements:       1,
		MinDuplicates:       2,
		SkipObjects:         true,
		IgnoreFiles:         []string{"src/gen/**"},
		ProcessIgnoredFiles: []string{"dist/**"},
		NeedHashes:          true,
	}, false)

	s := got.Settings
	checks := []struct {
		field string
		ok    bool
	}{
		{"blinding", s.Blinding == "identifiers+numbers"},
		{"blindIdentifiers", s.BlindIdentifiers},
		{"blindStrings", !s.BlindStrings},
		{"blindNumbers", s.BlindNumbers},
		{"minTokens", s.MinTokens == 10},
		{"minLines", s.MinLines == 1},
		{"minDepth", s.MinDepth == 2},
		{"minStatements", s.MinStatements == 1},
		{"minDuplicates", s.MinDuplicates == 2},
		{"skipObjects", s.SkipObjects},
		{"ignoreFiles", len(s.IgnoreFiles) == 1 && s.IgnoreFiles[0] == "src/gen/**"},
		{"processIgnoredFiles", len(s.ProcessIgnoredFiles) == 1 && s.ProcessIgnoredFiles[0] == "dist/**"},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("settings.%s does not reflect the run: %+v", c.field, s)
		}
	}
	if got.Cwd != fixtureProject {
		t.Errorf("cwd = %q, want %q", got.Cwd, fixtureProject)
	}
}

// TestJSONBlindingLabelAgreesWithItsBooleans pins the one piece of redundancy in the
// report, since a consumer may key on either.
func TestJSONBlindingLabelAgreesWithItsBooleans(t *testing.T) {
	for _, b := range []Blinding{
		{},
		{Identifiers: true},
		{Strings: true, Numbers: true},
		{Identifiers: true, Strings: true, Numbers: true},
	} {
		got := renderJSON(t, Options{Cwd: fixtureProject, Blinding: b, MinTokens: 10, MinLines: 1}, false)
		s := got.Settings
		if s.Blinding != b.String() {
			t.Errorf("label %q does not name %+v", s.Blinding, b)
		}
		if s.BlindIdentifiers != b.Identifiers || s.BlindStrings != b.Strings || s.BlindNumbers != b.Numbers {
			t.Errorf("booleans %+v disagree with the label %q", s, s.Blinding)
		}
	}
}

// TestJSONFindingsCarryWhatAComparisonNeeds covers the reason the format exists: another
// tool's output has to be lined up against this one without reading console text.
func TestJSONFindingsCarryWhatAComparisonNeeds(t *testing.T) {
	got := renderJSON(t, Options{
		Cwd: fixtureProject, Blinding: Blinding{}, MinTokens: 10, MinLines: 1, NeedHashes: true,
	}, true)

	if len(got.Findings) == 0 {
		t.Fatal("fixture produced no findings")
	}
	if got.Summary.Findings != len(got.Findings) {
		t.Errorf("summary says %d findings, array holds %d", got.Summary.Findings, len(got.Findings))
	}

	occurrences, files := 0, map[string]bool{}
	for _, f := range got.Findings {
		if f.Hash == "" {
			t.Errorf("finding has no hash: %+v", f)
		}
		if f.Kind == "" || f.Role == "" {
			t.Errorf("finding is not classified: %+v", f)
		}
		if f.Tokens <= 0 || f.Lines <= 0 || f.Depth <= 0 {
			t.Errorf("finding has no metrics to attribute a threshold to: %+v", f)
		}
		if f.Snippet == "" {
			t.Errorf("--json-snippets was on but the finding carries no source: %+v", f)
		}
		if len(f.Occurrences) < 2 {
			t.Errorf("a duplication needs at least two occurrences: %+v", f)
		}
		occurrences += len(f.Occurrences)
		for _, o := range f.Occurrences {
			files[o.File] = true
			if o.EndByte <= o.StartByte {
				t.Errorf("byte range is empty or reversed: %+v", o)
			}
			if o.StartLine < 1 || o.StartCol < 1 || o.EndLine < o.StartLine {
				t.Errorf("line range is not 1-based and ordered: %+v", o)
			}
		}
	}
	if got.Summary.Occurrences != occurrences {
		t.Errorf("summary says %d occurrences, findings hold %d", got.Summary.Occurrences, occurrences)
	}
	if got.Summary.FilesWithFindings != len(files) {
		t.Errorf("summary says %d files with findings, occurrences name %d",
			got.Summary.FilesWithFindings, len(files))
	}
	if got.Summary.FilesScanned < got.Summary.FilesWithFindings {
		t.Errorf("scanned %d files but found duplicates in %d",
			got.Summary.FilesScanned, got.Summary.FilesWithFindings)
	}
	composition := got.Summary.StatementBlocks + got.Summary.ObjectLiterals +
		got.Summary.JSXBlocks + got.Summary.Other
	if composition != len(got.Findings) {
		t.Errorf("composition sums to %d, want %d findings", composition, len(got.Findings))
	}
}

// TestJSONOmitsSnippetsByDefault keeps the format usable at corpus scale: on a real project
// the snippets are most of the bytes, and a comparison keyed on hashes and ranges never
// reads them.
func TestJSONOmitsSnippetsByDefault(t *testing.T) {
	got := renderJSON(t, Options{Cwd: fixtureProject, Blinding: Blinding{}, MinTokens: 10, MinLines: 1}, false)
	for _, f := range got.Findings {
		if f.Snippet != "" {
			t.Errorf("snippet written without --json-snippets: %+v", f)
		}
	}
}

// TestJSONDoesNotEscapeSourceCode guards the same thing the snapshot does: JSX in a
// snippet arriving as \u003c makes the output unreadable to a human reviewer and forces
// every consumer to decode it back.
func TestJSONDoesNotEscapeSourceCode(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{Cwd: fixtureProject, Blinding: Blinding{}, MinTokens: 10, MinLines: 1}
	dups, stats, err := Detect(opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(&buf, dups, stats, opts, true); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `\u003c`) {
		t.Errorf("source code is HTML-escaped:\n%s", buf.String())
	}
}

// TestJSONEmptyResultIsStillAnArray matters to anything consuming the report: a null where
// an array was expected is a crash rather than zero findings.
func TestJSONEmptyResultIsStillAnArray(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, nil, Stats{}, Options{Cwd: "/w", Blinding: Blinding{}}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"findings": []`) {
		t.Errorf("empty findings are not an array:\n%s", buf.String())
	}
	var got JSONReport
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Findings == nil || len(got.Findings) != 0 {
		t.Errorf("findings = %#v, want an empty array", got.Findings)
	}
	// Defaults must be reported as the values actually used, not as the zeros the caller
	// happened to pass, or the report would not describe the run.
	if got.Settings.MinTokens != DefaultMinTokens || got.Settings.MinLines != DefaultMinLines {
		t.Errorf("defaults not resolved in the report: %+v", got.Settings)
	}
}

// ---------------------------------------------------------------------------
// snapshot-aware JSON
// ---------------------------------------------------------------------------

// With a baseline, JSON labels every finding instead of filtering to the change.
//
// Filtering would be the obvious mirror of the human report, and it is the wrong call here:
// the delta is recoverable from a labelled list by filtering on status, but the total is not
// recoverable from a filtered one. Anything tracking how much duplication a project carries
// would silently start reading the size of the last commit's diff instead.

func snapshotJSON(t *testing.T, dups, below []Duplication, delta *Delta, opts Options) JSONReport {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteJSONWithSnapshot(&buf, dups, below, delta, Stats{}, opts, "dups.json", false); err != nil {
		t.Fatal(err)
	}
	var got JSONReport
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	return got
}

func jsonFindingFixture(hash string, files ...string) Duplication {
	d := Duplication{Hash: hash, Tokens: 20, Lines: 4, Depth: 2, NormLen: 40}
	for i, f := range files {
		d.Occurrences = append(d.Occurrences, Occurrence{File: f, Line: 10 + i, Col: 1})
	}
	d.FileCount = len(files)
	return d
}

func TestJSONLabelsEveryFindingAgainstTheSnapshot(t *testing.T) {
	unchanged := jsonFindingFixture("uuuu", "keep.ts", "keep2.ts")
	spread := jsonFindingFixture("ssss", "a.ts", "b.ts", "c.ts")

	delta := &Delta{
		Unchanged: 1,
		Spread: []EntryChange{{
			Was: SnapshotEntry{Hash: "ssss", Occurrences: 2, Files: map[string]int{"a.ts": 1, "b.ts": 1}},
			Now: SnapshotEntry{Hash: "ssss", Occurrences: 3,
				Files: map[string]int{"a.ts": 1, "b.ts": 1, "c.ts": 1}},
			FileChanges: []FileChange{{Path: "c.ts", Was: 0, Now: 1}},
		}},
	}
	got := snapshotJSON(t, []Duplication{unchanged, spread}, nil, delta, Options{Cwd: "/w"})

	// Nothing was dropped: both findings are here.
	if len(got.Findings) != 2 {
		t.Fatalf("findings = %d, want both of them - the list must not be filtered", len(got.Findings))
	}
	byHash := map[string]JSONFinding{}
	for _, f := range got.Findings {
		byHash[f.Hash] = f
	}
	if byHash["uuuu"].Status != "unchanged" {
		t.Errorf("an acknowledged finding is labelled %q, want unchanged", byHash["uuuu"].Status)
	}
	s := byHash["ssss"]
	if s.Status != "spread" {
		t.Errorf("status = %q, want spread", s.Status)
	}
	if s.Was == nil || s.Was.Occurrences != 2 {
		t.Errorf("the acknowledged state is missing: %+v", s.Was)
	}
	if len(s.FileChanges) != 1 || s.FileChanges[0].Path != "c.ts" || s.FileChanges[0].Now != 1 {
		t.Errorf("the file that gained a copy is not reported: %+v", s.FileChanges)
	}
	if got.Snapshot == nil || got.Snapshot.Clean {
		t.Errorf("the snapshot block is missing or claims a clean run: %+v", got.Snapshot)
	}
	if got.Snapshot.Counts.Spread != 1 || got.Snapshot.Counts.Unchanged != 1 {
		t.Errorf("counts do not match the delta: %+v", got.Snapshot.Counts)
	}
}

// Resolved entries have no code left, so they cannot be findings - there are no occurrences,
// no metrics and no snippet to report. They go beside the findings rather than among them.
func TestJSONReportsResolvedEntriesSeparately(t *testing.T) {
	delta := &Delta{Resolved: []SnapshotEntry{{
		Hash: "rrrr", Occurrences: 2, Files: map[string]int{"gone.ts": 2}, Label: "const x = 1;",
	}}}
	got := snapshotJSON(t, nil, nil, delta, Options{Cwd: "/w"})

	if len(got.Findings) != 0 {
		t.Errorf("a resolved entry was reported as a finding: %+v", got.Findings)
	}
	if len(got.Snapshot.Resolved) != 1 {
		t.Fatalf("resolved = %d, want 1", len(got.Snapshot.Resolved))
	}
	r := got.Snapshot.Resolved[0]
	if r.Hash != "rrrr" || r.Files["gone.ts"] != 2 {
		t.Errorf("the acknowledged record was not carried over: %+v", r)
	}
}

// Below-threshold entries DO have code, which is what separates them from resolved ones -
// and why they are worth reporting at all. They are still not findings, because the run did
// not report them.
func TestJSONReportsBelowThresholdEntriesWithTheirCode(t *testing.T) {
	below := jsonFindingFixture("bbbb", "a.ts", "b.ts")
	delta := &Delta{Dropped: []EntryChange{{
		Was:         SnapshotEntry{Hash: "bbbb", Occurrences: 3, Files: map[string]int{"a.ts": 1, "b.ts": 1, "c.ts": 1}},
		Now:         SnapshotEntry{Hash: "bbbb", Occurrences: 2, Files: map[string]int{"a.ts": 1, "b.ts": 1}},
		FileChanges: []FileChange{{Path: "c.ts", Was: 1, Now: 0}},
	}}}
	got := snapshotJSON(t, nil, []Duplication{below}, delta, Options{Cwd: "/w"})

	if len(got.Findings) != 0 {
		t.Errorf("a below-threshold entry was counted as a finding: %+v", got.Findings)
	}
	if len(got.Snapshot.BelowThreshold) != 1 {
		t.Fatalf("belowThreshold = %d, want 1", len(got.Snapshot.BelowThreshold))
	}
	b := got.Snapshot.BelowThreshold[0]
	if b.Status != "under-min" {
		t.Errorf("status = %q, want under-min", b.Status)
	}
	if len(b.Occurrences) != 2 {
		t.Errorf("the surviving copies are not located: %+v", b.Occurrences)
	}
	if got.Snapshot.Counts.BelowThreshold != 1 {
		t.Errorf("counts do not mention it: %+v", got.Snapshot.Counts)
	}
}

// A run with no snapshot must be byte-for-byte what it always was, so nothing already
// reading this output has to care that the feature exists.
func TestJSONWithoutASnapshotIsUnchanged(t *testing.T) {
	got := snapshotJSON(t, []Duplication{jsonFindingFixture("aaaa", "a.ts", "b.ts")}, nil, nil,
		Options{Cwd: "/w"})
	if got.Snapshot != nil {
		t.Errorf("a run with no snapshot reported one: %+v", got.Snapshot)
	}
	if got.Findings[0].Status != "" {
		t.Errorf("a finding carries a status with nothing to compare against: %q", got.Findings[0].Status)
	}
}

// The statuses are the same words the human report prints, so the two cannot describe the
// same run differently.
func TestJSONStatusesMatchTheHumanKinds(t *testing.T) {
	for _, kind := range []string{kindNew, kindSpread, kindMoved, kindFewer, kindUnderMin} {
		if kind == "" {
			t.Error("a kind is empty")
		}
	}
	delta := &Delta{
		New:    []NewEntry{{SnapshotEntry: SnapshotEntry{Hash: "nnnn", Occurrences: 2}}},
		Moved:  []EntryChange{{Was: SnapshotEntry{Hash: "mmmm"}, Now: SnapshotEntry{Hash: "mmmm"}}},
		Shrunk: []EntryChange{{Was: SnapshotEntry{Hash: "ffff"}, Now: SnapshotEntry{Hash: "ffff"}}},
	}
	statuses := findingStatuses(delta)
	for hash, want := range map[string]string{"nnnn": kindNew, "mmmm": kindMoved, "ffff": kindFewer} {
		if got := statuses[hash].status; got != want {
			t.Errorf("%s: status = %q, want %q", hash, got, want)
		}
	}
}
