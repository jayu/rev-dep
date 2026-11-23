package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rev-dep-go/internal/config"
	"rev-dep-go/internal/diag"
	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/model"
	"rev-dep-go/internal/resolve"
	"rev-dep-go/internal/testutil"
)

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestUnresolvedCmdRun(t *testing.T) {
	testCwd, err := testutil.FixturePath("configProcessorProject")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}

	// Run helper directly
	out, err := getUnresolvedOutput(testCwd, "tsconfig.json", []string{}, model.FollowMonorepoPackagesValue{FollowAll: true}, nil, nil, nil)
	if err != nil {
		t.Fatalf("getUnresolvedOutput failed: %v", err)
	}

	if out == "" {
		t.Errorf("Expected non-empty output from getUnresolvedOutput, got empty string")
	}
}

func TestUnresolvedCmdRun_TsconfigOptionReplacement(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"package.json": `{"name":"multiple-tsconfig-parents","private":true}`,
		"configs/first.json": `{
			"extends":"./shared.json",
			"compilerOptions":{"paths":{"@first/*":["../src/first/*"]}}
		}`,
		"configs/second.json": `{
			"extends":["./shared.json"],
			"compilerOptions":{"paths":{"@second/*":["../src/second/*"]}}
		}`,
		"configs/shared.json":  `{"compilerOptions":{"paths":{"@shared/*":["../src/shared/*"]}}}`,
		"configs/inherit.json": `{"extends":"./shared.json"}`,
		"src/index.ts": `import '@first/value';
import '@second/value';
import '@shared/value';
import '@child/value';
import '@missing';
`,
		"src/first/value.ts":  `export const value = 1;`,
		"src/second/value.ts": `export const value = 2;`,
		"src/shared/value.ts": `export const value = 3;`,
		"src/child/value.ts":  `export const value = 4;`,
	}
	for name, content := range files {
		filename := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name       string
		config     string
		unresolved []string
	}{
		{
			name:       "later parent replaces earlier aliases",
			config:     `{"extends":["./configs/first.json","./configs/second.json"]}`,
			unresolved: []string{"@first/value", "@shared/value", "@child/value", "@missing"},
		},
		{
			name:       "child replaces all parent aliases",
			config:     `{"extends":["./configs/first.json","./configs/second.json"],"compilerOptions":{"paths":{"@child/*":["src/child/*"]}}}`,
			unresolved: []string{"@first/value", "@second/value", "@shared/value", "@missing"},
		},
		{
			name:       "empty child paths clear inherited aliases",
			config:     `{"extends":["./configs/first.json","./configs/second.json"],"compilerOptions":{"paths":{}}}`,
			unresolved: []string{"@first/value", "@second/value", "@shared/value", "@child/value", "@missing"},
		},
		{
			name:       "omitted child paths inherit the effective parent aliases",
			config:     `{"extends":["./configs/first.json","./configs/second.json"],"compilerOptions":{"strict":true}}`,
			unresolved: []string{"@first/value", "@shared/value", "@child/value", "@missing"},
		},
		{
			name:       "string inheritance replaces base aliases",
			config:     `{"extends":"./configs/first.json"}`,
			unresolved: []string{"@second/value", "@shared/value", "@child/value", "@missing"},
		},
		{
			name:       "child replaces aliases from a string parent",
			config:     `{"extends":"./configs/first.json","compilerOptions":{"paths":{"@child/*":["src/child/*"]}}}`,
			unresolved: []string{"@first/value", "@second/value", "@shared/value", "@missing"},
		},
		{
			name:       "shared ancestor aliases are inherited independently",
			config:     `{"extends":["./configs/first.json","./configs/inherit.json"]}`,
			unresolved: []string{"@first/value", "@second/value", "@child/value", "@missing"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(root, "tsconfig.json"), []byte(tt.config), 0644); err != nil {
				t.Fatal(err)
			}
			diag.SetVerbose(true)
			t.Cleanup(func() { diag.SetVerbose(false) })
			var out string
			warnings, err := captureOutput(func() error {
				var err error
				out, err = getUnresolvedOutput(root, "tsconfig.json", nil, model.FollowMonorepoPackagesValue{}, nil, nil, nil)
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			if warnings != "" {
				t.Fatalf("unexpected warnings for valid parent configs: %s", warnings)
			}
			want := "src/index.ts\n  - " + strings.Join(tt.unresolved, "\n  - ") + "\n"
			if out != want {
				t.Fatalf("unresolved imports = %q, want %q", out, want)
			}
		})
	}
}

