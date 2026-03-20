package monorepo_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/model"
	monorepo "rev-dep-go/internal/monorepo"
	"rev-dep-go/internal/pathutil"
	"rev-dep-go/internal/resolve"
)

func TestPnpmWorkspaceParsing(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-pnpm-workspace")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"pnpm-workspace.yaml": `packages:
  - 'packages/*'
  - 'components/**'
`,
		"package.json":                      `{}`,
		"packages/pkg-a/package.json":       `{ "name": "@pnpm/pkg-a", "version": "1.0.0" }`,
		"components/ui/button/package.json": `{ "name": "@pnpm/button", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via pnpm-workspace.yaml")
	}

	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	expectedPackages := []string{
		"@pnpm/pkg-a",
		"@pnpm/button",
	}

	for _, pkgName := range expectedPackages {
		if _, ok := monorepoCtx.PackageToPath[pkgName]; !ok {
			t.Errorf("Expected to find package %s, but didn't", pkgName)
		}
	}
}

func TestNpmWorkspaceDiscovery(t *testing.T) {
	// NPM workspaces use "workspaces" array in package.json
	tmpDir, err := os.MkdirTemp("", "rev-dep-npm-workspace")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json": `{
			"workspaces": ["packages/a", "packages/b"]
		}`,
		"packages/a/package.json": `{ "name": "@npm/a", "version": "1.0.0" }`,
		"packages/b/package.json": `{ "name": "@npm/b", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via npm workspaces")
	}
	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	expected := []string{"@npm/a", "@npm/b"}
	for _, pkg := range expected {
		if _, ok := monorepoCtx.PackageToPath[pkg]; !ok {
			t.Errorf("Expected to find package %s", pkg)
		}
	}
}

func TestYarnWorkspaceDiscovery(t *testing.T) {
	// Yarn workspaces use "workspaces" array in package.json
	// Or object { packages: [] }? (Actually standard `workspaces` is array, some older Yarn might support object but array is standard)
	tmpDir, err := os.MkdirTemp("", "rev-dep-yarn-workspace")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json": `{
			"workspaces": ["modules/*"]
		}`,
		"modules/x/package.json": `{ "name": "@yarn/x", "version": "1.0.0" }`,
		"modules/y/package.json": `{ "name": "@yarn/y", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via yarn workspaces")
	}
	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	expected := []string{"@yarn/x", "@yarn/y"}
	for _, pkg := range expected {
		if _, ok := monorepoCtx.PackageToPath[pkg]; !ok {
			t.Errorf("Expected to find package %s", pkg)
		}
	}
}

func TestBunWorkspaceDiscovery(t *testing.T) {
	// Bun respects "workspaces" in package.json just like npm/yarn
	tmpDir, err := os.MkdirTemp("", "rev-dep-bun-workspace")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json": `{
			"workspaces": ["libs/*"]
		}`,
		"libs/1/package.json": `{ "name": "bun-lib-1", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via bun workspaces")
	}
	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	if _, ok := monorepoCtx.PackageToPath["bun-lib-1"]; !ok {
		t.Errorf("Expected to find bun-lib-1")
	}
}

