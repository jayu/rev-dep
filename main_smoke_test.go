package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func captureOutput(fn func() error) (string, error) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	var stdoutBuf, stderrBuf bytes.Buffer
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	os.Stdout = wOut
	os.Stderr = wErr

	// Capture both stdout and stderr
	done := make(chan struct{})
	go func() {
		_, _ = stdoutBuf.ReadFrom(rOut)
		close(done)
	}()

	// Use recover to catch os.Exit calls
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				// os.Exit was called, convert to error
				if exitCode, ok := r.(int); ok {
					err = fmt.Errorf("exit status %d", exitCode)
				} else {
					err = fmt.Errorf("panic: %v", r)
				}
			}
		}()
		err = fn()
	}()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	// Read stderr
	_, _ = stderrBuf.ReadFrom(rErr)

	// Wait for stdout capture to finish
	<-done

	// Combine stdout and stderr for complete output
	output := stdoutBuf.String()
	if stderrBuf.String() != "" {
		output += stderrBuf.String()
	}

	return output, err
}

func TestCircularCmd(t *testing.T) {
	t.Run("circular", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "circularSmoke")

		output, err := captureOutput(func() error {
			// finds standard cycle and path-based cycle (default tsconfig.json)
			_, err := circularCmdFn(mockProjectPath, false, "", "", []string{}, false)
			return err
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "circular.golden")
	})

	t.Run("circular --ignore-type-imports --condition-names node,imports", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "circularMonorepoSmoke")

		output, err := captureOutput(func() error {
			// finds inter-package standard cycle AND condition-based cycle (node)
			_, err := circularCmdFn(mockProjectPath, true, "", "", []string{"node", "imports"}, true)
			return err
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "circular-monorepo-conditions-ignore-type.golden")
	})

	t.Run("circular --package-json custom.package.json --tsconfig-json custom.tsconfig.json", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "circularSmoke")

		output, err := captureOutput(func() error {
			// finds ONLY standard cycle because custom.tsconfig.json lacks paths
			_, err := circularCmdFn(mockProjectPath, false, "custom.package.json", "custom.tsconfig.json", []string{}, false)
			return err
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "circular-custom-config.golden")
	})

	t.Run("circular --follow-monorepo-packages --ignore-type-imports", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "circularMonorepoSmoke")

		output, err := captureOutput(func() error {
			// finds inter-package cycles
			_, err := circularCmdFn(mockProjectPath, true, "", "", []string{}, true)
			return err
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "circular-monorepo-ignore-type.golden")
	})
}

func TestListCwdFiles(t *testing.T) {
	t.Run("list-cwd-files", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return listCwdFilesCmdFn(mockProjectPath, []string{}, []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "list-cwd-files.golden")
	})

	t.Run("list-cwd-files --count", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return listCwdFilesCmdFn(mockProjectPath, []string{}, []string{}, true)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "list-cwd-files-count.golden")
	})

	t.Run("list-cwd-files --include 'src/**/*.ts' --exclude 'src/nested/**'", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return listCwdFilesCmdFn(mockProjectPath, []string{"src/**/*.ts"}, []string{"src/nested/**"}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "list-cwd-files-include-exclude.golden")
	})

	t.Run("list-cwd-files --include 'src/**/*.ts' --exclude '**/*.d.ts' --count", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return listCwdFilesCmdFn(mockProjectPath, []string{"src/**/*.ts"}, []string{"**/*.d.ts"}, true)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "list-cwd-files-complex-count.golden")
	})

	t.Run("list-cwd-files --include '**/*.ts' --include '**/*.js' --exclude 'src/nested/**'", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return listCwdFilesCmdFn(mockProjectPath, []string{"**/*.ts", "**/*.js"}, []string{"src/nested/**"}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "list-cwd-files-multiple-include.golden")
	})
}

func TestEntryPoints(t *testing.T) {
	t.Run("entry-points", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return entryPointsCmdFn(mockProjectPath, false, false, false, []string{}, []string{}, []string{}, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "entry-points.golden")
	})

	t.Run("entry-points --print-deps-count", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return entryPointsCmdFn(mockProjectPath, false, false, true, []string{}, []string{}, []string{}, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "entry-points-deps-count.golden")
	})

	t.Run("entry-points --ignore-type-imports --graph-exclude 'src/nested/**' --result-exclude '**/*.d.ts' --count", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return entryPointsCmdFn(mockProjectPath, true, false, true, []string{"src/nested/**"}, []string{"**/*.d.ts"}, []string{}, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "entry-points-exclude-patterns-count.golden")
	})

	t.Run("entry-points --result-include '**/*.ts' --print-deps-count --ignore-type-imports", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return entryPointsCmdFn(mockProjectPath, true, false, true, []string{}, []string{}, []string{"**/*.ts"}, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "entry-points-include-patterns-deps-count.golden")
	})

	t.Run("entry-points --graph-exclude 'packages/exported-package/src/deep/**' --result-exclude '**/*.d.ts' --result-include 'packages/**/*.ts' --follow-monorepo-packages", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockMonorepo")

		output, err := captureOutput(func() error {
			return entryPointsCmdFn(mockProjectPath, false, false, false, []string{"packages/exported-package/src/deep/**"}, []string{"**/*.d.ts"}, []string{"packages/**/*.ts"}, "", "", []string{}, true)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "entry-points-complex-filtering-monorepo.golden")
	})
}

