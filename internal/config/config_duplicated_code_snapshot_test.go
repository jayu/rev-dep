package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"rev-dep-go/internal/dupcode"
	"strings"
	"testing"
)

func TestDuplicatedCodeDetectionParsesSnapshotPath(t *testing.T) {
	cfg := parseRuleJSON(t, `"duplicatedCodeDetection":{"enabled":true,"snapshotPath":".rev-dep-duplicates.json"}`)
	got := cfg.Rules[0].getDuplicatedCodeDetections()
	if len(got) != 1 || got[0].SnapshotPath != ".rev-dep-duplicates.json" {
		t.Fatalf("snapshotPath not parsed: %#v", got)
	}
}

func TestDuplicatedCodeDetectionRejectsBadSnapshotPath(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"wrong type", `"duplicatedCodeDetection":{"enabled":true,"snapshotPath":123}`, "must be a string"},
		{"empty", `"duplicatedCodeDetection":{"enabled":true,"snapshotPath":"  "}`, "must not be empty"},
	} {
		raw := `{"configVersion":"1.0","workspaces":[{"path":".",` + tc.body + `}]}`
		_, err := ParseConfig([]byte(raw))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want an error mentioning %q", tc.name, err, tc.want)
		}
	}
}

func TestDuplicatedCodeCommandCarriesSnapshot(t *testing.T) {
	got := DuplicatedCodeCommand(
		&DuplicatedCodeOptions{Enabled: true, SnapshotPath: ".rev-dep-duplicates.json"}, ".", nil, nil)
	want := "rev-dep duplicated-code --snapshot .rev-dep-duplicates.json"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// runSnapshotConfig runs the fixture project with a snapshot at path.
func runSnapshotConfig(t *testing.T, dir, snapshotPath string, update bool) RuleResult {
	t.Helper()
	cfg := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{{
			Path: ".",
			DuplicatedCodeDetections: []*DuplicatedCodeOptions{{
				Enabled: true, MinTokens: 10, MinLines: 1, SnapshotPath: snapshotPath,
			}},
		}},
	}
	result, err := ProcessConfigWithOptions(&cfg, dir, false, false, update)
	if err != nil {
		t.Fatalf("ProcessConfig: %v", err)
	}
	return result.RuleResults[0]
}

