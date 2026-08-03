package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/config"
	"rev-dep-go/internal/dupcode"
)

// dupRuleResult builds a rule result carrying one duplicated-code detection, which is all
// the two machine formats read.
func dupRuleResult(detections ...config.DuplicatedCodeRuleResult) config.RuleResult {
	return config.RuleResult{
		RulePath:       ".",
		EnabledChecks:  []string{"duplicated-code"},
		DuplicatedCode: detections,
	}
}

// withDuplication always fails without a snapshot - there is no tolerated count.
func withDuplication() config.DuplicatedCodeRuleResult {
	return config.DuplicatedCodeRuleResult{
		Result: checks.DuplicatedCodeResult{
			Snippets: 12, Occurrences: 30, Files: 8, Blinding: "identifiers",
			Scanned: 120, Ignored: 4,
		},
		Command: "rev-dep duplicated-code --blind-identifiers --min-tokens 60",
	}
}

// clean is the only way to pass without a snapshot.
func clean() config.DuplicatedCodeRuleResult {
	return config.DuplicatedCodeRuleResult{
		Result: checks.DuplicatedCodeResult{
			Snippets: 0, Occurrences: 0, Files: 0, Blinding: "exact", Scanned: 120,
		},
		Command: "rev-dep duplicated-code",
	}
}

func buildDupJSON(t *testing.T, rr config.RuleResult) jsonRuleResult {
	t.Helper()
	return buildJSONRuleResult(rr, "/work", nil)
}

// TestJSONOutputReportsDuplicatedCode is the gap this closes: the check ran, the console
// printed it and the exit code reflected it, but both machine formats dropped it entirely.
func TestJSONOutputReportsDuplicatedCode(t *testing.T) {
	got := buildDupJSON(t, dupRuleResult(withDuplication()))

	if len(got.Checks.DuplicatedCode) != 1 {
		t.Fatalf("duplicated code missing from JSON output: %+v", got.Checks)
	}
	d := got.Checks.DuplicatedCode[0]

	assertions := []struct {
		field string
		ok    bool
	}{
		{"status", d.Status == "fail"},
		{"blinding", d.Blinding == "identifiers"},
		{"snippets", d.Snippets == 12},
		{"occurrences", d.Occurrences == 30},
		{"files", d.Files == 8},
		{"scanned", d.Scanned == 120},
		{"ignored", d.Ignored == 4},
		{"command", strings.HasPrefix(d.Command, "rev-dep duplicated-code")},
		{"snapshot", d.Snapshot == nil},
	}
	for _, c := range assertions {
		if !c.ok {
			t.Errorf("%s is wrong: %+v", c.field, d)
		}
	}
}

// TestJSONOutputReportsCountsNotPlaces is the point of the shape: a real project holds
// thousands of duplicated blocks and the report must not grow with them.
func TestJSONOutputReportsCountsNotPlaces(t *testing.T) {
	raw, err := json.Marshal(buildDupJSON(t, dupRuleResult(withDuplication())))
	if err != nil {
		t.Fatal(err)
	}
	// No issues array, and nothing that could hold a file path or a snippet.
	for _, forbidden := range []string{`"issues"`, `"occurrencesList"`, `"snippet"`, `"filePath"`} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("duplicated code output lists places, not just counts (%s):\n%s",
				forbidden, raw)
		}
	}
}

func TestJSONOutputPassingDetectionIsStillReported(t *testing.T) {
	got := buildDupJSON(t, dupRuleResult(clean()))
	if len(got.Checks.DuplicatedCode) != 1 {
		t.Fatalf("a passing detection was dropped: %+v", got.Checks)
	}
	if got.Checks.DuplicatedCode[0].Status != "pass" {
		t.Errorf("status = %q, want pass", got.Checks.DuplicatedCode[0].Status)
	}
}

// TestJSONOutputReportsEveryDetection covers a workspace configured with several, which is
// why the field is an array.
func TestJSONOutputReportsEveryDetection(t *testing.T) {
	got := buildDupJSON(t, dupRuleResult(clean(), withDuplication()))
	if len(got.Checks.DuplicatedCode) != 2 {
		t.Fatalf("expected both detections, got %d", len(got.Checks.DuplicatedCode))
	}
	if got.Checks.DuplicatedCode[0].Blinding != "exact" ||
		got.Checks.DuplicatedCode[1].Blinding != "identifiers" {
		t.Errorf("order does not follow the config: %+v", got.Checks.DuplicatedCode)
	}
}

func TestJSONOutputCarriesTheSnapshotDelta(t *testing.T) {
	rr := withDuplication()
	rr.Result.SnapshotPath = ".rev-dep/duplicates.json"
	rr.Result.Delta = &dupcode.Delta{
		New:                  []dupcode.NewEntry{{}, {}},
		Spread:               []dupcode.EntryChange{{}},
		Unchanged:            17,
		ParameterChanges:     []string{"minTokens: 50 -> 60"},
		CanonicalFormChanged: true,
	}

	got := buildDupJSON(t, dupRuleResult(rr)).Checks.DuplicatedCode[0]
	if got.Snapshot == nil {
		t.Fatal("baselined detection reported no snapshot")
	}
	s := got.Snapshot
	if s.Path != ".rev-dep/duplicates.json" || s.New != 2 || s.Spread != 1 || s.Unchanged != 17 {
		t.Errorf("delta not carried over: %+v", s)
	}
	if !s.CanonicalFormChanged || len(s.ParameterChanges) != 1 {
		t.Errorf("context that explains a surprising delta was dropped: %+v", s)
	}
	// The totals are still reported alongside the delta; what changed decides the status.
	if got.Snippets != 12 {
		t.Errorf("totals were replaced by the delta: %+v", got)
	}
}