func TestFiles(t *testing.T) {
	t.Run("files --entry-point index.ts", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return filesCmdFn(mockProjectPath, "index.ts", false, false, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "files.golden")
	})

	t.Run("files --entry-point index.ts --count", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return filesCmdFn(mockProjectPath, "index.ts", false, true, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "files-count.golden")
	})

	t.Run("files --entry-point index.ts --ignore-type-imports --package-json custom.package.json", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return filesCmdFn(mockProjectPath, "index.ts", true, false, "custom.package.json", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "files-ignore-type-custom-config.golden")
	})

	t.Run("files --entry-point packages/exported-package/src/main.ts --condition-names node,imports --follow-monorepo-packages --count", func(t *testing.T) {
		tempDir := t.TempDir()
		mockProjectPath := filepath.Join("__fixtures__", "mockMonorepo")
		tempProjectPath := filepath.Join(tempDir, "mockMonorepo")

		err := copyDir(mockProjectPath, tempProjectPath)
		assert.NilError(t, err)

		// Create isolated entry point in temp monorepo that imports a conditional subpath
		entryPointPath := filepath.Join(tempProjectPath, "packages", "exported-package", "src", "smoke-files-count.ts")
		err = os.WriteFile(entryPointPath, []byte("import 'exported-package/deep';\nexport const countMe = 1;"), 0644)
		assert.NilError(t, err)

		output, err := captureOutput(func() error {
			return filesCmdFn(tempProjectPath, "packages/exported-package/src/smoke-files-count.ts", false, true, "", "", []string{"node", "imports"}, true)
		})

		assert.NilError(t, err)

		// Sanitize output
		output = strings.ReplaceAll(output, tempProjectPath, "<TMP_FIXTURE_PATH>")
		golden.Assert(t, output, "files-monorepo-conditions-count.golden")
	})

	t.Run("files --entry-point packages/consumer-package/index.ts --follow-monorepo-packages --ignore-type-imports", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockMonorepo")

		output, err := captureOutput(func() error {
			return filesCmdFn(mockProjectPath, "packages/consumer-package/index.ts", true, false, "", "", []string{}, true)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "files-monorepo-ignore-type.golden")
	})
}

func TestLinesOfCodeCmd(t *testing.T) {
	mockProjectPath := filepath.Join("__fixtures__", "mockProject")

	output, err := captureOutput(func() error {
		return linesOfCodeCmdFn(mockProjectPath)
	})

	assert.NilError(t, err)
	golden.Assert(t, output, "lines-of-code.golden")
}

func TestImportedByCmd(t *testing.T) {
	t.Run("imported-by --file src/types.ts", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return importedByCmdFn(mockProjectPath, "src/types.ts", false, false, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "imported-by-types.golden")
	})

	t.Run("imported-by --file src/types.ts --count", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return importedByCmdFn(mockProjectPath, "src/types.ts", true, false, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "imported-by-types-count.golden")
	})

	t.Run("imported-by --file src/types.ts --list-imports", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return importedByCmdFn(mockProjectPath, "src/types.ts", false, true, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "imported-by-types-list-imports.golden")
	})

	t.Run("imported-by --file moduleSrc/fileA.tsx", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return importedByCmdFn(mockProjectPath, "moduleSrc/fileA.tsx", false, false, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "imported-by-fileA.golden")
	})

	t.Run("imported-by --file moduleSrc/fileA.tsx --list-imports", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return importedByCmdFn(mockProjectPath, "moduleSrc/fileA.tsx", false, true, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "imported-by-fileA-list-imports.golden")
	})
}

