package cli

import (
	"os"
	"path/filepath"
	"testing"

	"rev-dep-go/internal/config"
)

func TestProcessConfigRun_RecheckRevealsNewOrphans(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rev-dep-config-run-recheck")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "index.ts"), []byte("export const entry = 1;\n"), 0644); err != nil {
		t.Fatalf("Failed to write entry file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "orphan-a.ts"), []byte("import './orphan-b';\nexport const a = 1;\n"), 0644); err != nil {
		t.Fatalf("Failed to write orphan-a file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "orphan-b.ts"), []byte("export const b = 1;\n"), 0644); err != nil {
		t.Fatalf("Failed to write orphan-b file: %v", err)
	}

	cfg := config.RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []config.Rule{
			{
				Path: ".",
				OrphanFilesDetections: []*config.OrphanFilesOptions{{
					Enabled:          true,
					ValidEntryPoints: []string{"src/index.ts"},
					Autofix:          true,
				}},
			},
		},
	}

	withoutRecheck, err := processConfigRun(&cfg, tempDir, "", "", true, false, false)
	if err != nil {
		t.Fatalf("processConfigRun without recheck failed: %v", err)
	}

	if withoutRecheck.DeletedFilesCount != 1 {
		t.Fatalf("expected 1 deleted file without recheck, got %d", withoutRecheck.DeletedFilesCount)
	}
	if len(withoutRecheck.RuleResults) != 1 || len(withoutRecheck.RuleResults[0].OrphanFiles) != 1 {
		t.Fatalf("expected first pass to report 1 orphan, got %+v", withoutRecheck.RuleResults)
	}
	if filepath.Base(withoutRecheck.RuleResults[0].OrphanFiles[0]) != "orphan-a.ts" {
		t.Fatalf("expected first pass orphan to be orphan-a.ts, got %s", withoutRecheck.RuleResults[0].OrphanFiles[0])
	}

	if _, err := os.Stat(filepath.Join(srcDir, "orphan-a.ts")); !os.IsNotExist(err) {
		t.Fatalf("expected orphan-a.ts to be deleted after first pass")
	}
	if _, err := os.Stat(filepath.Join(srcDir, "orphan-b.ts")); err != nil {
		t.Fatalf("expected orphan-b.ts to still exist after first pass: %v", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "orphan-a.ts"), []byte("import './orphan-b';\nexport const a = 1;\n"), 0644); err != nil {
		t.Fatalf("Failed to restore orphan-a file: %v", err)
	}

	withRecheck, err := processConfigRun(&cfg, tempDir, "", "", true, true, false)
	if err != nil {
		t.Fatalf("processConfigRun with recheck failed: %v", err)
	}

	if withRecheck.DeletedFilesCount != 1 {
		t.Fatalf("expected fix summary to preserve 1 deleted file, got %d", withRecheck.DeletedFilesCount)
	}
	if !withRecheck.HasFailures {
		t.Fatal("expected recheck to report remaining orphan issues")
	}
	if len(withRecheck.RuleResults) != 1 || len(withRecheck.RuleResults[0].OrphanFiles) != 1 {
		t.Fatalf("expected recheck to report 1 orphan, got %+v", withRecheck.RuleResults)
	}
	if filepath.Base(withRecheck.RuleResults[0].OrphanFiles[0]) != "orphan-b.ts" {
		t.Fatalf("expected recheck orphan to be orphan-b.ts, got %s", withRecheck.RuleResults[0].OrphanFiles[0])
	}
	if withRecheck.FixableIssuesCount != 1 {
		t.Fatalf("expected recheck to report 1 remaining fixable issue, got %d", withRecheck.FixableIssuesCount)
	}
}
