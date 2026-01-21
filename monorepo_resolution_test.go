package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMonorepoResolution(t *testing.T) {
	// Create a temporary directory for the monorepo
	tmpDir, err := os.MkdirTemp("", "rev-dep-monorepo-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json": `{
			"workspaces": ["packages/*", "apps/*"]
		}`,
		"packages/lib-a/package.json": `{
			"name": "@company/lib-a",
			"version": "1.0.0",
			"exports": {
				"./utils": "./src/utils.ts",
				".": "./src/index.ts"
			}
		}`,
		"packages/lib-a/src/index.ts": `export const A = 1;`,
		"packages/lib-a/src/utils.ts": `export const utils = 2;`,

		"packages/lib-b/package.json": `{
			"name": "@company/lib-b",
			"version": "1.0.0",
			"exports": "./dist/main.js"
		}`,
		"packages/lib-b/dist/main.js": `console.log("B");`,

		"apps/app-1/package.json": `{
			"name": "app-1",
			"dependencies": {
				"@company/lib-a": "*",
				"@company/lib-b": "workspace:*"
			}
		}`,
		"apps/app-1/src/main.ts": `
			import { utils } from '@company/lib-a/utils';
			import { A } from '@company/lib-a';
			import B from '@company/lib-b';
		`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	cwd := filepath.Join(tmpDir, "apps/app-1")

	allKeys := []string{}
	for k := range files {
		if filepath.Ext(k) == ".ts" || filepath.Ext(k) == ".js" || filepath.Ext(k) == ".json" {
			allKeys = append(allKeys, filepath.Join(tmpDir, k))
		}
	}

	rootParams := RootParams{
		TsConfigContent: []byte("{}"),
		PkgJsonContent:  []byte(files["apps/app-1/package.json"]),
		SortedFiles:     allKeys,
		Cwd:             cwd,
	}

	manager := NewResolverManager(true, []string{"import", "default", "node"}, rootParams, []GlobMatcher{})
	resolver := manager.GetResolverForFile(filepath.Join(cwd, "src/main.ts"))

	// Test 1: Resolve @company/lib-a/utils
	path1, rtype1, resErr := resolver.ResolveModule("@company/lib-a/utils", filepath.Join(cwd, "src/main.ts"))
	if resErr != nil {
		t.Errorf("Failed to resolve @company/lib-a/utils: %v", *resErr)
	}
	if rtype1 != MonorepoModule {
		t.Errorf("Expected MonorepoModule, got %v", rtype1)
	}
	expectedPath1 := filepath.Join(tmpDir, "packages/lib-a/src/utils.ts")
	if path1 != expectedPath1 {
		t.Errorf("Expected %s, got %s", expectedPath1, path1)
	}

	// Test 2: Resolve @company/lib-a (main export)
	path2, rtype2, resErr2 := resolver.ResolveModule("@company/lib-a", filepath.Join(cwd, "src/main.ts"))
	if resErr2 != nil {
		t.Errorf("Failed to resolve @company/lib-a: %v", resErr2)
	}
	if rtype2 != MonorepoModule {
		t.Errorf("Expected MonorepoModule, got %v", rtype2)
	}
	expectedPath2 := filepath.Join(tmpDir, "packages/lib-a/src/index.ts")
	if path2 != expectedPath2 {
		t.Errorf("Expected %s, got %s", expectedPath2, path2)
	}

	// Test 3: Resolve @company/lib-b (string export sugar)
	path3, rtype3, resErr3 := resolver.ResolveModule("@company/lib-b", filepath.Join(cwd, "src/main.ts"))
	if resErr3 != nil {
		t.Errorf("Failed to resolve @company/lib-b: %v", resErr3)
	}
	if rtype3 != MonorepoModule {
		t.Errorf("Expected MonorepoModule, got %v", rtype3)
	}
	expectedPath3 := filepath.Join(tmpDir, "packages/lib-b/dist/main.js")
	if path3 != expectedPath3 {
		t.Errorf("Expected %s, got %s", expectedPath3, path3)
	}
}

func TestDependencyValidation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-monorepo-validation")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json":                     `{ "workspaces": ["packages/*"] }`,
		"packages/lib-secret/package.json": `{ "name": "@company/secret", "exports": "./index.js" }`,
		"packages/lib-secret/index.js":     `secret`,
		"packages/app/package.json":        `{ "name": "app", "dependencies": { "other": "1.0.0" } }`,
		"packages/app/index.ts":            `import secret from '@company/secret'`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	cwd := filepath.Join(tmpDir, "packages/app")

	rootParams := RootParams{
		TsConfigContent: []byte("{}"),
		PkgJsonContent:  []byte(files["packages/app/package.json"]),
		SortedFiles:     []string{filepath.Join(cwd, "index.ts")},
		Cwd:             cwd,
	}

	manager := NewResolverManager(true, []string{"import", "default", "node"}, rootParams, []GlobMatcher{})
	resolver := manager.GetResolverForFile(filepath.Join(cwd, "index.ts"))

	_, _, resErr := resolver.ResolveModule("@company/secret", filepath.Join(cwd, "index.ts"))

	if resErr == nil || *resErr != AliasNotResolved {
		t.Errorf("Expected AliasNotResolved for invalid dep, got %v", *resErr)
	}
}