func TestResolveCmd(t *testing.T) {
	t.Run("resolve --file src/types.ts", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return resolveCmdFn(mockProjectPath, "src/types.ts", []string{}, []string{}, false, false, false, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "resolve.golden")
	})

	t.Run("resolve --file src/types.ts --entry-points index.ts", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return resolveCmdFn(mockProjectPath, "src/types.ts", []string{"index.ts"}, []string{}, false, false, false, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "resolve-with-entry-points.golden")
	})

	t.Run("resolve --file src/types.ts --entry-point src/exclude-test-entry.ts --graph-exclude 'src/nested/**'", func(t *testing.T) {
		tempDir := t.TempDir()
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")
		tempProjectPath := filepath.Join(tempDir, "mockProject")

		err := copyDir(mockProjectPath, tempProjectPath)
		assert.NilError(t, err)

		// Create isolated entry points and helpers in temp project
		err = os.MkdirAll(filepath.Join(tempProjectPath, "src", "nested"), 0755)
		assert.NilError(t, err)

		err = os.WriteFile(filepath.Join(tempProjectPath, "src", "nested", "exclude-helper.ts"), []byte("export const helper = 1;"), 0644)
		assert.NilError(t, err)

		err = os.WriteFile(filepath.Join(tempProjectPath, "src", "exclude-test-entry.ts"), []byte("import './nested/exclude-helper';\nimport './types';"), 0644)
		assert.NilError(t, err)

		output, err := captureOutput(func() error {
			return resolveCmdFn(tempProjectPath, "src/types.ts", []string{"src/exclude-test-entry.ts"}, []string{"src/nested/**"}, false, false, false, "", "", []string{}, false)
		})

		assert.NilError(t, err)

		// Sanitize output
		output = strings.ReplaceAll(output, tempProjectPath, "<TMP_FIXTURE_PATH>")

		golden.Assert(t, output, "resolve-multiple-entry-points-graph-exclude.golden")
	})

	t.Run("resolve --file src/types.ts --entry-points index.ts --all", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return resolveCmdFn(mockProjectPath, "src/types.ts", []string{"index.ts"}, []string{}, false, true, false, "", "", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "resolve-all-paths.golden")
	})

	t.Run("resolve --file src/types.ts --entry-points index.ts --compact-summary --condition-names node,imports", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return resolveCmdFn(mockProjectPath, "src/types.ts", []string{"index.ts"}, []string{}, false, false, true, "", "", []string{"node", "imports"}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "resolve-compact-summary-conditions.golden")
	})

	t.Run("resolve --file src/types.ts --entry-points index.ts --package-json custom.package.json --tsconfig-json custom.tsconfig.json", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return resolveCmdFn(mockProjectPath, "src/types.ts", []string{"index.ts"}, []string{}, false, false, false, "custom.package.json", "custom.tsconfig.json", []string{}, false)
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "resolve-custom-config.golden")
	})
}

func TestNodeModules(t *testing.T) {
	t.Run("node-modules used", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

		output, err := captureOutput(func() error {
			result, _ := NodeModulesCmd(
				nodeModulesPath,
				false,
				[]string{},
				false,
				false,
				false,
				false,
				false,
				[]string{},
				[]string{},
				[]string{},
				[]string{},
				[]string{},
				"",
				"",
				[]string{},
				false,
			)
			fmt.Print(result)
			return nil
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "node-modules-used.golden")
	})

	t.Run("node-modules used --entry-points index.ts,src/importFileA.ts --group-by-module --ignore-type-imports", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

		output, err := captureOutput(func() error {
			result, _ := NodeModulesCmd(
				nodeModulesPath,
				true,
				[]string{"index.ts", "src/importFileA.ts"},
				false,
				false,
				false,
				true,
				false,
				[]string{},
				[]string{},
				[]string{},
				[]string{},
				[]string{},
				"",
				"",
				[]string{},
				false,
			)
			fmt.Print(result)
			return nil
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "node-modules-used-grouping-entry-points.golden")
	})

	t.Run("node-modules used --files-with-binaries fileWithBinary.txt --files-with-node-modules fileWithModule.txt --condition-names node,imports", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

		output, err := captureOutput(func() error {
			result, _ := NodeModulesCmd(
				nodeModulesPath,
				false,
				[]string{},
				false,
				false,
				false,
				false,
				false,
				[]string{},
				[]string{"fileWithBinary.txt"},
				[]string{"fileWithModule.txt"},
				[]string{},
				[]string{},
				"",
				"",
				[]string{"node", "imports"},
				false,
			)
			fmt.Print(result)
			return nil
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "node-modules-used-file-filters-conditions.golden")
	})

	t.Run("node-modules unused", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

		output, err := captureOutput(func() error {
			result, _ := NodeModulesCmd(
				nodeModulesPath,
				false,
				[]string{},
				false,
				true,
				false,
				false,
				false,
				[]string{},
				[]string{},
				[]string{},
				[]string{},
				[]string{},
				"",
				"",
				[]string{},
				false,
			)
			fmt.Print(result)
			return nil
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "node-modules-unused.golden")
	})

	t.Run("node-modules unused --exclude-modules @types/*,lodash-* --count", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

		output, err := captureOutput(func() error {
			result, _ := NodeModulesCmd(
				nodeModulesPath,
				false,
				[]string{},
				true,
				true,
				false,
				false,
				false,
				[]string{},
				[]string{},
				[]string{},
				[]string{},
				[]string{"@types/*", "lodash-*"},
				"",
				"",
				[]string{},
				false,
			)
			fmt.Print(result)
			return nil
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "node-modules-unused-exclude-count.golden")
	})

	t.Run("node-modules missing", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

		output, err := captureOutput(func() error {
			result, _ := NodeModulesCmd(
				nodeModulesPath,
				false,
				[]string{},
				false,
				false,
				true,
				false,
				false,
				[]string{},
				[]string{},
				[]string{},
				[]string{},
				[]string{},
				"",
				"",
				[]string{},
				false,
			)
			fmt.Print(result)
			return nil
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "node-modules-missing.golden")
	})

	t.Run("node-modules missing --entry-points packages/consumer-package/index.ts --condition-names node,imports --follow-monorepo-packages", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "mockMonorepo")

		output, err := captureOutput(func() error {
			result, _ := NodeModulesCmd(
				nodeModulesPath,
				false,
				[]string{"packages/consumer-package/index.ts"},
				false,
				false,
				true,
				false,
				false,
				[]string{},
				[]string{},
				[]string{},
				[]string{},
				[]string{},
				"",
				"",
				[]string{"node", "imports"},
				true,
			)
			fmt.Print(result)
			return nil
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "node-modules-missing-monorepo-conditions.golden")
	})
}

