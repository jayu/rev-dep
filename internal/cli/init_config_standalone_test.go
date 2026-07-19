package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"rev-dep-go/internal/config"
)

// writePackageJson writes a package.json with the given name at dir (created if needed).
func writePackageJson(t *testing.T, dir string, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create dir %s: %v", dir, err)
	}
	content := `{"name":"` + name + `"}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write package.json at %s: %v", dir, err)
	}
}

func rulePaths(rules []config.Rule) []string {
	paths := make([]string, 0, len(rules))
	for _, r := range rules {
		paths = append(paths, r.Path)
	}
	return paths
}

func TestInitConfig_NonMonorepoRootWithStandalonePackages(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rev-dep-standalone-root")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Root package.json WITHOUT workspaces -> not a monorepo, but the root is a package.
	writePackageJson(t, tempDir, "root-app")
	// Standalone packages in subdirectories.
	writePackageJson(t, filepath.Join(tempDir, "services", "api"), "@acme/api")
	writePackageJson(t, filepath.Join(tempDir, "services", "web"), "@acme/web")

	result, err := initConfigFileCore(tempDir)
	if err != nil {
		t.Fatalf("initConfigFileCore: %v", err)
	}

	if result.isMonorepo {
		t.Errorf("Expected isMonorepo=false")
	}
	if !result.rootHasPackageJson {
		t.Errorf("Expected rootHasPackageJson=true")
	}
	if !result.rootRuleCreated {
		t.Errorf("Expected a root rule to be created")
	}
	wantStandalone := []string{"services/api", "services/web"}
	if !slices.Equal(result.standalonePackagePaths, wantStandalone) {
		t.Errorf("Expected standalone %v, got %v", wantStandalone, result.standalonePackagePaths)
	}
	// Rules: root "." + two standalone packages.
	wantPaths := []string{".", "services/api", "services/web"}
	if got := rulePaths(result.rules); !slices.Equal(got, wantPaths) {
		t.Errorf("Expected rule paths %v, got %v", wantPaths, got)
	}
}

func TestInitConfig_NoRootPackageOnlySubdirs(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rev-dep-standalone-noroot")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// No root package.json at all, only packages in subdirectories.
	writePackageJson(t, filepath.Join(tempDir, "pkgs", "a"), "@acme/a")
	writePackageJson(t, filepath.Join(tempDir, "pkgs", "b"), "@acme/b")

	result, err := initConfigFileCore(tempDir)
	if err != nil {
		t.Fatalf("initConfigFileCore: %v", err)
	}

	if result.isMonorepo {
		t.Errorf("Expected isMonorepo=false")
	}
	if result.rootHasPackageJson {
		t.Errorf("Expected rootHasPackageJson=false")
	}
	if result.rootRuleCreated {
		t.Errorf("Expected NO root rule when there is no root package.json and standalone packages exist")
	}
	want := []string{"pkgs/a", "pkgs/b"}
	if !slices.Equal(result.standalonePackagePaths, want) {
		t.Errorf("Expected standalone %v, got %v", want, result.standalonePackagePaths)
	}
	// Rules: only the two standalone packages, no "." root rule.
	if got := rulePaths(result.rules); !slices.Equal(got, want) {
		t.Errorf("Expected rule paths %v, got %v", want, got)
	}
}

func TestInitConfig_NoRootPackageNoSubdirsFallsBackToRootRule(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rev-dep-standalone-empty")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	result, err := initConfigFileCore(tempDir)
	if err != nil {
		t.Fatalf("initConfigFileCore: %v", err)
	}

	if len(result.standalonePackagePaths) != 0 {
		t.Errorf("Expected no standalone packages, got %v", result.standalonePackagePaths)
	}
	if !result.rootRuleCreated {
		t.Errorf("Expected a fallback root rule when nothing is discovered")
	}
	if got := rulePaths(result.rules); !slices.Equal(got, []string{"."}) {
		t.Errorf("Expected a single '.' rule, got %v", got)
	}
}

func TestInitConfig_MonorepoWithStandalonePackagesOutsideWorkspaces(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rev-dep-monorepo-standalone")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Monorepo root declaring only packages/* as workspaces.
	rootPkg := `{"name":"root","workspaces":["packages/*"]}`
	if err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(rootPkg), 0644); err != nil {
		t.Fatalf("write root package.json: %v", err)
	}
	writePackageJson(t, filepath.Join(tempDir, "packages", "pkg1"), "@myorg/pkg1")
	// A package NOT covered by the workspaces glob.
	writePackageJson(t, filepath.Join(tempDir, "tools", "cli"), "@myorg/cli")

	result, err := initConfigFileCore(tempDir)
	if err != nil {
		t.Fatalf("initConfigFileCore: %v", err)
	}

	if !result.isMonorepo {
		t.Errorf("Expected isMonorepo=true")
	}
	if !slices.Equal(result.workspacePackagePaths, []string{"packages/pkg1"}) {
		t.Errorf("Expected 1 workspace package [packages/pkg1], got %v", result.workspacePackagePaths)
	}
	if !slices.Equal(result.standalonePackagePaths, []string{"tools/cli"}) {
		t.Errorf("Expected standalone [tools/cli], got %v", result.standalonePackagePaths)
	}
	// Rules: root "." + workspace pkg1 + standalone tools/cli.
	want := []string{".", "packages/pkg1", "tools/cli"}
	if got := rulePaths(result.rules); !slices.Equal(got, want) {
		t.Errorf("Expected rule paths %v, got %v", want, got)
	}
}

// node_modules must never be treated as a standalone package, even without a .gitignore.
func TestInitConfig_SkipsNodeModules(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rev-dep-standalone-nodemodules")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	writePackageJson(t, tempDir, "root-app")
	// An installed dependency living under node_modules - must be ignored.
	writePackageJson(t, filepath.Join(tempDir, "node_modules", "left-pad"), "left-pad")
	writePackageJson(t, filepath.Join(tempDir, "node_modules", "@scope", "thing"), "@scope/thing")
	// A genuine standalone package.
	writePackageJson(t, filepath.Join(tempDir, "packages", "real"), "@acme/real")

	result, err := initConfigFileCore(tempDir)
	if err != nil {
		t.Fatalf("initConfigFileCore: %v", err)
	}

	for _, p := range result.standalonePackagePaths {
		if filepath.ToSlash(p) == "node_modules/left-pad" || filepath.ToSlash(p) == "node_modules/@scope/thing" {
			t.Errorf("node_modules package %q must not be discovered", p)
		}
	}
	if !slices.Contains(result.standalonePackagePaths, "packages/real") {
		t.Errorf("Expected packages/real to be discovered, got %v", result.standalonePackagePaths)
	}
}