func TestMonorepoSubpackageExports(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-monorepo-subpkg")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json": `{ "workspaces": ["packages/*", "apps/*"] }`,
		"packages/common/package.json": `{
			"name": "@company/common",
			"exports": {
				"./file-utils": {
					"node": "./dist/file-utils.js",
					"default": "./src/file-utils.ts"
				}
			},
			"imports": {
				"#common/*.ts": {
					"default": "./src/*.ts"
				}
			}
		}`,
		"packages/common/src/file-utils.ts": `export const file = 1;`,
		"apps/app/package.json": `{
			"name": "app",
			"dependencies": {
				"@company/common": "*"
			}
		}`,
		"apps/app/src/index.ts": `import { file } from "@company/common/file-utils";`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	cwd := filepath.Join(tmpDir, "apps/app")

	allKeys := []string{}
	for k := range files {
		if filepath.Ext(k) == ".ts" || filepath.Ext(k) == ".js" || filepath.Ext(k) == ".json" {
			allKeys = append(allKeys, filepath.Join(tmpDir, k))
		}
	}

	rootParams := RootParams{
		TsConfigContent: []byte("{}"),
		PkgJsonContent:  []byte(files["apps/app/package.json"]),
		SortedFiles:     allKeys,
		Cwd:             cwd,
	}

	manager := NewResolverManager(true, []string{"import", "default", "node"}, rootParams, []GlobMatcher{})
	resolver := manager.GetResolverForFile(filepath.Join(cwd, "src/index.ts"))

	// Test 1: External import via exports
	p, rtype, resErr := resolver.ResolveModule("@company/common/file-utils", filepath.Join(cwd, "src/index.ts"))
	if resErr != nil {
		t.Errorf("Expected nil error for exports resolution, got %v", *resErr)
	}
	if rtype != MonorepoModule {
		t.Errorf("Expected MonorepoModule, got %v", rtype)
	}

	expected := filepath.Join(tmpDir, "packages/common/src/file-utils.ts")
	if p != expected {
		t.Errorf("Expected %s, got %s", expected, p)
	}

	// Test 2: Internal self-import via imports field
	commonFile := filepath.Join(tmpDir, "packages/common/src/file-utils.ts")
	commonResolver := manager.GetResolverForFile(commonFile)

	p2, rtype2, resErr2 := commonResolver.ResolveModule("#common/file-utils.ts", commonFile)
	if resErr2 != nil {
		t.Errorf("Expected nil error for imports resolution, got %v", resErr2)
	}
	if rtype2 != UserModule {
		t.Errorf("Expected UserModule, got %v", rtype2)
	}
	if p2 != commonFile {
		t.Errorf("Expected %s, got %s", commonFile, p2)
	}
}