func TestNodeModulesInstalled(t *testing.T) {
	t.Run("node-modules installed", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

		output, err := captureOutput(func() error {
			result := GetInstalledModulesCmd(
				nodeModulesPath,
				[]string{},
				[]string{},
			)
			fmt.Print(result)
			return nil
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "node-modules-installed.golden")
	})

	t.Run("node-modules installed --include-modules dep1,dep2 --exclude-modules dep1", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

		output, err := captureOutput(func() error {
			result := GetInstalledModulesCmd(
				nodeModulesPath,
				[]string{"dep1", "dep2"},
				[]string{"dep1"},
			)
			fmt.Print(result)
			return nil
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "node-modules-installed-include-exclude.golden")
	})

	t.Run("node-modules installed-duplicates", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

		output, err := captureOutput(func() error {
			result := GetDuplicatedModulesCmd(
				nodeModulesPath,
				false,
				false,
				false,
				false,
			)
			fmt.Print(result)
			return nil
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "node-modules-installed-duplicates.golden")
	})

	t.Run("node-modules installed-duplicates --optimize --size-stats --verbose --isolate", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

		// Create a temporary copy of the fixture to allow safe mutation
		tmpDir, err := os.MkdirTemp("", "rev-dep-smoke-test-*")
		assert.NilError(t, err)
		defer os.RemoveAll(tmpDir)

		tmpFixturePath := filepath.Join(tmpDir, "nodeModulesCmdSmoke")
		err = copyDir(nodeModulesPath, tmpFixturePath)
		assert.NilError(t, err)

		output, err := captureOutput(func() error {
			result := GetDuplicatedModulesCmd(
				tmpFixturePath,
				true,
				true,
				true,
				true,
			)
			fmt.Print(result)
			return nil
		})

		assert.NilError(t, err)

		// Sanitize output to remove temporary directory paths
		output = strings.ReplaceAll(output, tmpFixturePath, "<TMP_FIXTURE_PATH>")

		golden.Assert(t, output, "node-modules-installed-duplicates-optimized.golden")
	})
}

func TestNodeModulesAnalyze(t *testing.T) {
	t.Run("node-modules analyze-size", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

		output, err := captureOutput(func() error {
			modules, _ := GetInstalledModules(nodeModulesPath, []string{}, []string{})
			results, err := AnalyzeNodeModules(nodeModulesPath, modules)
			if err != nil {
				return err
			}

			PrintAnalysis(results)
			return nil
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "node-modules-analyze-size.golden")
	})

	t.Run("node-modules dirs-size", func(t *testing.T) {
		nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

		output, err := captureOutput(func() error {
			result := ModulesDiskSizeCmd(nodeModulesPath)
			fmt.Print(result)
			return nil
		})

		assert.NilError(t, err)
		golden.Assert(t, output, "node-modules-dirs-size.golden")
	})
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err == nil {
		return os.Chmod(dst, info.Mode())
	}

	return nil
}
