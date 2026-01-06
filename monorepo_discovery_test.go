package main

import (
	"os"
	"path/filepath"
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

	monorepoCtx.FindWorkspacePackages(monorepoCtx.WorkspaceRoot)

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
	monorepoCtx.FindWorkspacePackages(monorepoCtx.WorkspaceRoot)

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
	monorepoCtx.FindWorkspacePackages(monorepoCtx.WorkspaceRoot)

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
	monorepoCtx.FindWorkspacePackages(monorepoCtx.WorkspaceRoot)

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

	manager := NewResolverManager(true, []string{"import"}, rootParams)
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