func TestMonorepoRelaxedAndAliases(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-monorepo-relaxed")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json": `{ "workspaces": ["packages/*"] }`,
		"packages/lib/package.json": `{
			"name": "@company/lib",
			"version": "1.0.0",
			"main": "./src/index.ts"
		}`,
		"packages/lib/src/index.ts": `export const a = 1;`,
		"packages/app/package.json": `{
			"name": "app",
			"dependencies": {
				"@company/lib": "1.0.0"
			}
		}`,
		"packages/app/tsconfig.json": `{
			"compilerOptions": {
				"paths": {
					"@lib/*": ["../lib/src/*"]
				}
			}
		}`,
		"packages/app/src/index.ts": `import { a } from "@lib/index";`,
	}

	for path, content := range files {
		fullPath := NormalizePathForInternal(filepath.Join(tmpDir, path))
		os.MkdirAll(filepath.Dir(DenormalizePathForOS(fullPath)), 0755)
		os.WriteFile(DenormalizePathForOS(fullPath), []byte(content), 0644)
	}

	appDir := NormalizePathForInternal(filepath.Join(tmpDir, "packages/app"))

	allKeys := []string{}
	for k := range files {
		if filepath.Ext(k) == ".ts" {
			allKeys = append(allKeys, NormalizePathForInternal(filepath.Join(tmpDir, k)))
		}
	}

	rootParams := RootParams{
		TsConfigContent: []byte(files["packages/app/tsconfig.json"]),
		PkgJsonContent:  []byte(files["packages/app/package.json"]),
		SortedFiles:     allKeys,
		Cwd:             appDir,
	}

	manager := NewResolverManager(true, []string{"import", "default", "node"}, rootParams, []GlobMatcher{})
	resolver := manager.GetResolverForFile(NormalizePathForInternal(filepath.Join(appDir, "src/index.ts")))

	// Test 1: Resolve via alias but should be MonorepoModule
	path, rtype, resErr := resolver.ResolveModule("@lib/index", NormalizePathForInternal(filepath.Join(appDir, "src/index.ts")))
	if resErr != nil {
		t.Errorf("Expected nil error, got %v", *resErr)
	}
	if rtype != UserModule {
		t.Errorf("Expected UserModule for alias pointing to workspace, got %v", rtype)
	}
	expected := NormalizePathForInternal(filepath.Join(tmpDir, "packages/lib/src/index.ts"))
	if path != expected {
		t.Errorf("Expected %s, got %s", expected, path)
	}

	// Test 2: Resolve via name with non-workspace version in package.json
	path2, rtype2, resErr2 := resolver.ResolveModule("@company/lib", NormalizePathForInternal(filepath.Join(appDir, "src/index.ts")))
	if resErr2 != nil {
		t.Errorf("Expected nil error for name resolution, got %v", resErr2)
	}
	if rtype2 != MonorepoModule {
		t.Errorf("Expected MonorepoModule for relaxed version check, got %v", rtype2)
	}
	if path2 == "" {
		t.Errorf("Expected non-empty path for name resolution")
	}
}

func TestMonorepoInternalImportsAlias(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-monorepo-internal-imports")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json": `{ "workspaces": ["packages/*", "apps/*"] }`,
		"packages/common/package.json": `{
			"name": "@company/common",
			"exports": {
				".": "./src/index.ts",
				"./file-utils": "./src/file-utils.ts"
			},
			"imports": {
				"#common/*": "./src/*"
			}
		}`,
		"packages/common/src/index.ts":      `export { helper } from "#common/helpers.ts";`,
		"packages/common/src/helpers.ts":    `export const helper = () => {};`,
		"packages/common/src/file-utils.ts": `import { helper } from "#common/helpers.ts"; export const file = helper();`,
		"apps/app/package.json": `{
			"name": "app",
			"dependencies": {
				"@company/common": "*"
			}
		}`,
		"apps/app/src/index.ts": `import { file } from "@company/common/file-utils";`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	cwd := filepath.Join(tmpDir, "apps/app")

	allKeys := []string{}
	for k := range files {
		if filepath.Ext(k) == ".ts" {
			allKeys = append(allKeys, NormalizePathForInternal(filepath.Join(tmpDir, k)))
		}
	}

	rootParams := RootParams{
		TsConfigContent: []byte("{}"),
		PkgJsonContent:  []byte(files["apps/app/package.json"]),
		SortedFiles:     allKeys,
		Cwd:             cwd,
	}

	manager := NewResolverManager(true, []string{"import", "default"}, rootParams, []GlobMatcher{})

	// Test 1: Resolve @company/common/file-utils from apps/app
	appFile := NormalizePathForInternal(filepath.Join(cwd, "src/index.ts"))
	resolver := manager.GetResolverForFile(appFile)

	p, rtype, resErr := resolver.ResolveModule("@company/common/file-utils", appFile)
	if resErr != nil {
		t.Errorf("Expected nil error for exports resolution, got %v", *resErr)
	}
	if rtype != MonorepoModule {
		t.Errorf("Expected MonorepoModule, got %v", rtype)
	}
	expectedFileUtils := NormalizePathForInternal(filepath.Join(tmpDir, "packages/common/src/file-utils.ts"))
	if p != expectedFileUtils {
		t.Errorf("Expected %s, got %s", expectedFileUtils, p)
	}

	// Test 2: Resolve #common/helpers.ts from within packages/common/src/file-utils.ts
	commonFileUtilsPath := NormalizePathForInternal(filepath.Join(tmpDir, "packages/common/src/file-utils.ts"))
	commonResolver := manager.GetResolverForFile(commonFileUtilsPath)

	p2, rtype2, resErr2 := commonResolver.ResolveModule("#common/helpers.ts", commonFileUtilsPath)
	if resErr2 != nil {
		t.Errorf("Expected nil error for internal #common import, got %v", resErr2)
	}

	if rtype2 != UserModule {
		t.Errorf("Expected UserModule for internal import, got %v", rtype2)
	}

	expectedHelpers := NormalizePathForInternal(filepath.Join(tmpDir, "packages/common/src/helpers.ts"))
	if p2 != expectedHelpers {
		t.Errorf("Expected %s, got %s", expectedHelpers, p2)
	}
}

