package config

import (
	"strings"
	"testing"
)

func TestMigrateConfig_AutoEdits(t *testing.T) {
	in := `{
  // my config
  "configVersion": "1.8",
  "rules": [
    {
      "path": ".",
      "prodEntryPoints": ["src/index.ts"],
      "circularImportsDetection": {
        "enabled": true,
        "algorithm": "SCC" // pick algo
      }
    }
  ]
}`
	res, err := MigrateConfig([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("expected Changed=true")
	}
	got := string(res.Migrated)

	// `// my config` (an unrelated comment) survives; `// pick algo` was inline on the removed
	// `algorithm` line, so it is removed with it.
	for _, want := range []string{`"workspaces"`, `"configVersion": "2.0"`, `// my config`} {
		if !strings.Contains(got, want) {
			t.Errorf("migrated output missing %q\n---\n%s", want, got)
		}
	}
	for _, unwant := range []string{`"rules"`, `"algorithm"`, `"1.8"`, `"SCC"`} {
		if strings.Contains(got, unwant) {
			t.Errorf("migrated output should not contain %q\n---\n%s", unwant, got)
		}
	}
	// The result must still parse as a valid v3 config.
	if _, err := ParseConfig(res.Migrated); err != nil {
		t.Errorf("migrated config does not parse: %v\n---\n%s", err, got)
	}
}

func TestMigrateConfig_AlreadyV3IsNoOp(t *testing.T) {
	in := `{"configVersion":"2.0","workspaces":[{"path":"."}]}`
	res, err := MigrateConfig([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Errorf("expected no changes, applied: %v", res.AppliedChanges)
	}
}

func TestMigrateConfig_GlobReviews(t *testing.T) {
	in := `{
  "configVersion": "1.5",
  "rules": [
    {
      "path": ".",
      "prodEntryPoints": ["src/index.ts", "src/*.ts", "**/*.test.*"],
      "ignoreEntryPoints": ["!src/keep.ts"]
    },
    {
      "path": "apps/web",
      "followMonorepoPackages": true,
      "prodEntryPoints": ["api", "src/**"]
    }
  ]
}`
	res, err := MigrateConfig([]byte(in))
	if err != nil {
		t.Fatal(err)
	}

	byPattern := map[string]PatternReview{}
	for _, r := range res.PatternReviews {
		byPattern[r.Pattern] = r
	}

	// Gitignore-semantics flags: single *, **/, and leading !.
	for _, p := range []string{"src/*.ts", "**/*.test.*", "!src/keep.ts"} {
		if _, ok := byPattern[p]; !ok {
			t.Errorf("expected %q flagged for gitignore semantics", p)
		}
	}
	// Not flagged: a plain literal path.
	if _, ok := byPattern["src/index.ts"]; ok {
		t.Errorf("src/index.ts should not be flagged")
	}
	// Cross-workspace: bare name in a non-root workspace with follow enabled.
	if r, ok := byPattern["api"]; !ok {
		t.Errorf("expected bare name 'api' flagged in apps/web")
	} else if !hasReason(r, "sibling") {
		t.Errorf("'api' should carry a cross-workspace reason, got %v", r.Reasons)
	}
	// src/** in apps/web: scoped literal first segment -> no cross-leak, no gitignore flag.
	if _, ok := byPattern["src/**"]; ok {
		t.Errorf("src/** should not be flagged")
	}
}

func TestMigrateConfig_ResultNotes(t *testing.T) {
	in := `{
  "configVersion": "1.5",
  "rules": [
    { "path": ".", "circularImportsDetection": true, "unusedNodeModulesDetection": { "enabled": true } }
  ]
}`
	res, err := MigrateConfig([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.ResultNotes, "\n")
	if !strings.Contains(joined, "SCC") {
		t.Errorf("expected circular/SCC result note, got %v", res.ResultNotes)
	}
	if !strings.Contains(joined, "whole words") {
		t.Errorf("expected unused-deps result note, got %v", res.ResultNotes)
	}
}

func hasReason(r PatternReview, substr string) bool {
	for _, reason := range r.Reasons {
		if strings.Contains(reason, substr) {
			return true
		}
	}
	return false
}