// copyFixture gives each test its own copy so it can edit files.
func copyFixture(t *testing.T) string {
	t.Helper()
	src := fixtureDupProject(t)
	dst := t.TempDir()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestConfigRunSnapshotLifecycle(t *testing.T) {
	dir := copyFixture(t)
	const snapName = "dup-snapshot.json"

	// 1. Configured but absent is a broken setup, not an empty one.
	rr := runSnapshotConfig(t, dir, snapName, false)
	if rr.DuplicatedCode == nil || !onlyDuplicatedCode(t, rr).SnapshotMissing {
		t.Fatalf("a missing snapshot should be reported: %+v", rr.DuplicatedCode)
	}

	// 2. Writing it is explicit.
	rr = runSnapshotConfig(t, dir, snapName, true)
	if !onlyDuplicatedCode(t, rr).SnapshotWritten {
		t.Fatal("--update-snapshot did not write the file")
	}
	if _, err := os.Stat(filepath.Join(dir, snapName)); err != nil {
		t.Fatalf("snapshot not on disk: %v", err)
	}

	// 3. The next run matches it.
	rr = runSnapshotConfig(t, dir, snapName, false)
	if onlyDuplicatedCode(t, rr).Delta == nil {
		t.Fatal("no delta computed")
	}
	if !onlyDuplicatedCode(t, rr).Delta.IsClean() {
		t.Fatalf("clean run produced a delta: %+v", onlyDuplicatedCode(t, rr).Delta)
	}
}

func TestConfigRunSnapshotFailsOnAnyDifference(t *testing.T) {
	dir := copyFixture(t)
	const snapName = "dup-snapshot.json"
	runSnapshotConfig(t, dir, snapName, true)

	cfg := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{{
			Path: ".",
			DuplicatedCodeDetections: []*DuplicatedCodeOptions{{
				Enabled: true, MinTokens: 10, MinLines: 1, SnapshotPath: snapName,
			}},
		}},
	}

	clean, err := ProcessConfigWithOptions(&cfg, dir, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if clean.HasFailures {
		t.Fatal("a run matching its snapshot should pass")
	}

	// Removing a copy is an improvement, and still fails until acknowledged.
	if err := os.Remove(filepath.Join(dir, "src", "components", "Card.tsx")); err != nil {
		t.Fatal(err)
	}
	improved, err := ProcessConfigWithOptions(&cfg, dir, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !improved.HasFailures {
		t.Fatal("an out-of-date snapshot should fail even when duplication went down")
	}
	if !improved.RuleResults[0].DuplicatedCode[0].Result.Delta.HasImprovements() {
		t.Error("the delta should record the improvement")
	}
	if improved.RuleResults[0].DuplicatedCode[0].Result.Delta.HasRegressions() {
		t.Error("removing a copy is not a regression")
	}
}

func TestConfigRunSnapshotDecidesRatherThanTheCount(t *testing.T) {
	dir := copyFixture(t)
	const snapName = "dup-snapshot.json"
	runSnapshotConfig(t, dir, snapName, true)

	// A snapshot supersedes the count: the question becomes "did anything change", so a run
	// matching its baseline passes even though duplication exists and would otherwise fail.
	cfg := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{{
			Path: ".",
			DuplicatedCodeDetections: []*DuplicatedCodeOptions{{
				Enabled: true, MinTokens: 10, MinLines: 1, SnapshotPath: snapName,
			}},
		}},
	}
	result, err := ProcessConfigWithOptions(&cfg, dir, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasFailures {
		t.Errorf("the count decided the verdict while a snapshot was in use: %+v",
			result.RuleResults[0].DuplicatedCode)
	}
}

// TestDuplicatedCodeCommandEmitsSnapshotAsWritten pins that the two sides agree on where a
// snapshot lives. The config resolves snapshotPath against the workspace, and the command
// resolves --snapshot against its --cwd, which IS that workspace - so the hint can pass the
// path straight through, and a re-based one would point somewhere else.
func TestDuplicatedCodeCommandEmitsSnapshotAsWritten(t *testing.T) {
	cases := []struct {
		name     string
		snapshot string
		rulePath string
		want     string
	}{
		{"root workspace", ".rev-dep-duplicates.json", ".",
			"rev-dep duplicated-code --snapshot .rev-dep-duplicates.json"},
		{"nested workspace", ".rev-dep-duplicates.json", "packages/app",
			"rev-dep duplicated-code --cwd packages/app --snapshot .rev-dep-duplicates.json"},
		{"path inside the workspace", "snapshots/dups.json", "packages/app",
			"rev-dep duplicated-code --cwd packages/app --snapshot snapshots/dups.json"},
		{"absolute path", "/tmp/dups.json", "packages/app",
			"rev-dep duplicated-code --cwd packages/app --snapshot /tmp/dups.json"},
	}
	for _, tc := range cases {
		got := DuplicatedCodeCommand(
			&DuplicatedCodeOptions{Enabled: true, SnapshotPath: tc.snapshot}, tc.rulePath, nil, nil)
		if got != tc.want {
			t.Errorf("%s:\n  got  %s\n  want %s", tc.name, got, tc.want)
		}
	}
}

// Two detections writing one snapshot file is not a config with a subtle consequence, it is
// a config that cannot work: each --update-snapshot overwrites the other, so the file never
// settles and both detections keep re-reporting duplications that were already acknowledged.
// Workspaces are also checked in parallel, so which one survives is not fixed.

func dupDetectionConfig(t *testing.T, rules ...Rule) *RevDepConfig {
	t.Helper()
	return &RevDepConfig{ConfigVersion: "2.0", Rules: rules}
}

func dupRule(path string, snapshots ...string) Rule {
	detections := make([]*DuplicatedCodeOptions, 0, len(snapshots))
	for _, snapshot := range snapshots {
		detections = append(detections, &DuplicatedCodeOptions{Enabled: true, SnapshotPath: snapshot})
	}
	return Rule{Path: path, DuplicatedCodeDetections: detections}
}

func TestValidateConfigRejectsSharedSnapshotPaths(t *testing.T) {
	cases := []struct {
		name  string
		rules []Rule
		want  string
	}{
		{
			// The case that motivated the change: every workspace taking the default name.
			name: "two workspaces, same relative path",
			rules: []Rule{
				dupRule("packages/app", "../../.rev-dep-duplicates.json"),
				dupRule("packages/lib", "../../.rev-dep-duplicates.json"),
			},
			want: "both resolve to",
		},
		{
			// The array case: one workspace, several detections - an exact one and a
			// structural one, say - sharing a file.
			name:  "two detections inside one workspace",
			rules: []Rule{dupRule("packages/app", "dups.json", "dups.json")},
			want:  "duplicatedCodeDetection[0]",
		},
		{
			// Different spellings, same file. Comparing the raw strings would miss it.
			name:  "different spellings of one path",
			rules: []Rule{dupRule(".", "dups.json", "./snapshots/../dups.json")},
			want:  "both resolve to",
		},
		{
			name: "absolute paths that collide",
			rules: []Rule{
				dupRule("packages/app", "/tmp/dups.json"),
				dupRule("packages/lib", "/tmp/dups.json"),
			},
			want: "both resolve to",
		},
	}
	for _, tc := range cases {
		err := ValidateConfig(dupDetectionConfig(t, tc.rules...))
		if err == nil {
			t.Errorf("%s: expected a validation error", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error does not name the problem: %v", tc.name, err)
		}
	}
}

func TestValidateConfigAcceptsDistinctSnapshotPaths(t *testing.T) {
	cases := []struct {
		name  string
		rules []Rule
	}{
		{
			// Same STRING, different workspaces - which is exactly what resolving against
			// the workspace makes safe, and what a raw-string check would wrongly reject.
			name: "same name under different workspaces",
			rules: []Rule{
				dupRule("packages/app", ".rev-dep-duplicates.json"),
				dupRule("packages/lib", ".rev-dep-duplicates.json"),
			},
		},
		{
			name:  "two detections in one workspace with their own files",
			rules: []Rule{dupRule("packages/app", "dups-exact.json", "dups-structural.json")},
		},
		{
			name:  "no snapshots at all",
			rules: []Rule{{Path: "."}, {Path: "packages/app"}},
		},
		{
			name: "only one detection uses a snapshot",
			rules: []Rule{
				dupRule("packages/app", "dups.json"),
				{Path: "packages/lib", DuplicatedCodeDetections: []*DuplicatedCodeOptions{
					{Enabled: true}}},
			},
		},
	}
	for _, tc := range cases {
		if err := ValidateConfig(dupDetectionConfig(t, tc.rules...)); err != nil {
			t.Errorf("%s: unexpected validation error: %v", tc.name, err)
		}
	}
}

// A disabled detection is included on purpose: enabling one later must not be able to turn a
// valid config into a broken one.
func TestValidateConfigRejectsSharedSnapshotPathWhenOneIsDisabled(t *testing.T) {
	rules := []Rule{
		dupRule("packages/app", "dups.json"),
		{Path: "packages/app", DuplicatedCodeDetections: []*DuplicatedCodeOptions{
			{Enabled: false, SnapshotPath: "dups.json"}}},
	}
	if err := ValidateConfig(dupDetectionConfig(t, rules...)); err == nil {
		t.Error("a disabled detection sharing a snapshot path must still be rejected")
	}
}

// TestDuplicatedCodeCommandOmitsSettingsWhenASnapshotIsUsed covers the seam between the hint
// and the settings check.
//
// The snapshot records every detection setting and the command reads them back from it, so
// spelling them out again is at best noise. It is worse than noise once the config is edited
// without re-baselining: the hint would then carry flags that contradict the snapshot, and
// running it would print a settings conflict instead of the snippets it exists to show.
func TestDuplicatedCodeCommandOmitsSettingsWhenASnapshotIsUsed(t *testing.T) {
	full := &DuplicatedCodeOptions{
		Enabled: true, BlindIdentifiers: true, BlindStrings: true, BlindNumbers: true,
		SkipObjects: true,
		MinTokens:   1, MinLines: 2, MinDepth: 3, MinStatements: 4, MinDuplicates: 5,
		IgnoreFiles:  []string{"src/gen/**"},
		SnapshotPath: ".rev-dep-duplicates.json",
	}
	got := DuplicatedCodeCommand(full, "packages/app", []string{"vendor/**"}, []string{"dist/**"})

	want := "rev-dep duplicated-code --cwd packages/app --snapshot .rev-dep-duplicates.json"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// Without a snapshot there is nothing to read the settings back from, so every one of them
// has to be on the command line or it would report a different number than the rule did.
func TestDuplicatedCodeCommandKeepsSettingsWithoutASnapshot(t *testing.T) {
	full := &DuplicatedCodeOptions{
		Enabled: true, BlindIdentifiers: true, SkipObjects: true,
		MinTokens: 1, MinDuplicates: 5,
		IgnoreFiles: []string{"src/gen/**"},
	}
	got := DuplicatedCodeCommand(full, "packages/app", nil, []string{"dist/**"})

	for _, want := range []string{
		"--cwd packages/app", "--blind-identifiers", "--skip-objects",
		"--min-tokens 1", "--min-duplicates 5",
		"--ignore-files 'src/gen/**'", "--process-ignored-files 'dist/**'",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("command is missing %q: %s", want, got)
		}
	}
}

// TestSnapshotCarriesConfigLevelIgnoresSoTheHintReproduces closes the loop the hint depends
// on. `rev-dep duplicated-code --cwd <workspace> --snapshot <path>` is printed beside a
// failure as the way to see the snippets, so it has to see the same FILES the rule saw - and
// the config's top-level ignoreFiles never reach the detector, they shape discovery before
// it. Without them recorded, the command reported copies the rule had excluded, as a
// regression nobody caused.
func TestSnapshotCarriesConfigLevelIgnoresSoTheHintReproduces(t *testing.T) {
	dir := t.TempDir()
	body := `{
  const request = buildRequest(endpoint, payload);
  const response = await transport.send(request);
  if (!response.ok) { throw new Fail(response.status); }
  return response.body;
}`
	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{"name":"root"}`)
	write("pkg/package.json", `{"name":"pkg"}`)
	write("pkg/src/a.ts", "export function a(endpoint, payload) "+body+"\n")
	write("pkg/src/b.ts", "export function b(endpoint, payload) "+body+"\n")
	// A third copy, in a directory the CONFIG excludes - not the detection.
	write("pkg/generated/c.ts", "export function c(endpoint, payload) "+body+"\n")
	// Written to disk and LOADED, not hand-built beside a stub file. The snapshot records
	// which config produced it, so the config on disk has to be the one that ran - otherwise
	// the recorded path names a file that cannot reproduce the baseline, which is the whole
	// thing this test is checking.
	write("rev-dep.config.json", `{
	  "configVersion": "2.0",
	  "ignoreFiles": ["pkg/generated/**"],
	  "workspaces": [{
	    "path": "pkg",
	    "duplicatedCodeDetection": {
	      "enabled": true, "minTokens": 10, "minLines": 1, "snapshotPath": "dups.json"
	    }
	  }]
	}`)

	snapshotPath := filepath.Join(dir, "pkg", "dups.json")
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Write the baseline the way a config run does.
	if _, err := ProcessConfigWithOptions(&cfg, dir, false, false, true); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("no snapshot was written: %v", err)
	}
	var snap dupcode.Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Entries) != 1 || snap.Entries[0].Occurrences != 2 {
		t.Fatalf("the config run should have acknowledged the two non-ignored copies, got %+v",
			snap.Entries)
	}

	// The config's own patterns travel with the config they were written in.
	cl := snap.Parameters.ConfigLevelSettings
	if cl == nil {
		t.Fatal("the config-level ignoreFiles were not recorded, so the hint cannot reproduce")
	}
	if cl.ConfigPath != "../rev-dep.config.json" {
		t.Errorf("configPath = %q, want it relative to the snapshot", cl.ConfigPath)
	}

	// Now the part that matters: a standalone run pointed only at the workspace and the
	// snapshot has to see exactly the two files the rule did.
	opts := dupcode.Options{
		Cwd: filepath.Join(dir, "pkg"), SnapshotPath: snapshotPath,
		// A snapshot comparison keys on the canonical hash of every finding.
		NeedHashes: true,
	}
	opts, err = dupcode.ReconcileFlagsWithSnapshot(opts, &snap, snapshotPath, map[string]bool{})
	if err != nil {
		t.Fatalf("a run with no flags should adopt the snapshot's settings: %v", err)
	}
	dups, stats, err := dupcode.Detect(opts)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Files != 2 {
		t.Errorf("scanned %d files, want the 2 the config run saw", stats.Files)
	}
	if len(dups) != 1 || len(dups[0].Occurrences) != 2 {
		t.Fatalf("the standalone run disagrees with the config run: %d findings, %+v", len(dups), dups)
	}

	delta := dupcode.CompareToSnapshot(dups, nil, &snap, opts)
	if !delta.IsClean() {
		t.Errorf("the hint reports a change against a snapshot it just reproduced: %s",
			dupcode.DeltaSummary(delta))
	}
}