// TestMonorepoInternalTsconfigAlias tests that tsconfig path aliases within a workspace
// package are correctly resolved when traversing into that package from another package.
func TestMonorepoInternalTsconfigAlias(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-monorepo-internal-tsconfig")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json": `{ "workspaces": ["packages/*", "apps/*"] }`,
		"packages/common/package.json": `{
			"name": "@company/common",
			"exports": {
				".": "./src/index.ts",
				"./utils": "./src/utils.ts"
			}
		}`,
		"packages/common/tsconfig.json": `{
			"compilerOptions": {
				"baseUrl": ".",
				"paths": {
					"@internal/*": ["./src/internal/*"]
				}
			}
		}`,
		"packages/common/src/index.ts":         `export { internalFn } from "@internal/core";`,
		"packages/common/src/utils.ts":         `import { internalFn } from "@internal/core"; export const util = internalFn();`,
		"packages/common/src/internal/core.ts": `export const internalFn = () => "core";`,
		"apps/app/package.json": `{
			"name": "app",
			"dependencies": {
				"@company/common": "*"
			}
		}`,
		"apps/app/src/index.ts": `import { util } from "@company/common/utils";`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	cwd := filepath.Join(tmpDir, "apps/app")

	allKeys := []string{}
	for k := range files {
		if filepath.Ext(k) == ".ts" {
			allKeys = append(allKeys, NormalizePathForInternal(filepath.Join(tmpDir, k)))
		}
	}

	rootParams := RootParams{
		TsConfigContent: []byte("{}"),
		PkgJsonContent:  []byte(files["apps/app/package.json"]),
		SortedFiles:     allKeys,
		Cwd:             cwd,
	}

	manager := NewResolverManager(true, []string{"import", "default"}, rootParams, []GlobMatcher{})

	// Test 1: Resolve @company/common/utils from apps/app
	appFile := NormalizePathForInternal(filepath.Join(cwd, "src/index.ts"))
	resolver := manager.GetResolverForFile(appFile)

	p, rtype, resErr := resolver.ResolveModule("@company/common/utils", appFile)
	if resErr != nil {
		t.Errorf("Expected nil error for exports resolution, got %v", *resErr)
	}
	if rtype != MonorepoModule {
		t.Errorf("Expected MonorepoModule, got %v", rtype)
	}
	expectedUtils := NormalizePathForInternal(filepath.Join(tmpDir, "packages/common/src/utils.ts"))
	if p != expectedUtils {
		t.Errorf("Expected %s, got %s", expectedUtils, p)
	}

	// Test 2: Resolve @internal/core from within packages/common/src/utils.ts
	commonUtilsPath := NormalizePathForInternal(filepath.Join(tmpDir, "packages/common/src/utils.ts"))
	commonResolver := manager.GetResolverForFile(commonUtilsPath)

	p2, rtype2, resErr2 := commonResolver.ResolveModule("@internal/core", commonUtilsPath)
	if resErr2 != nil {
		t.Errorf("Expected nil error for internal tsconfig alias, got %v", resErr2)
	}
	// Note: Internal tsconfig aliases within the same package resolve as UserModule,
	// not MonorepoModule, because the resolved path stays within the same package.
	if rtype2 != UserModule {
		t.Errorf("Expected UserModule for intra-package tsconfig alias, got %v", rtype2)
	}
	expectedCore := NormalizePathForInternal(filepath.Join(tmpDir, "packages/common/src/internal/core.ts"))
	if p2 != expectedCore {
		t.Errorf("Expected %s, got %s", expectedCore, p2)
	}
}

