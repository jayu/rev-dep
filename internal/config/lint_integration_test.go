package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeLintProject scaffolds a tiny project with a JSONC config containing a mix of
// live and dead patterns, and returns its directory.
func writeLintProject(t *testing.T, configBody string) string {
	t.Helper()
	dir := t.TempDir()

	mustWrite := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mustWrite("package.json", `{"name":"lint-fixture","version":"1.0.0","dependencies":{"react":"^18.0.0"}}`)
	mustWrite("src/index.ts", "import React from 'react'\nimport { used } from './used'\nexport const app = used(React)\n")
	mustWrite("src/used.ts", "export const used = (x: unknown) => x\n")
	mustWrite("rev-dep.config.jsonc", configBody)
	return dir
}

const lintConfigBody = `{
  // rev-dep config for lint fixture
  "configVersion": "1.11",
  "ignoreFiles": [
    "**/*.spec.ts", // no spec files exist -> dead
    "src/used.ts"
  ],
  "rules": [
    {
      "path": "src",
      "prodEntryPoints": [
        "index.ts",
        "ghost.ts" // renamed away -> dead
      ],
      "orphanFilesDetection": {
        "enabled": true,
        "validEntryPoints": ["*.config.ts"]
      },
      "restrictedImportsDetection": {
        "enabled": true,
        "entryPoints": ["index.ts"],
        "denyModules": ["nonexistent-pkg"]
      }
    }
  ]
}`

func findDead(deads []DeadPattern, detector, option, value string) *DeadPattern {
	for i := range deads {
		d := deads[i]
		if d.DetectorType == detector && d.OptionKey == option && d.Value == value {
			return &deads[i]
		}
	}
	return nil
}

func TestLintConfig_DetectsDeadPatterns(t *testing.T) {
	dir := writeLintProject(t, lintConfigBody)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	result, err := LintConfig(&cfg, dir, "", "", nil)
	if err != nil {
		t.Fatalf("LintConfig: %v", err)
	}

	cases := []struct {
		detector, option, value string
		removable               bool
		kind                    PatternKind
	}{
		{"", "ignoreFiles", "**/*.spec.ts", true, KindFile},
		{"", "prodEntryPoints", "ghost.ts", true, KindFile},
		{"orphanFilesDetection", "validEntryPoints", "*.config.ts", true, KindFile},
		{"restrictedImportsDetection", "denyModules", "nonexistent-pkg", false, KindModule},
	}
	for _, c := range cases {
		d := findDead(result.DeadPatterns, c.detector, c.option, c.value)
		if d == nil {
			t.Errorf("expected dead pattern %s.%s=%q not found", c.detector, c.option, c.value)
			continue
		}
		if d.Removable != c.removable {
			t.Errorf("%s.%s=%q removable=%v, want %v", c.detector, c.option, c.value, d.Removable, c.removable)
		}
		if d.Kind != c.kind {
			t.Errorf("%s.%s=%q kind=%v, want %v", c.detector, c.option, c.value, d.Kind, c.kind)
		}
	}

	// Live patterns must NOT be reported.
	for _, v := range []string{"src/used.ts", "index.ts"} {
		for _, d := range result.DeadPatterns {
			if d.Value == v {
				t.Errorf("live pattern %q incorrectly reported dead", v)
			}
		}
	}
}

// Regression: patterns in a rule whose path is a nested workspace dir (e.g. apps/web)
// must be resolved RELATIVE TO THE RULE PATH, not the repo root. A glob like "pages/**"
// must match apps/web/pages/* and not be reported dead.
func TestLintConfig_NestedRulePathResolvesRelative(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	mustWrite("package.json", `{"name":"root","version":"1.0.0","workspaces":["apps/*"]}`)
	mustWrite("apps/web/package.json", `{"name":"web","version":"1.0.0"}`)
	mustWrite("apps/web/pages/home.tsx", "export const Home = () => null\n")
	mustWrite("apps/web/server/src/index.ts", "export const server = 1\n")
	mustWrite("rev-dep.config.jsonc", `{
  "configVersion": "1.11",
  "rules": [
    {
      "path": "apps/web",
      "prodEntryPoints": [
        "server/src/index.ts",
        "ghost/main.ts"
      ],
      "orphanFilesDetection": {
        "enabled": true,
        "validEntryPoints": ["pages/**", "nowhere/**"]
      }
    }
  ]
}`)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	result, err := LintConfig(&cfg, dir, "", "", nil)
	if err != nil {
		t.Fatalf("LintConfig: %v", err)
	}

	// Live (must NOT be reported): resolve under apps/web.
	for _, live := range []string{"pages/**", "server/src/index.ts"} {
		if d := findDeadByValue(result.DeadPatterns, live); d != nil {
			t.Errorf("pattern %q wrongly reported dead (option %s)", live, d.OptionKey)
		}
	}
	// Dead (must be reported): no such files exist under apps/web.
	for _, dead := range []string{"nowhere/**", "ghost/main.ts"} {
		if findDeadByValue(result.DeadPatterns, dead) == nil {
			t.Errorf("expected %q to be reported dead", dead)
		}
	}
}