func TestUnresolvedCmdRun_WithIgnoreOptions(t *testing.T) {
	testCwd, err := testutil.FixturePath("configProcessorProject")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}

	t.Run("ignore exact file and import pair", func(t *testing.T) {
		opts := &config.UnresolvedImportsOptions{
			Enabled: true,
			Ignore: globutil.FileValueIgnoreMap{
				"src/index.ts": []string{"non-existent-module"},
			},
		}

		if err := config.ValidateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			t.Fatalf("validateUnresolvedImportsOptions failed: %v", err)
		}

		out, err := getUnresolvedOutput(testCwd, "tsconfig.json", []string{}, model.FollowMonorepoPackagesValue{FollowAll: true}, opts, nil, nil)
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
		opts := &config.UnresolvedImportsOptions{
			Enabled:     true,
			IgnoreFiles: []string{"**/broken-import.ts"},
		}

		if err := config.ValidateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			t.Fatalf("validateUnresolvedImportsOptions failed: %v", err)
		}

		out, err := getUnresolvedOutput(testCwd, "tsconfig.json", []string{}, model.FollowMonorepoPackagesValue{FollowAll: true}, opts, nil, nil)
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
		opts := &config.UnresolvedImportsOptions{
			Enabled: true,
			Ignore: globutil.FileValueIgnoreMap{
				"**/broken-import.ts": []string{"non-existent-pkg"},
			},
		}

		if err := config.ValidateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			t.Fatalf("validateUnresolvedImportsOptions failed: %v", err)
		}

		out, err := getUnresolvedOutput(testCwd, "tsconfig.json", []string{}, model.FollowMonorepoPackagesValue{FollowAll: true}, opts, nil, nil)
		if err != nil {
			t.Fatalf("getUnresolvedOutput failed: %v", err)
		}

		if contains(out, "broken-import.ts\n  - non-existent-pkg\n") {
			t.Errorf("Expected unresolved import to be filtered out by glob ignore map")
		}
	})

	t.Run("ignore map supports glob import value", func(t *testing.T) {
		opts := &config.UnresolvedImportsOptions{
			Enabled: true,
			Ignore: globutil.FileValueIgnoreMap{
				"src/index.ts": []string{"non-existent-*"},
			},
		}

		if err := config.ValidateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			t.Fatalf("validateUnresolvedImportsOptions failed: %v", err)
		}

		out, err := getUnresolvedOutput(testCwd, "tsconfig.json", []string{}, model.FollowMonorepoPackagesValue{FollowAll: true}, opts, nil, nil)
		if err != nil {
			t.Fatalf("getUnresolvedOutput failed: %v", err)
		}

		if contains(out, "src/index.ts\n  - non-existent-module\n") {
			t.Errorf("Expected unresolved import to be filtered out by glob ignore value")
		}
	})

	t.Run("ignore imports globally", func(t *testing.T) {
		opts := &config.UnresolvedImportsOptions{
			Enabled:       true,
			IgnoreImports: []string{"non-existent-module", "non-existent-pkg"},
		}

		if err := config.ValidateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			t.Fatalf("validateUnresolvedImportsOptions failed: %v", err)
		}

		out, err := getUnresolvedOutput(testCwd, "tsconfig.json", []string{}, model.FollowMonorepoPackagesValue{FollowAll: true}, opts, nil, nil)
		if err != nil {
			t.Fatalf("getUnresolvedOutput failed: %v", err)
		}

		if out != "" {
			t.Errorf("Expected empty output after ignoring all known unresolved imports, got: %s", out)
		}
	})

	t.Run("ignore imports supports glob", func(t *testing.T) {
		opts := &config.UnresolvedImportsOptions{
			Enabled:       true,
			IgnoreImports: []string{"non-existent-*"},
		}

		if err := config.ValidateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			t.Fatalf("validateUnresolvedImportsOptions failed: %v", err)
		}

		out, err := getUnresolvedOutput(testCwd, "tsconfig.json", []string{}, model.FollowMonorepoPackagesValue{FollowAll: true}, opts, nil, nil)
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

	outDefault, err := getUnresolvedOutput(tempDir, "tsconfig.json", []string{}, model.FollowMonorepoPackagesValue{FollowAll: true}, nil, nil, nil)
	if err != nil {
		t.Fatalf("getUnresolvedOutput failed: %v", err)
	}
	if !contains(outDefault, "src/index.ts\n  - ./logo.custom\n") {
		t.Fatalf("expected custom asset import to be unresolved without custom extension, got: %s", outDefault)
	}

	outCustom, err := getUnresolvedOutput(tempDir, "tsconfig.json", []string{}, model.FollowMonorepoPackagesValue{FollowAll: true}, nil, []string{"custom"}, nil)
	if err != nil {
		t.Fatalf("getUnresolvedOutput failed: %v", err)
	}
	if contains(outCustom, "./logo.custom") {
		t.Fatalf("expected custom asset import to be resolved with custom extension, got: %s", outCustom)
	}
}

func TestValidateCustomAssetExtensions(t *testing.T) {
	if err := resolve.ValidateCustomAssetExtensions([]string{"mp3", "glb"}, "unresolved.customAssetExtensions"); err != nil {
		t.Fatalf("expected valid custom asset extensions, got: %v", err)
	}
	if err := resolve.ValidateCustomAssetExtensions([]string{"d.ts"}, "unresolved.customAssetExtensions"); err != nil {
		t.Fatalf("expected extension containing dot to be valid, got: %v", err)
	}

	if err := resolve.ValidateCustomAssetExtensions([]string{".mp3"}, "unresolved.customAssetExtensions"); err == nil {
		t.Fatal("expected dot-prefixed extension to fail validation")
	}
	if err := resolve.ValidateCustomAssetExtensions([]string{"  mp3  "}, "unresolved.customAssetExtensions"); err == nil {
		t.Fatal("expected extension with surrounding spaces to fail validation")
	}
}
