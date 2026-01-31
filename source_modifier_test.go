package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyChangesToContent(t *testing.T) {
	content := "The quick brown fox jumps over the lazy dog"
	// Indices:
	// "The quick brown fox jumps over the lazy dog"
	//  0123456789012345678901234567890123456789012
	//  0         1         2         3         4

	tests := []struct {
		name     string
		changes  []Change
		expected string
	}{
		{
			name: "Basic replacement (quick -> slow)",
			changes: []Change{
				{Start: 4, End: 9, Text: "slow"},
			},
			expected: "The slow brown fox jumps over the lazy dog",
		},
		{
			name: "Multiple non-overlapping (The -> A, dog -> cat)",
			changes: []Change{
				{Start: 0, End: 3, Text: "A"},     // "The"
				{Start: 40, End: 43, Text: "cat"}, // "dog"
			},
			expected: "A quick brown fox jumps over the lazy cat",
		},
		{
			name: "Nested changes (remove smaller) - (quick brown -> fast, brown -> black)",
			changes: []Change{
				{Start: 4, End: 15, Text: "fast"},   // "quick brown" (Len 11)
				{Start: 10, End: 15, Text: "black"}, // "brown" (Len 5)
			},
			expected: "The fast fox jumps over the lazy dog",
		},
		{
			name: "Overlapping changes (keep bigger) - (quick brown -> swift, brown fox -> silver fox)",
			changes: []Change{
				{Start: 4, End: 15, Text: "swift"},       // "quick brown" (Len 11)
				{Start: 10, End: 19, Text: "silver fox"}, // "brown fox" (Len 9)
			},
			expected: "The swift fox jumps over the lazy dog",
		},
		{
			name: "Tie-break on length (keep first by Start)",
			changes: []Change{
				{Start: 4, End: 15, Text: "speedy"},     // "quick brown" (Len 11)
				{Start: 10, End: 21, Text: "agile fox"}, // "brown fox j" (Len 11)
			},
			expected: "The speedy fox jumps over the lazy dog",
		},
		{
			name:     "No changes",
			changes:  []Change{},
			expected: "The quick brown fox jumps over the lazy dog",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyChangesToContent(content, tt.changes)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestApplyFileChanges(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	initialContent := "Hello World"
	if err := os.WriteFile(filePath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	changes := map[string][]Change{
		filePath: {
			{Start: 6, End: 11, Text: "Universe"},
		},
	}

	if err := ApplyFileChanges(changes); err != nil {
		t.Fatalf("ApplyFileChanges failed: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	expected := "Hello Universe"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}
