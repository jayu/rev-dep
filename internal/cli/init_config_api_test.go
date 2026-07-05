package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"rev-dep-go/internal/config"
)

// TestInitConfig_NonInteractiveWrapper exercises the exported one-call API end to end: standalone
// subfolder filtering (curated), per-package entry-point detection, and detector selection — all
// without any prompt.
func TestInitConfig_NonInteractiveWrapper(t *testing.T) {
	dir, err := os.MkdirTemp("", "rev-dep-initapi")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.RemoveAll(dir)

	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	// Monorepo root + one workspace package (so per-package entry-point detection is clean).
	write("package.json", `{"name":"root","workspaces":["packages/*"]}`)
	write("packages/api/package.json", `{"name":"@m/api"}`)
	write("packages/api/index.ts", `import './lib';`) // entry
	write("packages/api/lib.ts", `export const l=1;`) // imported -> not an entry
	// A real standalone package and a fixture-like one (should be filtered by "curated").
	write("tools/cli/package.json", `{"name":"@m/cli"}`)
	write("tools/cli/index.ts", `export const c=1;`)
	write("__fixtures__/x/package.json", `{"name":"fx"}`)

	res, err := InitConfig(dir, InitOptions{
		Standalone:        StandaloneCurated,
		DetectEntryPoints: true,
		Detectors:         DetectorsAll,
	})
	if err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	// Config file was written and is parseable.
	if _, err := os.Stat(res.ConfigPath); err != nil {
		t.Fatalf("expected config file at %s: %v", res.ConfigPath, err)
	}
	content, _ := os.ReadFile(res.ConfigPath)
	if _, err := config.ParseConfig(content); err != nil {
		t.Fatalf("generated config does not parse: %v", err)
	}

	// Monorepo detection + curated standalone filtering (fixtures excluded).
	if !res.IsMonorepo || res.MonorepoPackageCount != 1 {
		t.Errorf("expected monorepo with 1 workspace package, got isMonorepo=%v count=%d", res.IsMonorepo, res.MonorepoPackageCount)
	}
	if !slices.Equal(res.StandalonePackagePaths, []string{"tools/cli"}) {
		t.Errorf("expected curated standalone [tools/cli], got %v", res.StandalonePackagePaths)
	}
	if got := rulePaths(res.Rules); !slices.Equal(got, []string{".", "packages/api", "tools/cli"}) {
		t.Errorf("unexpected rule paths %v", got)
	}

	// Entry-point detection ran (monorepo root skipped -> 2 package rules).
	if !res.EntryPointsDetected || res.EntryPointPackageCount != 2 {
		t.Errorf("expected entry-point detection on 2 packages, got detected=%v count=%d", res.EntryPointsDetected, res.EntryPointPackageCount)
	}

	// The workspace package rule has detected entry points and the "all" detector preset applied.
	var apiRule *config.Rule
	for i := range res.Rules {
		if res.Rules[i].Path == "packages/api" {
			apiRule = &res.Rules[i]
		}
	}
	if apiRule == nil {
		t.Fatalf("no rule for packages/api")
	}
	if !slices.Equal(apiRule.ProdEntryPoints, []string{"index.ts"}) {
		t.Errorf("packages/api prod entry points: got %v", apiRule.ProdEntryPoints)
	}
	if len(apiRule.OrphanFilesDetections) == 0 || !apiRule.OrphanFilesDetections[0].Enabled {
		t.Errorf("expected DetectorsAll to enable orphan-files on packages/api")
	}
}

// A pre-existing config must make the wrapper error.
func TestInitConfig_ErrorsWhenConfigExists(t *testing.T) {
	dir, err := os.MkdirTemp("", "rev-dep-initapi-exists")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, ".rev-dep.config.jsonc"), []byte(`{"configVersion":"1.0"}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := InitConfig(dir, InitOptions{}); err == nil {
		t.Fatalf("expected an error when a config already exists")
	}
}
