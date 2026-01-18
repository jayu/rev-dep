package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
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

	monorepoCtx := DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via pnpm-workspace.yaml")
	}

	monorepoCtx.FindWorkspacePackages(monorepoCtx.WorkspaceRoot, []GlobMatcher{})

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

	monorepoCtx := DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via npm workspaces")
	}
	monorepoCtx.FindWorkspacePackages(monorepoCtx.WorkspaceRoot, []GlobMatcher{})

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

	monorepoCtx := DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via yarn workspaces")
	}
	monorepoCtx.FindWorkspacePackages(monorepoCtx.WorkspaceRoot, []GlobMatcher{})

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

	monorepoCtx := DetectMonorepo(tmpDir)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo via bun workspaces")
	}
	monorepoCtx.FindWorkspacePackages(monorepoCtx.WorkspaceRoot, []GlobMatcher{})

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
			allKeys = append(allKeys, NormalizePathForInternal(filepath.Join(tmpDir, k)))
		}
	}

	rootParams := RootParams{
		TsConfigContent: []byte("{}"),
		PkgJsonContent:  []byte(files["packages/app/package.json"]),
		SortedFiles:     allKeys,
		Cwd:             cwd,
	}

	manager := NewResolverManager(true, []string{"import"}, rootParams, []GlobMatcher{})
	appFile := NormalizePathForInternal(filepath.Join(cwd, "src/main.ts"))
	resolver := manager.GetResolverForFile(appFile)

	// Resolve
	path, rtype, resErr := resolver.ResolveModule("@pnpm/lib", appFile)
	if resErr != nil {
		t.Errorf("Expected nil error for pnpm resolution, got %v", resErr)
	}
	if rtype != MonorepoModule {
		t.Errorf("Expected MonorepoModule, got %v", rtype)
	}
	expected := NormalizePathForInternal(filepath.Join(tmpDir, "packages/lib/src/index.ts"))
	if path != expected {
		t.Errorf("Expected %s, got %s", expected, path)
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
	//     lib2/internal/lib2-internal/package.json (recursion should stop at lib2)
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

	ctx := NewMonorepoContext(root)

	excludeMatchers := CreateGlobMatchers([]string{"**/ignored/**"}, root)

	ctx.FindWorkspacePackages(root, excludeMatchers)

	expectedPackages := map[string]string{
		"@app/app1":   NormalizePathForInternal(filepath.Join(root, "apps", "app1")),
		"@app/app2":   NormalizePathForInternal(filepath.Join(root, "apps", "app2")),
		"@lib/lib1":   NormalizePathForInternal(filepath.Join(root, "libs", "lib1")),
		"@lib/lib2":   NormalizePathForInternal(filepath.Join(root, "libs", "lib2")),
		"@tool/tool1": NormalizePathForInternal(filepath.Join(root, "tools", "tool1")),
	}

	if len(ctx.PackageToPath) != len(expectedPackages) {
		t.Errorf("Expected %d packages, got %d", len(expectedPackages), len(ctx.PackageToPath))
	}

	for pkg, path := range expectedPackages {
		if gotPath, ok := ctx.PackageToPath[pkg]; !ok || gotPath != path {
			t.Errorf("Package %s: expected path %s, got %s", pkg, path, gotPath)
		}
	}

	// Verify that lib2-internal was NOT found because recursion stopped at lib2
	if _, ok := ctx.PackageToPath["@lib/lib2-internal"]; ok {
		t.Errorf("Recursion did NOT stop at lib2; found lib2-internal")
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

	ctx := NewMonorepoContext(root)

	ctx.FindWorkspacePackages(root, []GlobMatcher{})

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

	ctx := NewMonorepoContext(root)
	ctx.FindWorkspacePackages(root, []GlobMatcher{})

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
	monorepoCtx := DetectMonorepo(nestedPath)

	// Should find the root monorepo, not treat nested package as root
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo from nested package path")
	}

	if monorepoCtx.WorkspaceRoot != NormalizePathForInternal(root) {
		t.Errorf("Expected monorepo root at %s, got %s", NormalizePathForInternal(root), monorepoCtx.WorkspaceRoot)
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

	monorepoCtx := DetectMonorepo(root)

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

	monorepoCtx := DetectMonorepo(root)

	// Should detect as monorepo
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo with valid workspaces array")
	}

	if monorepoCtx.WorkspaceRoot != NormalizePathForInternal(root) {
		t.Errorf("Expected monorepo root at %s, got %s", NormalizePathForInternal(root), monorepoCtx.WorkspaceRoot)
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
	monorepoArray := DetectMonorepo(filepath.Join(arrayRoot, "packages", "pkg"))
	if monorepoArray == nil {
		t.Fatalf("Failed to detect monorepo for array-style workspaces")
	}
	if monorepoArray.WorkspaceRoot != NormalizePathForInternal(arrayRoot) {
		t.Errorf("Expected workspace root %s, got %s", NormalizePathForInternal(arrayRoot), monorepoArray.WorkspaceRoot)
	}
	monorepoArray.FindWorkspacePackages(monorepoArray.WorkspaceRoot, []GlobMatcher{})
	if _, ok := monorepoArray.PackageToPath["@arr/pkg"]; !ok {
		t.Errorf("Expected to find @arr/pkg in array-style workspaces")
	}

	// Detect and verify obj-root
	monorepoObj := DetectMonorepo(filepath.Join(objRoot, "packages", "pkg"))
	if monorepoObj == nil {
		t.Fatalf("Failed to detect monorepo for object-with-packages-style workspaces")
	}
	if monorepoObj.WorkspaceRoot != NormalizePathForInternal(objRoot) {
		t.Errorf("Expected workspace root %s, got %s", NormalizePathForInternal(objRoot), monorepoObj.WorkspaceRoot)
	}
	monorepoObj.FindWorkspacePackages(monorepoObj.WorkspaceRoot, []GlobMatcher{})
	if _, ok := monorepoObj.PackageToPath["@obj/pkg"]; !ok {
		t.Errorf("Expected to find @obj/pkg in object-with-packages-style workspaces")
	}
}