func TestPnpmResolution(t *testing.T) {
	// Setup a pnpm workspace and verify we can resolve modules between packages
	tmpDir, err := os.MkdirTemp("", "rev-dep-pnpm-resolution")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"pnpm-workspace.yaml": `packages:
  - 'packages/*'
`,
		"package.json": `{}`,
		"packages/lib/package.json": `{
			"name": "@pnpm/lib",
			"version": "1.0.0",
			"main": "./src/index.ts"
		}`,
		"packages/lib/src/index.ts": `export const val = "pnpm-lib";`,
		"packages/app/package.json": `{
			"name": "@pnpm/app",
			"dependencies": {
				"@pnpm/lib": "workspace:*"
			}
		}`,
		"packages/app/src/main.ts": `import { val } from "@pnpm/lib";`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	// Detect Monorepo
	cwd := filepath.Join(tmpDir, "packages/app")

	// Setup Resolver
	allKeys := []string{}
	for k := range files {
		if filepath.Ext(k) == ".ts" {
			allKeys = append(allKeys, pathutil.NormalizePathForInternal(filepath.Join(tmpDir, k)))
		}
	}

	rootParams := resolve.RootParams{
		TsConfigContent: []byte("{}"),
		PkgJsonContent:  []byte(files["packages/app/package.json"]),
		SortedFiles:     allKeys,
		Cwd:             cwd,
	}

	manager := resolve.NewResolverManager(model.FollowMonorepoPackagesValue{FollowAll: true}, []string{"import"}, rootParams, []globutil.GlobMatcher{})
	appFile := pathutil.NormalizePathForInternal(filepath.Join(cwd, "src/main.ts"))
	resolver := manager.GetResolverForFile(appFile)

	// Resolve
	path, rtype, resErr := resolver.ResolveModule("@pnpm/lib", appFile)
	if resErr != nil {
		t.Errorf("Expected nil error for pnpm resolution, got %v", resErr)
	}
	if rtype != model.MonorepoModule {
		t.Errorf("Expected model.MonorepoModule, got %v", rtype)
	}
	expected := pathutil.NormalizePathForInternal(filepath.Join(tmpDir, "packages/lib/src/index.ts"))
	if path != expected {
		t.Errorf("Expected %s, got %s", expected, path)
	}
}

func TestPnpmWorkspaceRecursiveGlobDiscoversNestedPackages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-pnpm-recursive-workspace")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"pnpm-workspace.yaml": `packages:
  - 'packages/**'
