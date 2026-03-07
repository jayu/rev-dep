package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnresolvedCmdRun(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/configProcessorProject")

	// Run helper directly
	out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, nil, nil)
	if err != nil {
		t.Fatalf("getUnresolvedOutput failed: %v", err)
	}

	if out == "" {
		t.Errorf("Expected non-empty output from getUnresolvedOutput, got empty string")
	}
}

func TestUnresolvedCmdRun_WithIgnoreOptions(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/configProcessorProject")

	t.Run("ignore exact file and import pair", func(t *testing.T) {
		opts := &UnresolvedImportsOptions{
			Enabled: true,
			Ignore: FileValueIgnoreMap{
				"src/index.ts": []string{"non-existent-module"},
			},
		}

		if err := validateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			t.Fatalf("validateUnresolvedImportsOptions failed: %v", err)
		}

		out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, opts, nil)
		if err != nil {
			t.Fatalf("getUnresolvedOutput failed: %v", err)
		}

		if contains(out, "src/index.ts\n  - non-existent-module\n") {
			t.Errorf("Expected src/index.ts -> non-existent-module to be filtered out")
		}
		if !contains(out, "packages/subpkg/src/broken-import.ts\n  - non-existent-pkg\n") {
			t.Errorf("Expected non-existent-pkg unresolved import to remain")
		}
	})

	t.Run("ignore files glob", func(t *testing.T) {
		opts := &UnresolvedImportsOptions{
			Enabled:     true,
			IgnoreFiles: []string{"**/broken-import.ts"},
		}

		if err := validateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			t.Fatalf("validateUnresolvedImportsOptions failed: %v", err)
		}

		out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, opts, nil)
		if err != nil {
			t.Fatalf("getUnresolvedOutput failed: %v", err)
		}

		if contains(out, "broken-import.ts") {
			t.Errorf("Expected broken-import.ts unresolved imports to be filtered out")
		}
		if !contains(out, "src/index.ts\n  - non-existent-module\n") {
			t.Errorf("Expected unresolved import from src/index.ts to remain")
		}
	})

	t.Run("ignore map supports glob", func(t *testing.T) {
		opts := &UnresolvedImportsOptions{
			Enabled: true,
			Ignore: FileValueIgnoreMap{
				"**/broken-import.ts": []string{"non-existent-pkg"},
			},
		}

		if err := validateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			t.Fatalf("validateUnresolvedImportsOptions failed: %v", err)
		}

		out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, opts, nil)
		if err != nil {
			t.Fatalf("getUnresolvedOutput failed: %v", err)
		}

		if contains(out, "broken-import.ts\n  - non-existent-pkg\n") {
			t.Errorf("Expected unresolved import to be filtered out by glob ignore map")
		}
	})

	t.Run("ignore map supports glob import value", func(t *testing.T) {
		opts := &UnresolvedImportsOptions{
			Enabled: true,
			Ignore: FileValueIgnoreMap{
				"src/index.ts": []string{"non-existent-*"},
			},
		}

		if err := validateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			t.Fatalf("validateUnresolvedImportsOptions failed: %v", err)
		}

		out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, opts, nil)
		if err != nil {
			t.Fatalf("getUnresolvedOutput failed: %v", err)
		}

		if contains(out, "src/index.ts\n  - non-existent-module\n") {
			t.Errorf("Expected unresolved import to be filtered out by glob ignore value")
		}
	})

	t.Run("ignore imports globally", func(t *testing.T) {
		opts := &UnresolvedImportsOptions{
			Enabled:       true,
			IgnoreImports: []string{"non-existent-module", "non-existent-pkg"},
		}

		if err := validateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			t.Fatalf("validateUnresolvedImportsOptions failed: %v", err)
		}

		out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, opts, nil)
		if err != nil {
			t.Fatalf("getUnresolvedOutput failed: %v", err)
		}

		if out != "" {
			t.Errorf("Expected empty output after ignoring all known unresolved imports, got: %s", out)
		}
	})

	t.Run("ignore imports supports glob", func(t *testing.T) {
		opts := &UnresolvedImportsOptions{
			Enabled:       true,
			IgnoreImports: []string{"non-existent-*"},
		}

		if err := validateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			t.Fatalf("validateUnresolvedImportsOptions failed: %v", err)
		}

		out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, opts, nil)
		if err != nil {
			t.Fatalf("getUnresolvedOutput failed: %v", err)
		}

		if out != "" {
			t.Errorf("Expected empty output after glob ignoreImports, got: %s", out)
		}
	})
}

func TestUnresolvedCmdRun_WithCustomAssetExtensions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rev-dep-unresolved-custom-assets")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	if err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name":"test-project"}`), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(`{"compilerOptions":{"module":"esnext"}}`), 0644); err != nil {
		t.Fatalf("failed to write tsconfig.json: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "src"), 0755); err != nil {
		t.Fatalf("failed to create src directory: %v", err)
	}

	indexFile := "import logo from './logo.custom';\nconsole.log(logo);\n"
	if err := os.WriteFile(filepath.Join(tempDir, "src", "index.ts"), []byte(indexFile), 0644); err != nil {
		t.Fatalf("failed to write index.ts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "src", "logo.custom"), []byte("asset"), 0644); err != nil {
		t.Fatalf("failed to write custom asset file: %v", err)
	}

	outDefault, err := getUnresolvedOutput(tempDir, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, nil, nil)
	if err != nil {
		t.Fatalf("getUnresolvedOutput failed: %v", err)
	}
	if !contains(outDefault, "src/index.ts\n  - ./logo.custom\n") {
		t.Fatalf("expected custom asset import to be unresolved without custom extension, got: %s", outDefault)
	}

	outCustom, err := getUnresolvedOutput(tempDir, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, nil, []string{"custom"})
	if err != nil {
		t.Fatalf("getUnresolvedOutput failed: %v", err)
	}
	if contains(outCustom, "./logo.custom") {
		t.Fatalf("expected custom asset import to be resolved with custom extension, got: %s", outCustom)
	}
}

func TestValidateCustomAssetExtensions(t *testing.T) {
	if err := validateCustomAssetExtensions([]string{"mp3", "glb"}, "unresolved.customAssetExtensions"); err != nil {
		t.Fatalf("expected valid custom asset extensions, got: %v", err)
	}
	if err := validateCustomAssetExtensions([]string{"d.ts"}, "unresolved.customAssetExtensions"); err != nil {
		t.Fatalf("expected extension containing dot to be valid, got: %v", err)
	}

	if err := validateCustomAssetExtensions([]string{".mp3"}, "unresolved.customAssetExtensions"); err == nil {
		t.Fatal("expected dot-prefixed extension to fail validation")
	}
	if err := validateCustomAssetExtensions([]string{"  mp3  "}, "unresolved.customAssetExtensions"); err == nil {
		t.Fatal("expected extension with surrounding spaces to fail validation")
	}
}