func TestLintConfig_RuleSelection(t *testing.T) {
	dir := writeLintProject(t, lintConfigBody)
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// File rule only: no module-kind dead patterns, and the module pattern is not reported.
	fileOnly, err := LintConfig(&cfg, dir, "", "", []LintRuleName{RuleOrphanFileGlobs})
	if err != nil {
		t.Fatalf("LintConfig(file): %v", err)
	}
	for _, d := range fileOnly.DeadPatterns {
		if d.Kind == KindModule || d.Kind == KindMixed {
			t.Errorf("file-only run produced non-file dead pattern: %+v", d)
		}
	}
	if findDeadByValue(fileOnly.DeadPatterns, "nonexistent-pkg") != nil {
		t.Error("file-only run should not report module pattern")
	}
	if findDeadByValue(fileOnly.DeadPatterns, "**/*.spec.ts") == nil {
		t.Error("file-only run should still report file globs")
	}

	// Module rule only: only module/mixed dead patterns; no file globs reported.
	modOnly, err := LintConfig(&cfg, dir, "", "", []LintRuleName{RuleOrphanModuleGlobs})
	if err != nil {
		t.Fatalf("LintConfig(module): %v", err)
	}
	for _, d := range modOnly.DeadPatterns {
		if d.Kind == KindFile || d.Kind == KindDir {
			t.Errorf("module-only run produced file dead pattern: %+v", d)
		}
	}
	if findDeadByValue(modOnly.DeadPatterns, "nonexistent-pkg") == nil {
		t.Error("module-only run should report the dead module pattern")
	}
	if findDeadByValue(modOnly.DeadPatterns, "**/*.spec.ts") != nil {
		t.Error("module-only run should not report file globs")
	}
}

// Regression: a detector object with several fully-dead array options must have ALL of
// them removed, and the reported count must equal what was actually removed. Removing
// them one-by-one produced overlapping byte edits that silently dropped some members
// (leaving them in the file) while still counting them as removed.
func TestLintConfig_FixRemovesMultipleFullyDeadMembers(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	mustWrite("package.json", `{"name":"root","version":"1.0.0"}`)
	mustWrite("src/index.ts", "export const a = 1\n")
	mustWrite("rev-dep.config.jsonc", `{
  "configVersion": "1.11",
  "rules": [
    {
      "path": "src",
      "unusedExportsDetection": {
        "enabled": true,
        "validEntryPoints": ["deadA/**", "deadB/**"],
        "graphExclude": ["deadC/**"],
        "ignoreFiles": ["deadE/**"]
      }
    }
  ]
}`)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	result, err := LintConfig(&cfg, dir, "", "", []LintRuleName{RuleOrphanFileGlobs})
	if err != nil {
		t.Fatalf("LintConfig: %v", err)
	}
	fix, err := ApplyLintFix(result)
	if err != nil {
		t.Fatalf("ApplyLintFix: %v", err)
	}
	if fix.RemovedCount != 4 || fix.ReportOnlyKept != 0 {
		t.Fatalf("RemovedCount=%d ReportOnlyKept=%d, want 4 and 0", fix.RemovedCount, fix.ReportOnlyKept)
	}

	// The reported removal must match reality: re-linting the written file finds nothing.
	cfg2, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig (reparse): %v", err)
	}
	rerun, err := LintConfig(&cfg2, dir, "", "", []LintRuleName{RuleOrphanFileGlobs})
	if err != nil {
		t.Fatalf("LintConfig (rerun): %v", err)
	}
	if len(rerun.DeadPatterns) != 0 {
		t.Fatalf("re-run still reports %d dead patterns (count/file disagreement): %+v", len(rerun.DeadPatterns), rerun.DeadPatterns)
	}
}

