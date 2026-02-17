package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestFormatAndPrintConfigResults_Summary(t *testing.T) {
	tests := []struct {
		name     string
		result   *ConfigProcessingResult
		expected string
	}{
		{
			name: "Only imports fixed",
			result: &ConfigProcessingResult{
				FixedImportsCount: 5,
				FixedFilesCount:   2,
			},
			expected: "✍️ Fixed 5 imports in 2 files",
		},
		{
			name: "Only files removed",
			result: &ConfigProcessingResult{
				DeletedFilesCount: 3,
			},
			expected: "✍️ Removed 3 orphan files",
		},
		{
			name: "Both fixed and removed",
			result: &ConfigProcessingResult{
				FixedImportsCount: 5,
				FixedFilesCount:   2,
				DeletedFilesCount: 3,
			},
			expected: "✍️ Fixed 5 imports in 2 files, removed 3 orphan files",
		},
		{
			name: "Nothing fixed",
			result: &ConfigProcessingResult{
				FixedImportsCount: 0,
				FixedFilesCount:   0,
				DeletedFilesCount: 0,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save current stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// formatAndPrintConfigResults uses fmt.Printf which writes to stdout
			formatAndPrintConfigResults(tt.result, ".", true)

			w.Close()
			os.Stdout = old

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			if tt.expected == "" {
				if strings.Contains(output, "✍️") {
					t.Errorf("Expected no summary, but got: %q", output)
				}
			} else {
				if !strings.Contains(output, tt.expected) {
					t.Errorf("Expected summary %q, but got: %q", tt.expected, output)
				}
			}
		})
	}
}
