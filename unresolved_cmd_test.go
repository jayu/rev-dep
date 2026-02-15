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
	out, err := getUnresolvedOutput(testCwd, "package.json", "tsconfig.json", []string{}, true)
	if err != nil {
		t.Fatalf("getUnresolvedOutput failed: %v", err)
	}

	if out == "" {
		t.Errorf("Expected non-empty output from getUnresolvedOutput, got empty string")
	}
}