func TestWorkspaceDependencyVariations(t *testing.T) {
	// Verify that different ways of specifying workspace dependencies are supported
	// as long as the package exists in the workspace.
	tmpDir, err := os.MkdirTemp("", "rev-dep-monorepo-variations")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json": `{ "workspaces": ["packages/*"] }`,
		"packages/target/package.json": `{
			"name": "@pkg/target",
			"version": "1.5.0",
			"main": "index.ts"
		}`,
		"packages/target/index.ts": `export const val = 1;`,
		"packages/app/package.json": `{
			"name": "@pkg/app",
			"dependencies": {
				"a": "*",
				"b": "^",
				"c": "~",
				"d": "1.5.0",
				"e": "^1.5.0",
				"f": "workspace:1.5.0",
				"g": "workspace:^1.5.0",
				"h": "workspace:*",
				"i": "workspace:^",
				"j": "workspace:~"
			}
		}`,
		"packages/app/index.ts": `
			import { val as a } from "a";
			import { val as b } from "b";
			import { val as c } from "c";
			import { val as d } from "d";
			import { val as e } from "e";
			import { val as f } from "f";
			import { val as g } from "g";
			import { val as h } from "h";
			import { val as i } from "i";
			import { val as j } from "j";
		`,
	}

	variations := map[string]string{
		"star":                  "*",
		"caret-empty":           "^",
		"tilde-empty":           "~",
		"exact":                 "1.5.0",
		"caret-ver":             "^1.5.0",
		"workspace-exact":       "workspace:1.5.0",
		"workspace-caret":       "workspace:^1.5.0",
		"workspace-star":        "workspace:*",
		"workspace-caret-empty": "workspace:^",
		"workspace-tilde-empty": "workspace:~",
	}

	files["package.json"] = `{ "workspaces": ["packages/*", "apps/*"] }`
	// Target package
	files["packages/target/package.json"] = `{ "name": "@pkg/target", "version": "1.5.0", "main": "index.ts" }`
	files["packages/target/index.ts"] = `export const val = 1;`

	// Generate apps
	sortedApps := []string{}
	for name, version := range variations {
		appName := "app-" + name
		pkgPath := "apps/" + appName + "/package.json"
		srcPath := "apps/" + appName + "/index.ts"

		files[pkgPath] = fmt.Sprintf(`{
			"name": "%s",
			"dependencies": {
				"@pkg/target": "%s"
			}
		}`, appName, version)

		files[srcPath] = `import { val } from "@pkg/target";`
		sortedApps = append(sortedApps, appName)
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	cwd := NormalizePathForInternal(tmpDir)
	monorepoCtx := DetectMonorepo(cwd)
	if monorepoCtx == nil {
		t.Fatalf("Failed to detect monorepo")
	}
	monorepoCtx.FindWorkspacePackages(monorepoCtx.WorkspaceRoot, []GlobMatcher{})

	// Verify target is found
	if _, ok := monorepoCtx.PackageToPath["@pkg/target"]; !ok {
		t.Fatalf("@pkg/target not found in workspace")
	}

	targetPath := NormalizePathForInternal(filepath.Join(tmpDir, "packages/target/index.ts"))

	for _, appName := range sortedApps {
		appPath := filepath.Join(tmpDir, "apps", appName)

		// Setup manager for this app
		rootParams := RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(files["apps/"+appName+"/package.json"]),
			SortedFiles: []string{
				NormalizePathForInternal(filepath.Join(appPath, "index.ts")),
				targetPath,
			},
			Cwd: appPath,
		}

		manager := NewResolverManager(true, []string{"import"}, rootParams, []GlobMatcher{})
		resolver := manager.GetResolverForFile(NormalizePathForInternal(filepath.Join(appPath, "index.ts")))

		path, rtype, resErr := resolver.ResolveModule("@pkg/target", NormalizePathForInternal(filepath.Join(appPath, "index.ts")))

		if resErr != nil {
			t.Errorf("[%s] Resolution failed: %v", appName, *resErr)
			continue
		}
		if rtype != MonorepoModule {
			t.Errorf("[%s] Expected MonorepoModule, got %v", appName, rtype)
		}
		if path != targetPath {
			t.Errorf("[%s] Expected path %s, got %s", appName, targetPath, path)
		}
	}
}

func TestMonorepoImportAliasToWorkspacePackage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rev-dep-monorepo-internal-imports")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files := map[string]string{
		"package.json": `{ "workspaces": ["packages/*", "apps/*"] }`,
		"packages/common/package.json": `{
			"name": "@company/common",
			"exports": {
				".": "./src/index.ts",
				"./file-utils": "./src/file-utils.ts"
			}
		}`,
		"packages/common/src/index.ts":      `export { helper } from "./src/helpers.ts";`,
		"packages/common/src/helpers.ts":    `export const helper = () => {};`,
		"packages/common/src/file-utils.ts": `import { helper } from "./src/helpers.ts"; export const file = helper();`,
		"apps/app/package.json": `{
			"name": "app",
			"dependencies": {
				"@company/common": "*"
			},
			"imports": {
				"#common-pkg" : "@company/common",
				"#common-pkg-file-utils" : "@company/common/file-utils",
				"#common-pkg-wildcard/*" : "@company/common/*"
			}
		}`,
		"apps/app/src/index.ts": `import { file } from "@company/common/file-utils";`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	cwd := filepath.Join(tmpDir, "apps/app")

	allKeys := []string{}
	for k := range files {
		if filepath.Ext(k) == ".ts" {
			allKeys = append(allKeys, NormalizePathForInternal(filepath.Join(tmpDir, k)))
		}
	}

	rootParams := RootParams{
		TsConfigContent: []byte("{}"),
		PkgJsonContent:  []byte(files["apps/app/package.json"]),
		SortedFiles:     allKeys,
		Cwd:             cwd,
	}

	manager := NewResolverManager(true, []string{"import", "default"}, rootParams, []GlobMatcher{})

	// Test 1: Resolve #common-pkg-file-utils from apps/app
	appFile := NormalizePathForInternal(filepath.Join(cwd, "src/index.ts"))
	resolver := manager.GetResolverForFile(appFile)

	p, rtype, resErr := resolver.ResolveModule("#common-pkg-file-utils", appFile)
	if resErr != nil {
		t.Errorf("Expected nil error, got %v", *resErr)
	}
	if rtype != MonorepoModule {
		t.Errorf("Expected MonorepoModule, got %v", rtype)
	}
	expectedFileUtils := NormalizePathForInternal(filepath.Join(tmpDir, "packages/common/src/file-utils.ts"))
	if p != expectedFileUtils {
		t.Errorf("Expected %s, got %s", expectedFileUtils, p)
	}

	// Test 2: Resolve #common-pkg from apps/app
	appFile = NormalizePathForInternal(filepath.Join(cwd, "src/index.ts"))
	resolver = manager.GetResolverForFile(appFile)

	p2, rtype2, resErr2 := resolver.ResolveModule("#common-pkg", appFile)
	if resErr2 != nil {
		t.Errorf("Expected nil error, got %v", resErr2)
	}
	if rtype2 != MonorepoModule {
		t.Errorf("Expected MonorepoModule for internal import, got %v", rtype2)
	}
	expectedHelpers := NormalizePathForInternal(filepath.Join(tmpDir, "packages/common/src/index.ts"))
	if p2 != expectedHelpers {
		t.Errorf("Expected %s, got %s", expectedHelpers, p2)
	}

	// Test 3: Resolve #common-pkg-wildcard/* from apps/app
	appFile = NormalizePathForInternal(filepath.Join(cwd, "src/index.ts"))
	resolver = manager.GetResolverForFile(appFile)

	p2, rtype2, resErr2 = resolver.ResolveModule("#common-pkg-wildcard/file-utils", appFile)
	if resErr2 != nil {
		t.Errorf("Expected nil error, got %v", resErr2)
	}
	if rtype2 != MonorepoModule {
		t.Errorf("Expected MonorepoModule for internal import, got %v", rtype2)
	}
	expectedHelpers = NormalizePathForInternal(filepath.Join(tmpDir, "packages/common/src/file-utils.ts"))
	if p2 != expectedHelpers {
		t.Errorf("Expected %s, got %s", expectedHelpers, p2)
	}

	// Test 4: Resolve #common-pkg-wildcard/* from apps/app for not existing file should fail
	appFile = NormalizePathForInternal(filepath.Join(cwd, "src/index.ts"))
	resolver = manager.GetResolverForFile(appFile)

	p2, rtype2, resErr2 = resolver.ResolveModule("#common-pkg-wildcard/not-existing-file", appFile)

	if resErr2 != nil && *resErr2 != FileNotFound {
		t.Errorf("Expected FileNotFound error, got %v", resErr2)
	}

	if resErr2 == nil {
		t.Errorf("Expected FileNotFound error, got nil")
	}

	if rtype2 != UserModule {
		t.Errorf("Expected UserModule for internal import, got %v", rtype2)
	}
	if p2 != "@company/common/not-existing-file" {
		t.Errorf("Expected @company/common/not-existing-file, got %s", p2)
	}

}
