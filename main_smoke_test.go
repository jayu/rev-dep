package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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
	mockProjectPath := filepath.Join("__fixtures__", "multipleCyclesFromSameNode")

	// Run the command directly since os.Exit can't be captured
	output, err := captureOutput(func() error {
		// Use minimal dependency tree to avoid os.Exit
		excludeFiles := []string{}
		minimalTree, files, _ := GetMinimalDepsTreeForCwd(mockProjectPath, false, excludeFiles, []string{}, "", "", []string{}, false)
		cycles := FindCircularDependencies(minimalTree, files)

		// Format the output manually and write to stdout (not stderr like original)
		result := FormatCircularDependencies(cycles, mockProjectPath, minimalTree)
		fmt.Print(result)
		return nil
	})

	assert.NilError(t, err)
	golden.Assert(t, output, "circular.golden")
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
}

func TestLinesOfCodeCmd(t *testing.T) {
	mockProjectPath := filepath.Join("__fixtures__", "mockProject")

	output, err := captureOutput(func() error {
		return linesOfCodeCmdFn(mockProjectPath)
	})

	assert.NilError(t, err)
	golden.Assert(t, output, "lines-of-code.golden")
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

	t.Run("resolve --file src/types.ts --entry-points index.ts,src/importFileA.ts --graph-exclude 'src/nested/**'", func(t *testing.T) {
		mockProjectPath := filepath.Join("__fixtures__", "mockProject")

		output, err := captureOutput(func() error {
			return resolveCmdFn(mockProjectPath, "src/types.ts", []string{"index.ts", "src/importFileA.ts"}, []string{"src/nested/**"}, false, false, false, "", "", []string{}, false)
		})

		assert.NilError(t, err)
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
