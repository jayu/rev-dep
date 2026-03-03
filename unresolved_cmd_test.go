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
	out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, nil)
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

		out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, opts)
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

		out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, opts)
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

		out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, opts)
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

		out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, opts)
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

		out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, opts)
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

		out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, FollowMonorepoPackagesValue{FollowAll: true}, opts)
		if err != nil {
			t.Fatalf("getUnresolvedOutput failed: %v", err)
		}

		if out != "" {
			t.Errorf("Expected empty output after glob ignoreImports, got: %s", out)
		}
	})
}