func TestJSONOutputReportsAMissingSnapshotAsFailure(t *testing.T) {
	rr := clean()
	rr.Result.SnapshotPath = ".rev-dep/duplicates.json"
	rr.Result.SnapshotMissing = true

	got := buildDupJSON(t, dupRuleResult(rr)).Checks.DuplicatedCode[0]
	if got.Status != "fail" {
		t.Errorf("a configured snapshot that does not exist is a broken setup, got %q", got.Status)
	}
	if got.Snapshot == nil || !got.Snapshot.Missing {
		t.Errorf("nothing in the output says the snapshot is missing: %+v", got.Snapshot)
	}
}

func TestJSONOutputSnapshotWriteIsNotAFailure(t *testing.T) {
	rr := withDuplication()
	rr.Result.SnapshotPath = ".rev-dep/duplicates.json"
	rr.Result.SnapshotWritten = true

	got := buildDupJSON(t, dupRuleResult(rr)).Checks.DuplicatedCode[0]
	if got.Status != "pass" {
		t.Errorf("updating the baseline should not fail the run, got %q", got.Status)
	}
	if got.Snapshot == nil || !got.Snapshot.Written {
		t.Errorf("nothing in the output says the snapshot was rewritten: %+v", got.Snapshot)
	}
}

// TestIssuesListReportsDuplicatedCodeAsOneItem pins the same choice for the issues list:
// one line per detection stating the count, never one line per duplicated block.
func TestIssuesListReportsDuplicatedCodeAsOneItem(t *testing.T) {
	out := formatIssuesListOutput([]jsonRuleResult{buildDupJSON(t, dupRuleResult(withDuplication()))})

	if !strings.Contains(out, "Duplicated Code Issues (1):") {
		t.Fatalf("duplicated code missing from the issues list:\n%s", out)
	}
	if !strings.Contains(out, "12 duplicated snippets in 8 files (30 occurrences)") {
		t.Errorf("the count is not stated:\n%s", out)
	}
	if !strings.Contains(out, "rev-dep duplicated-code --blind-identifiers") {
		t.Errorf("no way to see the places:\n%s", out)
	}
	// One item means one line of body under the header.
	body := strings.Split(strings.TrimSpace(out), "\n")[1:]
	if len(body) != 1 {
		t.Errorf("expected a single line for the detection, got %d:\n%s", len(body), out)
	}
}

func TestIssuesListOmitsPassingDetections(t *testing.T) {
	out := formatIssuesListOutput([]jsonRuleResult{buildDupJSON(t, dupRuleResult(clean()))})
	if strings.Contains(out, "Duplicated Code") {
		t.Errorf("a detection that found nothing is not an issue:\n%s", out)
	}
	if !strings.Contains(out, "No issues found") {
		t.Errorf("expected an empty issues list:\n%s", out)
	}
}

func TestIssuesListStatesTheSnapshotChangeRatherThanTheTotal(t *testing.T) {
	rr := withDuplication()
	rr.Result.SnapshotPath = ".rev-dep/duplicates.json"
	rr.Result.Delta = &dupcode.Delta{
		New:       []dupcode.NewEntry{{}, {}},
		Unchanged: 17,
	}

	out := formatIssuesListOutput([]jsonRuleResult{buildDupJSON(t, dupRuleResult(rr))})
	if !strings.Contains(out, "2 new") || !strings.Contains(out, "17 unchanged") {
		t.Errorf("the change is not what is reported:\n%s", out)
	}
	if strings.Contains(out, "12 duplicated snippets") {
		t.Errorf("with a baseline the total is not the news:\n%s", out)
	}
}

func TestIssuesListNamesAMissingSnapshot(t *testing.T) {
	rr := clean()
	rr.Result.SnapshotPath = ".rev-dep/duplicates.json"
	rr.Result.SnapshotMissing = true

	out := formatIssuesListOutput([]jsonRuleResult{buildDupJSON(t, dupRuleResult(rr))})
	if !strings.Contains(out, "snapshot .rev-dep/duplicates.json does not exist") {
		t.Errorf("a broken setup reads as a clean run:\n%s", out)
	}
}

// The command is a shell string for a human: it stays on the struct for the issues list, but
// is absent from the machine output.
func TestJSONOutputDoesNotPublishTheCommand(t *testing.T) {
	raw, err := json.Marshal(buildDupJSON(t, dupRuleResult(withDuplication())).Checks.DuplicatedCode[0])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, published := fields["command"]; published {
		t.Errorf("the shell command is in the machine output: %s", raw)
	}
	if _, ok := fields["configIndex"]; !ok {
		t.Errorf("configIndex is missing, so nothing ties the result to its settings: %s", raw)
	}
}