`,
		"package.json":                  `{}`,
		"packages/desktop/package.json": `{ "name": "@repo/desktop", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/plugin-a/package.json": `{ "name": "@repo/plugin-a", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/plugin-b/package.json": `{ "name": "@repo/plugin-b", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via pnpm-workspace.yaml")
	}

	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	expected := []string{
		"@repo/desktop",
		"@repo/plugin-a",
		"@repo/plugin-b",
	}

	for _, pkgName := range expected {
		if _, ok := monorepoCtx.PackageToPath[pkgName]; !ok {
			t.Errorf("Expected to discover package %s with packages/** workspace glob", pkgName)
		}
	}
}

func TestPnpmWorkspaceSingleStarDoesNotDiscoverNestedPackages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-pnpm-single-star-workspace")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"pnpm-workspace.yaml": `packages:
  - 'packages/*'
`,
		"package.json":                  `{}`,
		"packages/desktop/package.json": `{ "name": "@repo/desktop", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/plugin-a/package.json": `{ "name": "@repo/plugin-a", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/plugin-b/package.json": `{ "name": "@repo/plugin-b", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via pnpm-workspace.yaml")
	}

	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	if _, ok := monorepoCtx.PackageToPath["@repo/desktop"]; !ok {
		t.Fatalf("Expected to discover direct package with packages/* workspace glob")
	}
	if _, ok := monorepoCtx.PackageToPath["@repo/plugin-a"]; ok {
		t.Errorf("Did not expect to discover nested package @repo/plugin-a with packages/* workspace glob")
	}
	if _, ok := monorepoCtx.PackageToPath["@repo/plugin-b"]; ok {
		t.Errorf("Did not expect to discover nested package @repo/plugin-b with packages/* workspace glob")
	}
}

func TestNpmWorkspaceRecursiveGlobDiscoversNestedPackages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-npm-recursive-workspace")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json": `{
			"workspaces": ["packages/**"]
		}`,
		"packages/desktop/package.json":                           `{ "name": "@repo/desktop", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/plugin-a/package.json": `{ "name": "@repo/plugin-a", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/plugin-b/package.json": `{ "name": "@repo/plugin-b", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via npm workspaces")
	}

	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	expected := []string{
		"@repo/desktop",
		"@repo/plugin-a",
		"@repo/plugin-b",
	}

	for _, pkgName := range expected {
		if _, ok := monorepoCtx.PackageToPath[pkgName]; !ok {
			t.Errorf("Expected to discover package %s with packages/** workspace glob", pkgName)
		}
	}
}

func TestYarnWorkspaceRecursiveGlobDiscoversNestedPackages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-yarn-recursive-workspace")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json": `{
			"workspaces": ["packages/**"]
		}`,
		"packages/desktop/package.json":                           `{ "name": "@repo/desktop", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/plugin-a/package.json": `{ "name": "@repo/plugin-a", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/plugin-b/package.json": `{ "name": "@repo/plugin-b", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via yarn workspaces")
	}

	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	expected := []string{
		"@repo/desktop",
		"@repo/plugin-a",
		"@repo/plugin-b",
	}

	for _, pkgName := range expected {
		if _, ok := monorepoCtx.PackageToPath[pkgName]; !ok {
			t.Errorf("Expected to discover package %s with packages/** workspace glob", pkgName)
		}
	}
}

func TestFindWorkspacePackagesSkipsGitIgnoredDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-monorepo-gitignore-skip")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"pnpm-workspace.yaml": `packages:
  - 'packages/**'
`,
		"package.json":                  `{}`,
		".gitignore":                    "plugin-workspace\n",
		"packages/desktop/package.json": `{ "name": "@repo/desktop", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/plugin-a/package.json": `{ "name": "@repo/plugin-a", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	// Ensure gitignore scan stops at a repository boundary.
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via pnpm-workspace.yaml")
	}

	// Pass no custom excludes - FindWorkspacePackages should still honor .gitignore.
	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	if _, ok := monorepoCtx.PackageToPath["@repo/desktop"]; !ok {
		t.Fatalf("Expected non-ignored package @repo/desktop to be discovered")
	}
	if _, ok := monorepoCtx.PackageToPath["@repo/plugin-a"]; ok {
		t.Errorf("Did not expect gitignored package @repo/plugin-a to be discovered")
	}
}

func TestFindWorkspacePackagesSkipsNestedGitIgnoredDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-monorepo-nested-gitignore-skip")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"pnpm-workspace.yaml": `packages:
  - 'packages/**'
`,
		"package.json":                                            `{}`,
		"packages/desktop/package.json":                           `{ "name": "@repo/desktop", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/.gitignore":            "plugin-b\n",
		"packages/desktop/plugin-workspace/plugin-a/package.json": `{ "name": "@repo/plugin-a", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/plugin-b/package.json": `{ "name": "@repo/plugin-b", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	// Ensure gitignore scan stops at a repository boundary.
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via pnpm-workspace.yaml")
	}

	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	if _, ok := monorepoCtx.PackageToPath["@repo/desktop"]; !ok {
		t.Fatalf("Expected package @repo/desktop to be discovered")
	}
	if _, ok := monorepoCtx.PackageToPath["@repo/plugin-a"]; !ok {
		t.Fatalf("Expected non-ignored nested package @repo/plugin-a to be discovered")
	}
	if _, ok := monorepoCtx.PackageToPath["@repo/plugin-b"]; ok {
		t.Errorf("Did not expect nested gitignored package @repo/plugin-b to be discovered")
	}
}

func TestPnpmWorkspaceEmptyShouldNotDetectMonorepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-pnpm-empty")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create an empty pnpm-workspace.yaml (no packages definition)
	files := map[string]string{
		"pnpm-workspace.yaml": "\n", // empty content
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx != nil {
		t.Fatalf("Expected NOT to detect monorepo when pnpm-workspace.yaml contains no packages, but detected %v", monorepoCtx.WorkspaceRoot)
	}
}

func TestFindWorkspacePackages(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "monorepo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up structure:
	// /root
	//   package.json (root)
	//   apps/
	//     app1/package.json
	//     app2/package.json
	//   libs/
	//     lib1/package.json
	//     lib1/node_modules/pkg/package.json (should be ignored)
	//     lib2/package.json
	//     lib2/internal/lib2-internal/package.json (should be discovered via libs/**)
	//   tools/
	//     tool1/package.json
	//   ignored/
	//     pkg/package.json

	root := tempDir
	mkdir := func(p ...string) string {
		path := filepath.Join(append([]string{root}, p...)...)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("Failed to mkdir %s: %v", path, err)
		}
		return path
	}
	writeFile := func(content string, p ...string) {
		path := filepath.Join(append([]string{root}, p...)...)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
	}

	writeFile(`{"name": "root", "workspaces": ["apps/*", "libs/**", "tools/tool1", "!**/ignored/**"]}`, "package.json")
	writeFile("node_modules\n", ".gitignore")
	mkdir(".git")

	mkdir("apps", "app1")
	writeFile(`{"name": "@app/app1"}`, "apps", "app1", "package.json")

	mkdir("apps", "app2")
	writeFile(`{"name": "@app/app2"}`, "apps", "app2", "package.json")

	mkdir("libs", "lib1")
	writeFile(`{"name": "@lib/lib1"}`, "libs", "lib1", "package.json")
	mkdir("libs", "lib1", "node_modules", "pkg")
	writeFile(`{"name": "ignored-node-module"}`, "libs", "lib1", "node_modules", "pkg", "package.json")

	mkdir("libs", "lib2")
	writeFile(`{"name": "@lib/lib2"}`, "libs", "lib2", "package.json")
	mkdir("libs", "lib2", "internal", "lib2-internal")
	writeFile(`{"name": "@lib/lib2-internal"}`, "libs", "lib2", "internal", "lib2-internal", "package.json")

	mkdir("tools", "tool1")
	writeFile(`{"name": "@tool/tool1"}`, "tools", "tool1", "package.json")

	mkdir("ignored", "pkg")
	writeFile(`{"name": "@ignored/pkg"}`, "ignored", "pkg", "package.json")

	ctx := monorepo.NewMonorepoContext(root)

	excludeMatchers := globutil.CreateGlobMatchers([]string{"**/ignored/**"}, root)

	ctx.FindWorkspacePackages(excludeMatchers)

	expectedPackages := map[string]string{
		"@app/app1":          pathutil.NormalizePathForInternal(filepath.Join(root, "apps", "app1")),
		"@app/app2":          pathutil.NormalizePathForInternal(filepath.Join(root, "apps", "app2")),
		"@lib/lib1":          pathutil.NormalizePathForInternal(filepath.Join(root, "libs", "lib1")),
		"@lib/lib2":          pathutil.NormalizePathForInternal(filepath.Join(root, "libs", "lib2")),
		"@lib/lib2-internal": pathutil.NormalizePathForInternal(filepath.Join(root, "libs", "lib2", "internal", "lib2-internal")),
		"@tool/tool1":        pathutil.NormalizePathForInternal(filepath.Join(root, "tools", "tool1")),
	}

	if len(ctx.PackageToPath) != len(expectedPackages) {
		t.Errorf("Expected %d packages, got %d", len(expectedPackages), len(ctx.PackageToPath))
	}

	for pkg, path := range expectedPackages {
		if gotPath, ok := ctx.PackageToPath[pkg]; !ok || gotPath != path {
			t.Errorf("Package %s: expected path %s, got %s", pkg, path, gotPath)
		}
	}

	// Verify that lib2-internal was found because libs/** is recursive
	if _, ok := ctx.PackageToPath["@lib/lib2-internal"]; !ok {
		t.Errorf("Expected nested package @lib/lib2-internal to be discovered for libs/**")
	}

	// Verify that ignored/pkg was NOT found because of exclude patterns
	if _, ok := ctx.PackageToPath["@ignored/pkg"]; ok {
		t.Errorf("Found package in ignored directory")
	}

	// Verify that node_modules was skipped
	if _, ok := ctx.PackageToPath["ignored-node-module"]; ok {
		t.Errorf("Found package in node_modules")
	}
}

func TestFindWorkspacePackagesSingleStarAtRoot(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "monorepo-test-root-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	root := tempDir
	writeFile := func(content string, p ...string) {
		path := filepath.Join(append([]string{root}, p...)...)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
	}

	writeFile(`{"name": "@pkg/a"}`, "a", "package.json")
	writeFile(`{"name": "@pkg/b"}`, "b", "package.json")
	writeFile(`{"name": "root", "workspaces": ["*"]}`, "package.json")

	ctx := monorepo.NewMonorepoContext(root)

	ctx.FindWorkspacePackages([]globutil.GlobMatcher{})

	expectedPackages := []string{"@pkg/a", "@pkg/b"}
	var gotPackages []string
	for pkg := range ctx.PackageToPath {
		gotPackages = append(gotPackages, pkg)
	}
	sort.Strings(gotPackages)
	sort.Strings(expectedPackages)

	if !reflect.DeepEqual(gotPackages, expectedPackages) {
		t.Errorf("Expected packages %v, got %v", expectedPackages, gotPackages)
	}
}
func TestWorkspaceRootExclusion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "monorepo-root-excl-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	root := tempDir
	writeFile := func(content string, p ...string) {
		path := filepath.Join(append([]string{root}, p...)...)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
	}

	// Package at root
	writeFile(`{"name": "@pkg/root", "workspaces": ["packages/*"]}`, "package.json")
	// Package in workspace
	writeFile(`{"name": "@pkg/a"}`, "packages", "a", "package.json")

	ctx := monorepo.NewMonorepoContext(root)
	ctx.FindWorkspacePackages([]globutil.GlobMatcher{})

	// Assert root package is NOT in the map
	if _, ok := ctx.PackageToPath["@pkg/root"]; ok {
		t.Errorf("Workspace root package '@pkg/root' should be excluded from discovery")
	}

	// Assert workspace package IS in the map
	if _, ok := ctx.PackageToPath["@pkg/a"]; !ok {
		t.Errorf("Workspace package '@pkg/a' should be discovered")
	}
}

func TestDetectMonorepoFalsePositiveWithWorkspacesKey(t *testing.T) {
	// Test case: non-root package with "workspaces" key should NOT be detected as monorepo root
	tempDir, err := os.MkdirTemp("", "monorepo-false-positive-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	root := tempDir
	writeFile := func(content string, p ...string) {
		path := filepath.Join(append([]string{root}, p...)...)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
	}

	// Root monorepo with proper workspaces configuration
	writeFile(`{"name": "root", "workspaces": ["packages/*"]}`, "package.json")

	// Package inside monorepo that also has a "workspaces" key (but it's not the root)
	writeFile(`{"name": "@internal/nested", "workspaces": []}`, "packages", "nested", "package.json")

	// Try to detect from the nested package
	nestedPath := filepath.Join(root, "packages", "nested")
	monorepoCtx := monorepo.DetectMonorepo(nestedPath)

	// Should find the root monorepo, not treat nested package as root
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo from nested package path")
	}

	if monorepoCtx.WorkspaceRoot != pathutil.NormalizePathForInternal(root) {
		t.Errorf("Expected monorepo root at %s, got %s", pathutil.NormalizePathForInternal(root), monorepoCtx.WorkspaceRoot)
	}
}

func TestDetectMonorepoIgnoresEmptyWorkspaces(t *testing.T) {
	// Test case: package with empty workspaces array/object should NOT be detected as monorepo root
	tempDir, err := os.MkdirTemp("", "monorepo-empty-ws-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	root := tempDir
	writeFile := func(content string, p ...string) {
		path := filepath.Join(append([]string{root}, p...)...)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
	}

	// Package with empty workspaces
	writeFile(`{"name": "@pkg/empty-ws", "workspaces": []}`, "package.json")

	monorepoCtx := monorepo.DetectMonorepo(root)

	// Should NOT detect as monorepo because workspaces is empty
	if monorepoCtx != nil {
		t.Errorf("Package with empty workspaces should not be detected as monorepo root")
	}
}

func TestDetectMonorepoWithValidWorkspacesArray(t *testing.T) {
	// Test case: package with non-empty workspaces array SHOULD be detected as monorepo root
	tempDir, err := os.MkdirTemp("", "monorepo-valid-ws-array-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	root := tempDir
	writeFile := func(content string, p ...string) {
		path := filepath.Join(append([]string{root}, p...)...)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
	}

	// Package with valid workspaces array
	writeFile(`{"name": "root", "workspaces": ["packages/*"]}`, "package.json")
	writeFile(`{"name": "@pkg/a"}`, "packages", "a", "package.json")

	monorepoCtx := monorepo.DetectMonorepo(root)

	// Should detect as monorepo
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo with valid workspaces array")
	}

	if monorepoCtx.WorkspaceRoot != pathutil.NormalizePathForInternal(root) {
		t.Errorf("Expected monorepo root at %s, got %s", pathutil.NormalizePathForInternal(root), monorepoCtx.WorkspaceRoot)
	}
}

func TestWorkspacesArrayAndPackagesObject(t *testing.T) {
	// Ensure both array-style and object-with-packages-style workspaces are supported
	tmpDir, err := os.MkdirTemp("", "monorepo-workspaces-formats-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Helper to write files
	writeFile := func(content string, p ...string) {
		path := filepath.Join(append([]string{tmpDir}, p...)...)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
	}

	// Array style
	arrayRoot := filepath.Join(tmpDir, "array-root")
	if err := os.MkdirAll(arrayRoot, 0755); err != nil {
		t.Fatalf("Failed to mkdir arrayRoot: %v", err)
	}
	writeFile(`{"name": "root-array", "workspaces": ["packages/*"]}`, "array-root", "package.json")
	writeFile(`{"name": "@arr/pkg"}`, "array-root", "packages", "pkg", "package.json")

	// Object-with-packages style
	objRoot := filepath.Join(tmpDir, "obj-root")
	if err := os.MkdirAll(objRoot, 0755); err != nil {
		t.Fatalf("Failed to mkdir objRoot: %v", err)
	}
	writeFile(`{"name": "root-obj", "workspaces": {"packages": ["packages/*"]}}`, "obj-root", "package.json")
	writeFile(`{"name": "@obj/pkg"}`, "obj-root", "packages", "pkg", "package.json")

	// Detect and verify array-root
	monorepoArray := monorepo.DetectMonorepo(filepath.Join(arrayRoot, "packages", "pkg"))
	if monorepoArray == nil {
		t.Fatalf("Failed to detect monorepo for array-style workspaces")
	}
	if monorepoArray.WorkspaceRoot != pathutil.NormalizePathForInternal(arrayRoot) {
		t.Errorf("Expected workspace root %s, got %s", pathutil.NormalizePathForInternal(arrayRoot), monorepoArray.WorkspaceRoot)
	}
	monorepoArray.FindWorkspacePackages([]globutil.GlobMatcher{})
	if _, ok := monorepoArray.PackageToPath["@arr/pkg"]; !ok {
		t.Errorf("Expected to find @arr/pkg in array-style workspaces")
	}

	// Detect and verify obj-root
	monorepoObj := monorepo.DetectMonorepo(filepath.Join(objRoot, "packages", "pkg"))
	if monorepoObj == nil {
		t.Fatalf("Failed to detect monorepo for object-with-packages-style workspaces")
	}
	if monorepoObj.WorkspaceRoot != pathutil.NormalizePathForInternal(objRoot) {
		t.Errorf("Expected workspace root %s, got %s", pathutil.NormalizePathForInternal(objRoot), monorepoObj.WorkspaceRoot)
	}
	monorepoObj.FindWorkspacePackages([]globutil.GlobMatcher{})
	if _, ok := monorepoObj.PackageToPath["@obj/pkg"]; !ok {
		t.Errorf("Expected to find @obj/pkg in object-with-packages-style workspaces")
	}
}

func TestPnpmTakesPrecedenceOverPackageJson(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-pnpm-vs-packagejson")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"pnpm-workspace.yaml": `packages:
  - 'packages/*'
`,
		"package.json": `{
			"workspaces": ["other/*"]
		}`,
		"packages/pkg-a/package.json": `{ "name": "@pnpm/only", "version": "1.0.0" }`,
		"other/pkg-b/package.json":    `{ "name": "@pkgjson/only", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	// Call monorepo.DetectMonorepo from inside the nested package that itself contains a `workspaces` key.
	// monorepo.DetectMonorepo should prefer the root pnpm-workspace.yaml, not the nested package.json.
	monorepoCtx := monorepo.DetectMonorepo(filepath.Join(tmpDir, "packages", "pkg-a"))
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via pnpm-workspace.yaml")
	}

	// Ensure the detected workspace root is the repo root (where pnpm-workspace.yaml lives)
	if monorepoCtx.WorkspaceRoot != pathutil.NormalizePathForInternal(tmpDir) {
		t.Fatalf("Expected monorepo root %s, got %s", pathutil.NormalizePathForInternal(tmpDir), monorepoCtx.WorkspaceRoot)
	}

	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	// Since pnpm-workspace.yaml is present at repo root, it should take precedence and only packages/* should be used
	if _, ok := monorepoCtx.PackageToPath["@pnpm/only"]; !ok {
		t.Errorf("Expected to find @pnpm/only from pnpm-workspace.yaml, but didn't")
	}
	if _, ok := monorepoCtx.PackageToPath["@pkgjson/only"]; ok {
		t.Errorf("Did not expect to find @pkgjson/only from package.json workspaces when pnpm-workspace.yaml is present")
	}
}

// TestPnpmWorkspaceDeepGlobStarStar verifies that /**/* glob patterns in
// pnpm-workspace.yaml are recognised as deep patterns. This was a real-world
// root cause: repos using /**/* instead of /** had zero subpackage resolvers
// discovered, making the per-file resolver lookup a no-op and causing all
// workspace dependencies to be falsely flagged as missing.
func TestPnpmWorkspaceDeepGlobStarStar(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-pnpm-deep-glob")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		// pnpm-workspace.yaml using /**/* (not /**)
		"pnpm-workspace.yaml": `packages:
  - 'packages/**/*'
`,
		"package.json":                           `{}`,
		"packages/apps/app1/package.json":        `{ "name": "@scope/app1", "version": "1.0.0" }`,
		"packages/libs/shared/package.json":      `{ "name": "@scope/shared", "version": "1.0.0" }`,
		"packages/libs/nested/deep/package.json": `{ "name": "@scope/deep", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via pnpm-workspace.yaml")
	}

	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	expected := []string{"@scope/app1", "@scope/shared", "@scope/deep"}
	for _, pkg := range expected {
		if _, ok := monorepoCtx.PackageToPath[pkg]; !ok {
			t.Errorf("Expected to find package %s with /**/* glob pattern, but didn't", pkg)
		}
	}

	if len(monorepoCtx.PackageToPath) != len(expected) {
		t.Errorf("Expected %d packages, got %d: %v", len(expected), len(monorepoCtx.PackageToPath), monorepoCtx.PackageToPath)
	}
}

// TestPnpmWorkspaceMiddleDeepThenSingleStar verifies support for patterns like
// "path/**/otherPath/*", where recursive matching is followed by immediate children.
func TestPnpmWorkspaceMiddleDeepThenSingleStar(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-pnpm-middle-deep-single-star")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"pnpm-workspace.yaml": `packages:
  - 'packages/**/plugin-workspace/*'
`,
		"package.json": `{}`,

		"packages/desktop/package.json":                           `{ "name": "@repo/desktop", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/plugin-a/package.json": `{ "name": "@repo/plugin-a", "version": "1.0.0" }`,
		"packages/desktop/plugin-workspace/plugin-b/package.json": `{ "name": "@repo/plugin-b", "version": "1.0.0" }`,

		"packages/other/nested/plugin-workspace/plugin-c/package.json": `{ "name": "@repo/plugin-c", "version": "1.0.0" }`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	monorepoCtx := monorepo.DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via pnpm-workspace.yaml")
	}

	monorepoCtx.FindWorkspacePackages([]globutil.GlobMatcher{})

	expected := []string{
		"@repo/plugin-a",
		"@repo/plugin-b",
		"@repo/plugin-c",
	}

	for _, pkgName := range expected {
		if _, ok := monorepoCtx.PackageToPath[pkgName]; !ok {
			t.Errorf("Expected to discover package %s with packages/**/plugin-workspace/* pattern", pkgName)
		}
	}

	if _, ok := monorepoCtx.PackageToPath["@repo/desktop"]; ok {
		t.Errorf("Did not expect to discover @repo/desktop for packages/**/plugin-workspace/* pattern")
	}
}
