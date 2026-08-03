package config

import (
	"path/filepath"
	"strings"
	"testing"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/dupcode"
)

// fixtureDupProject is the same project the dupcode package characterises, so the counts
// here and there move together and a disagreement is visible.
func fixtureDupProject(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../../__fixtures__/duplicatedCodeProject")
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func runDupConfig(t *testing.T, detection *DuplicatedCodeOptions) RuleResult {
	t.Helper()
	cwd := fixtureDupProject(t)
	cfg := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{{
			Path:                     ".",
			DuplicatedCodeDetections: []*DuplicatedCodeOptions{detection},
		}},
	}
	result, err := ProcessConfig(&cfg, cwd, false, false)
	if err != nil {
		t.Fatalf("ProcessConfig: %v", err)
	}
	if len(result.RuleResults) != 1 {
		t.Fatalf("expected 1 rule result, got %d", len(result.RuleResults))
	}
	return result.RuleResults[0]
}

// onlyDuplicatedCode returns the single detection's result, failing if a test that assumes
// one detection is looking at a rule with several.
func onlyDuplicatedCode(t *testing.T, rr RuleResult) checks.DuplicatedCodeResult {
	t.Helper()
	if len(rr.DuplicatedCode) != 1 {
		t.Fatalf("expected exactly 1 duplicated-code detection, got %d", len(rr.DuplicatedCode))
	}
	return rr.DuplicatedCode[0].Result
}

func TestConfigRunReportsDuplicatedCodeCount(t *testing.T) {
	rr := runDupConfig(t, &DuplicatedCodeOptions{
		Enabled: true, MinTokens: 10, MinLines: 3,
	})

	if rr.DuplicatedCode == nil {
		t.Fatal("duplicated code result is missing")
	}
	// Matches TestCharacterizationLiteral in the dupcode package.
	if onlyDuplicatedCode(t, rr).Snippets != 3 {
		t.Errorf("Snippets = %d, want 3", onlyDuplicatedCode(t, rr).Snippets)
	}
	if onlyDuplicatedCode(t, rr).Occurrences != 6 {
		t.Errorf("Occurrences = %d, want 6", onlyDuplicatedCode(t, rr).Occurrences)
	}
	if onlyDuplicatedCode(t, rr).Blinding != "exact" {
		t.Errorf("Blinding = %q, want exact", onlyDuplicatedCode(t, rr).Blinding)
	}
}

func TestConfigRunDuplicatedCodeIsListedAsEnabledCheck(t *testing.T) {
	rr := runDupConfig(t, &DuplicatedCodeOptions{Enabled: true, MinTokens: 10, MinLines: 3})
	found := false
	for _, c := range rr.EnabledChecks {
		if c == "duplicated-code" {
			found = true
		}
	}
	if !found {
		t.Errorf("duplicated-code missing from enabled checks: %v", rr.EnabledChecks)
	}
}

func TestConfigRunDuplicatedCodeDisabled(t *testing.T) {
	rr := runDupConfig(t, &DuplicatedCodeOptions{Enabled: false})
	if len(rr.DuplicatedCode) != 0 {
		t.Errorf("disabled detector still produced a result: %+v", rr.DuplicatedCode)
	}
}

func TestConfigRunDuplicatedCodeStructuralModeFindsMore(t *testing.T) {
	lit := runDupConfig(t, &DuplicatedCodeOptions{Enabled: true, BlindIdentifiers: false, MinTokens: 10, MinLines: 3})
	str := runDupConfig(t, &DuplicatedCodeOptions{Enabled: true, BlindIdentifiers: true, MinTokens: 8, MinLines: 3})

	if onlyDuplicatedCode(t, str).Blinding != "identifiers" {
		t.Errorf("Blinding = %q, want identifiers", onlyDuplicatedCode(t, str).Blinding)
	}
	// teamApi is a renamed copy, so structural blinding finds one more occurrence of the
	// API body than literal blinding does.
	if onlyDuplicatedCode(t, str).Occurrences <= onlyDuplicatedCode(t, lit).Occurrences {
		t.Errorf("structural found %d occurrences, literal found %d - expected more",
			onlyDuplicatedCode(t, str).Occurrences, onlyDuplicatedCode(t, lit).Occurrences)
	}
}

func TestConfigRunDuplicatedCodeRespectsFilters(t *testing.T) {
	base := runDupConfig(t, &DuplicatedCodeOptions{Enabled: true, MinTokens: 10, MinLines: 3})
	deep := runDupConfig(t, &DuplicatedCodeOptions{Enabled: true, MinTokens: 10, MinLines: 3, MinDepth: 2})

	if onlyDuplicatedCode(t, deep).Snippets >= onlyDuplicatedCode(t, base).Snippets {
		t.Errorf("minDepth did not narrow the result: %d vs %d",
			onlyDuplicatedCode(t, deep).Snippets, onlyDuplicatedCode(t, base).Snippets)
	}
}