// Regression: ParseConfig synthesizes orphan/unusedExports validEntryPoints from the
// rule's entry points when they are not explicitly set (inheritance). The linter must
// NOT report those synthesized values — they are not physically in the config file, so
// reporting them produced phantom duplicates and count/file disagreement.
func TestLintConfig_IgnoresInheritedEntryPoints(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	mustWrite("package.json", `{"name":"root","version":"1.0.0"}`)
	mustWrite("apps/web/package.json", `{"name":"web","version":"1.0.0"}`)
	mustWrite("apps/web/src/x.ts", "export const a = 1\n")
	// orphanFilesDetection has NO validEntryPoints (array form) -> it inherits
	// prodEntryPoints at parse time. Those inherited values must not be linted.
	mustWrite("rev-dep.config.jsonc", `{
  "configVersion": "1.11",
  "rules": [
    {
      "path": "apps/web",
      "prodEntryPoints": ["pages/**"],
      "orphanFilesDetection": [ { "enabled": true }, { "enabled": true, "graphExclude": ["ghost.test.ts"] } ]
    }
  ]
}`)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	result, err := LintConfig(&cfg, dir, "", "", []LintRuleName{RuleOrphanFileGlobs})
	if err != nil {
		t.Fatalf("LintConfig: %v", err)
	}

	// "pages/**" must appear exactly once (the real prodEntryPoints), not once per
	// inheriting orphan detection.
	pagesCount := 0
	for _, d := range result.DeadPatterns {
		if d.Value == "pages/**" {
			pagesCount++
		}
		if d.DetectorType == "orphanFilesDetection" && d.OptionKey == "validEntryPoints" {
			t.Errorf("inherited validEntryPoints was linted: %+v", d)
		}
	}
	if pagesCount != 1 {
		t.Errorf("pages/** reported %d times, want 1", pagesCount)
	}

	// Count must match reality: fix, then re-lint finds nothing.
	fix, err := ApplyLintFix(result)
	if err != nil {
		t.Fatalf("ApplyLintFix: %v", err)
	}
	cfg2, _ := LoadConfig(dir)
	rerun, err := LintConfig(&cfg2, dir, "", "", []LintRuleName{RuleOrphanFileGlobs})
	if err != nil {
		t.Fatalf("re-lint: %v", err)
	}
	if len(rerun.DeadPatterns) != 0 {
		t.Fatalf("re-run found %d dead (count said removed %d/kept %d): %+v",
			len(rerun.DeadPatterns), fix.RemovedCount, fix.ReportOnlyKept, rerun.DeadPatterns)
	}
}

func TestParseLintRules(t *testing.T) {
	all, err := ParseLintRules(nil)
	if err != nil || len(all) != len(AllLintRules) {
		t.Fatalf("nil should select all rules, got %v err=%v", all, err)
	}
	one, err := ParseLintRules([]string{"orphan-file-globs"})
	if err != nil || len(one) != 1 || one[0] != RuleOrphanFileGlobs {
		t.Fatalf("expected [orphan-file-globs], got %v err=%v", one, err)
	}
	if _, err := ParseLintRules([]string{"bogus"}); err == nil {
		t.Fatal("expected error for unknown rule name")
	}
	// Duplicates collapse.
	dup, _ := ParseLintRules([]string{"orphan-file-globs", "orphan-file-globs"})
	if len(dup) != 1 {
		t.Fatalf("duplicates should collapse, got %v", dup)
	}
}

func findDeadByValue(deads []DeadPattern, value string) *DeadPattern {
	for i := range deads {
		if deads[i].Value == value {
			return &deads[i]
		}
	}
	return nil
}

func TestLintConfig_FixPreservesCommentsAndLivePatterns(t *testing.T) {
	dir := writeLintProject(t, lintConfigBody)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	result, err := LintConfig(&cfg, dir, "", "", nil)
	if err != nil {
		t.Fatalf("LintConfig: %v", err)
	}

	fix, err := ApplyLintFix(result)
	if err != nil {
		t.Fatalf("ApplyLintFix: %v", err)
	}
	if fix.RemovedCount != 3 {
		t.Errorf("RemovedCount=%d, want 3", fix.RemovedCount)
	}
	if fix.ReportOnlyKept != 1 {
		t.Errorf("ReportOnlyKept=%d, want 1", fix.ReportOnlyKept)
	}

	out, err := os.ReadFile(filepath.Join(dir, "rev-dep.config.jsonc"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(out)

	// Removed dead patterns.
	for _, gone := range []string{"**/*.spec.ts", "ghost.ts", "*.config.ts", "validEntryPoints"} {
		if contains(got, gone) {
			t.Errorf("expected %q to be removed, still present:\n%s", gone, got)
		}
	}
	// Preserved comments and live patterns and report-only pattern.
	for _, keep := range []string{
		"// rev-dep config for lint fixture",
		"src/used.ts",
		`"index.ts"`,
		"nonexistent-pkg", // report-only, must remain
	} {
		if !contains(got, keep) {
			t.Errorf("expected %q to be preserved, missing:\n%s", keep, got)
		}
	}

	// Result must still be a parseable, valid config.
	if _, err := ParseConfig(out); err != nil {
		t.Fatalf("fixed config no longer parses: %v\n%s", err, got)
	}
}
