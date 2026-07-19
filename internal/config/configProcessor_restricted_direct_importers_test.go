package config

import (
	"os"
	"path/filepath"
	"testing"
)

// End-to-end: the processor builds the file tree from real sources, runs the direct-importers
// detector, and populates the RuleResult + enabled checks.
func TestConfigProcessor_RestrictedDirectImporters(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rev-dep-direct-importers")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mustWrite := func(rel, content string) {
		p := filepath.Join(tempDir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	// config/secret.ts is the restricted target.
	mustWrite("config/secret.ts", "export const secret = 'shh';\n")
	// public/leak.ts directly imports it -> should be a violation under denyImporters: public/**.
	mustWrite("src/public/leak.ts", "import { secret } from '../../config/secret';\nexport const leaked = secret;\n")
	// config/reader.ts directly imports it, but is not a denied importer.
	mustWrite("src/config/reader.ts", "import { secret } from '../../config/secret';\nexport const ok = secret;\n")
	// deep/consumer.ts reaches secret only transitively (through reader) -> never a violation.
	mustWrite("src/public/consumer.ts", "import { ok } from '../config/reader';\nexport const v = ok;\n")
	mustWrite("package.json", `{"name":"direct-importers-fixture","version":"1.0.0","private":true}`)
	mustWrite("tsconfig.json", `{"compilerOptions":{"strict":true},"include":["src","config"]}`)

	configJSON := `{
		"configVersion": "1.11",
		"workspaces": [{
			"path": ".",
			"restrictedDirectImportersDetection": {
				"enabled": true,
				"files": ["config/**"],
				"denyImporters": ["src/public/**"]
			}
		}]
	}`
	configPath := filepath.Join(tempDir, "rev-dep.config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	result, err := ProcessConfig(&cfg, tempDir, "package.json", "tsconfig.json", false, false)
	if err != nil {
		t.Fatalf("process config: %v", err)
	}

	if len(result.RuleResults) != 1 {
		t.Fatalf("expected 1 rule result, got %d", len(result.RuleResults))
	}
	rr := result.RuleResults[0]

	hasCheck := false
	for _, c := range rr.EnabledChecks {
		if c == "restricted-direct-importers" {
			hasCheck = true
		}
	}
	if !hasCheck {
		t.Errorf("expected 'restricted-direct-importers' in enabled checks, got %v", rr.EnabledChecks)
	}

	violations := rr.RestrictedDirectImportersViolations
	if len(violations) != 1 {
		names := make([]string, len(violations))
		for i, v := range violations {
			rel, _ := filepath.Rel(tempDir, v.ImporterFile)
			names[i] = filepath.ToSlash(rel)
		}
		t.Fatalf("expected exactly 1 violation (public/leak.ts), got %d: %v", len(violations), names)
	}

	v := violations[0]
	importerRel, _ := filepath.Rel(tempDir, v.ImporterFile)
	fileRel, _ := filepath.Rel(tempDir, v.File)
	if filepath.ToSlash(importerRel) != "src/public/leak.ts" {
		t.Errorf("expected importer src/public/leak.ts, got %s", filepath.ToSlash(importerRel))
	}
	if filepath.ToSlash(fileRel) != "config/secret.ts" {
		t.Errorf("expected denied file config/secret.ts, got %s", filepath.ToSlash(fileRel))
	}
	if !result.HasFailures {
		t.Error("expected HasFailures to be true")
	}
}