// Any duplication fails when no snapshot is configured: there is no tolerated count.
func TestConfigRunDuplicatedCodeFailsOnAnyDuplicationWithoutASnapshot(t *testing.T) {
	cwd := fixtureDupProject(t)
	cfg := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{{
			Path: ".",
			DuplicatedCodeDetections: []*DuplicatedCodeOptions{{
				Enabled: true, MinTokens: 10, MinLines: 3,
			}},
		}},
	}
	result, err := ProcessConfig(&cfg, cwd, false, false)
	if err != nil {
		t.Fatalf("ProcessConfig: %v", err)
	}
	if got := onlyDuplicatedCode(t, result.RuleResults[0]).Snippets; got == 0 {
		t.Fatal("the fixture is supposed to be duplicated; this test proves nothing")
	}
	if !result.HasFailures {
		t.Error("duplication was found and the rule passed anyway")
	}
	if !result.RuleResults[0].DuplicatedCode[0].Failed() {
		t.Error("the detection did not report itself as failed")
	}
}

func TestConfigRunDuplicatedCodeCommandReproducesTheCount(t *testing.T) {
	detection := &DuplicatedCodeOptions{
		Enabled: true, BlindIdentifiers: true, MinTokens: 8, MinLines: 3, MinDepth: 2, MinDuplicates: 2,
	}
	rr := runDupConfig(t, detection)
	if rr.DuplicatedCode == nil {
		t.Fatal("no duplicated code result")
	}

	// Run the detector directly with exactly the options the command carries.
	dups, _, err := dupcode.Detect(dupcode.Options{
		Cwd: fixtureDupProject(t),
		Blinding: dupcode.Blinding{
			Identifiers: detection.BlindIdentifiers,
			Strings:     detection.BlindStrings,
			Numbers:     detection.BlindNumbers,
		},
		MinTokens:     detection.MinTokens,
		MinLines:      detection.MinLines,
		MinDepth:      detection.MinDepth,
		MinDuplicates: detection.MinDuplicates,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(dups) != onlyDuplicatedCode(t, rr).Snippets {
		t.Errorf("the command would report %d snippets, the rule counted %d",
			len(dups), onlyDuplicatedCode(t, rr).Snippets)
	}
	occurrences := 0
	for _, d := range dups {
		occurrences += len(d.Occurrences)
	}
	if occurrences != onlyDuplicatedCode(t, rr).Occurrences {
		t.Errorf("the command would report %d occurrences, the rule counted %d",
			occurrences, onlyDuplicatedCode(t, rr).Occurrences)
	}

	// And the hint the run produced must be the command that does that.
	got := DuplicatedCodeCommand(detection, ".", nil, nil)
	if got != rr.DuplicatedCode[0].Command {
		t.Errorf("hint on the result is %q, want %q", rr.DuplicatedCode[0].Command, got)
	}
}

// TestConfigRunReportsEveryDuplicatedCodeDetection covers the convention every other
// detector follows: a workspace may configure several detections, and each is reported.
// Reporting only one would silently discard a result the user asked for.
func TestConfigRunReportsEveryDuplicatedCodeDetection(t *testing.T) {
	cwd := fixtureDupProject(t)
	cfg := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{{
			Path: ".",
			DuplicatedCodeDetections: []*DuplicatedCodeOptions{
				{Enabled: true, BlindIdentifiers: false, MinTokens: 10, MinLines: 3},
				{Enabled: true, BlindIdentifiers: true, MinTokens: 8, MinLines: 3},
			},
		}},
	}
	result, err := ProcessConfig(&cfg, cwd, false, false)
	if err != nil {
		t.Fatal(err)
	}
	got := result.RuleResults[0].DuplicatedCode
	if len(got) != 2 {
		t.Fatalf("expected both detections reported, got %d", len(got))
	}

	// Order follows the config, so a reader can match them up.
	if got[0].Result.Blinding != "exact" || got[1].Result.Blinding != "identifiers" {
		t.Fatalf("order does not follow the config: %q then %q",
			got[0].Result.Blinding, got[1].Result.Blinding)
	}
	// Each carries its own numbers rather than one standing in for both.
	if got[0].Result.Occurrences == got[1].Result.Occurrences {
		t.Errorf("both detections reported the same occurrence count (%d); "+
			"structural should find the renamed copy too", got[0].Result.Occurrences)
	}
	// And its own reproduction command.
	if !strings.Contains(got[1].Command, "--blind-identifiers") {
		t.Errorf("the blinded detection's command is wrong: %s", got[1].Command)
	}
	if strings.Contains(got[0].Command, "--blind-") {
		t.Errorf("the exact detection's command blinds something: %s", got[0].Command)
	}
}

// One detection passing must not mask another failing. With no tolerated count to raise, a
// detection is made to pass by putting its floors out of reach.
func TestConfigRunFailsIfAnyDuplicatedCodeDetectionFails(t *testing.T) {
	cwd := fixtureDupProject(t)
	const findsNothing = 1 << 20 // no block in the fixture is a million tokens
	const findsDuplication = 10

	run := func(literalFloor, structuralFloor int) bool {
		cfg := RevDepConfig{
			ConfigVersion: "1.0",
			Rules: []Rule{{
				Path: ".",
				DuplicatedCodeDetections: []*DuplicatedCodeOptions{
					{Enabled: true, BlindIdentifiers: false, MinTokens: literalFloor, MinLines: 3},
					{Enabled: true, BlindIdentifiers: true, MinTokens: structuralFloor, MinLines: 3},
				},
			}},
		}
		result, err := ProcessConfig(&cfg, cwd, false, false)
		if err != nil {
			t.Fatal(err)
		}
		return result.HasFailures
	}

	if run(findsNothing, findsNothing) {
		t.Error("neither detection found duplication: should pass")
	}
	if !run(findsDuplication, findsNothing) {
		t.Error("the literal detection found duplication: should fail")
	}
	if !run(findsNothing, findsDuplication) {
		t.Error("the structural detection found duplication: should fail")
	}
}

// A disabled detection produces no result, so ConfigIndex - not the position in the output -
// is what says which config entry a result came from.
func TestConfigRunDuplicatedCodeCarriesItsConfigIndex(t *testing.T) {
	cwd := fixtureDupProject(t)
	cfg := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{{
			Path: ".",
			DuplicatedCodeDetections: []*DuplicatedCodeOptions{
				{Enabled: false, MinTokens: 10, MinLines: 3},
				{Enabled: true, BlindIdentifiers: true, MinTokens: 10, MinLines: 3},
			},
		}},
	}
	result, err := ProcessConfig(&cfg, cwd, false, false)
	if err != nil {
		t.Fatal(err)
	}
	got := result.RuleResults[0].DuplicatedCode
	if len(got) != 1 {
		t.Fatalf("expected 1 result for 1 enabled detection, got %d", len(got))
	}
	if got[0].ConfigIndex != 1 {
		t.Errorf("result came from config entry 1, reports %d - a consumer looking its settings "+
			"up would read the disabled detection instead", got[0].ConfigIndex)
	}
}

