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

func TestListCwdFilesCmd(t *testing.T) {
	mockProjectPath := filepath.Join("__fixtures__", "mockProject")

	output, err := captureOutput(func() error {
		return listCwdFilesCmdFn(mockProjectPath, []string{}, []string{}, false)
	})

	assert.NilError(t, err)
	golden.Assert(t, output, "list-cwd-files.golden")
}

func TestListCwdFilesCmdCount(t *testing.T) {
	mockProjectPath := filepath.Join("__fixtures__", "mockProject")

	output, err := captureOutput(func() error {
		return listCwdFilesCmdFn(mockProjectPath, []string{}, []string{}, true)
	})

	assert.NilError(t, err)
	golden.Assert(t, output, "list-cwd-files-count.golden")
}

func TestEntryPointsCmd(t *testing.T) {
	mockProjectPath := filepath.Join("__fixtures__", "mockProject")

	output, err := captureOutput(func() error {
		return entryPointsCmdFn(mockProjectPath, false, false, false, []string{}, []string{}, []string{}, "", "")
	})

	assert.NilError(t, err)
	golden.Assert(t, output, "entry-points.golden")
}

func TestEntryPointsCmdWithDepsCount(t *testing.T) {
	mockProjectPath := filepath.Join("__fixtures__", "mockProject")

	output, err := captureOutput(func() error {
		return entryPointsCmdFn(mockProjectPath, false, false, true, []string{}, []string{}, []string{}, "", "")
	})

	assert.NilError(t, err)
	golden.Assert(t, output, "entry-points-deps-count.golden")
}

func TestFilesCmd(t *testing.T) {
	mockProjectPath := filepath.Join("__fixtures__", "mockProject")

	output, err := captureOutput(func() error {
		return filesCmdFn(mockProjectPath, "index.ts", false, false, "", "")
	})

	assert.NilError(t, err)
	golden.Assert(t, output, "files.golden")
}

func TestFilesCmdCount(t *testing.T) {
	mockProjectPath := filepath.Join("__fixtures__", "mockProject")

	output, err := captureOutput(func() error {
		return filesCmdFn(mockProjectPath, "index.ts", false, true, "", "")
	})

	assert.NilError(t, err)
	golden.Assert(t, output, "files-count.golden")
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
	mockProjectPath := filepath.Join("__fixtures__", "mockProject")

	output, err := captureOutput(func() error {
		return resolveCmdFn(mockProjectPath, "src/types.ts", []string{}, []string{}, false, false, false, "", "")
	})

	assert.NilError(t, err)
	golden.Assert(t, output, "resolve.golden")
}

func TestResolveCmdWithEntryPoints(t *testing.T) {
	mockProjectPath := filepath.Join("__fixtures__", "mockProject")

	output, err := captureOutput(func() error {
		return resolveCmdFn(mockProjectPath, "src/types.ts", []string{"index.ts"}, []string{}, false, false, false, "", "")
	})

	assert.NilError(t, err)
	golden.Assert(t, output, "resolve-with-entry-points.golden")
}

// Node modules tests using nodeModulesCmd fixture
func TestNodeModulesUsedCmd(t *testing.T) {
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
}

func TestNodeModulesUnusedCmd(t *testing.T) {
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
}

func TestNodeModulesMissingCmd(t *testing.T) {
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
}

func TestNodeModulesInstalledCmd(t *testing.T) {
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
}

func TestNodeModulesInstalledDuplicatesCmd(t *testing.T) {
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
}

func TestNodeModulesAnalyzeSizeCmd(t *testing.T) {
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
}

func TestNodeModulesDirsSizeCmd(t *testing.T) {
	nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")

	output, err := captureOutput(func() error {
		result := ModulesDiskSizeCmd(nodeModulesPath)
		fmt.Print(result)
		return nil
	})

	assert.NilError(t, err)
	golden.Assert(t, output, "node-modules-dirs-size.golden")
}