// TestConfigRunDisabledDetectionIsSkipped checks that a disabled entry in an array does
// not produce a result.
func TestConfigRunDisabledDetectionIsSkipped(t *testing.T) {
	cwd := fixtureDupProject(t)
	cfg := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{{
			Path: ".",
			DuplicatedCodeDetections: []*DuplicatedCodeOptions{
				{Enabled: true, BlindIdentifiers: false, MinTokens: 10, MinLines: 3},
				{Enabled: false, BlindIdentifiers: true},
			},
		}},
	}
	result, err := ProcessConfig(&cfg, cwd, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.RuleResults[0].DuplicatedCode; len(got) != 1 {
		t.Fatalf("a disabled detection should produce no result, got %d entries", len(got))
	}
}

// TestConfigRunSkipObjectsWithMultipleDetections is the end-to-end form of the bug: via
// `duplicated-code` the flag worked, via `config run` it did not, because the shared scan
// reconciled the detections' disagreement about objects by dropping the filter.
func TestConfigRunSkipObjectsWithMultipleDetections(t *testing.T) {
	cwd := fixtureDupProject(t)

	count := func(detections ...*DuplicatedCodeOptions) []int {
		t.Helper()
		cfg := RevDepConfig{
			ConfigVersion: "1.0",
			Rules:         []Rule{{Path: ".", DuplicatedCodeDetections: detections}},
		}
		result, err := ProcessConfig(&cfg, cwd, false, false)
		if err != nil {
			t.Fatal(err)
		}
		var out []int
		for _, d := range result.RuleResults[0].DuplicatedCode {
			out = append(out, d.Result.Snippets)
		}
		return out
	}

	lit := func() *DuplicatedCodeOptions {
		return &DuplicatedCodeOptions{Enabled: true, BlindIdentifiers: false, MinTokens: 10, MinLines: 3}
	}
	litSkip := func() *DuplicatedCodeOptions {
		o := lit()
		o.SkipObjects = true
		return o
	}

	alone := count(lit())[0]
	aloneSkipping := count(litSkip())[0]
	if aloneSkipping >= alone {
		t.Fatalf("skipObjects had no effect even on its own: %d vs %d", aloneSkipping, alone)
	}

	// Two detections sharing one scan and disagreeing. Each must get its own answer, the
	// same one it would get alone.
	pair := count(lit(), litSkip())
	if len(pair) != 2 {
		t.Fatalf("expected two results, got %d", len(pair))
	}
	if pair[0] != alone {
		t.Errorf("the detection keeping objects reported %d, want %d", pair[0], alone)
	}
	if pair[1] != aloneSkipping {
		t.Errorf("the detection skipping objects reported %d, want %d "+
			"(the shared scan dropped its filter)", pair[1], aloneSkipping)
	}

	// And in the other order, in case the first spec seen wins.
	reversed := count(litSkip(), lit())
	if reversed[0] != aloneSkipping || reversed[1] != alone {
		t.Errorf("order changed the answers: got %v, want [%d %d]", reversed, aloneSkipping, alone)
	}
}
