package main

import (
	"os"
	"sort"
	"strings"
)

// Change represents a text replacement in a file.
// Start and End are byte offsets in the original file content.
type Change struct {
	Start int32
	End   int32
	Text  string
}

// ApplyFileChanges accepts a list of changes grouped by file paths and applies them.
// Changes are applied to each file by reading it, filtering overlaps/nesting,
// and saving it back to the file system.
func ApplyFileChanges(changesByFile map[string][]Change) error {
	for filePath, changes := range changesByFile {
		if err := applyChangesToFile(filePath, changes); err != nil {
			return err
		}
	}
	return nil
}

func applyChangesToFile(filePath string, changes []Change) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	newContent := applyChangesToContent(string(content), changes)

	// Only write if there are actual changes or we want to ensure it's saved?
	// The requirement says "file at the end should be saved into file system".
	return os.WriteFile(filePath, []byte(newContent), 0644)
}

func applyChangesToContent(content string, changes []Change) string {
	if len(changes) == 0 {
		return content
	}

	// 1. To handle "only the bigger one is applied", we sort by Length DESC.
	// If lengths are equal, we sort by Start ASC to be deterministic.
	sort.Slice(changes, func(i, j int) bool {
		lenI := changes[i].End - changes[i].Start
		lenJ := changes[j].End - changes[j].Start
		if lenI != lenJ {
			return lenI > lenJ
		}
		return changes[i].Start < changes[j].Start
	})

	// 2. Pick non-overlapping changes. Since we sorted by Length DESC,
	// we always pick the largest available change for any given span.
	var picked []Change
	for _, c := range changes {
		overlaps := false
		for _, p := range picked {
			// Overlap check: (c.Start < p.End) && (p.Start < c.End)
			if c.Start < p.End && p.Start < c.End {
				overlaps = true
				break
			}
		}
		if !overlaps {
			picked = append(picked, c)
		}
	}

	// 3. Changes must be applied one by one, sorted by their start location.
	sort.Slice(picked, func(i, j int) bool {
		return picked[i].Start < picked[j].Start
	})

	// 4. String Replacement engine: split file into sections and join at the end.
	// We use strings.Builder and avoid unnecessary string concatenation.
	var builder strings.Builder
	lastPos := int32(0)
	for _, c := range picked {
		// Defensive checks for valid ranges
		if c.Start < 0 || c.End < c.Start || int(c.Start) > len(content) {
			continue
		}

		if c.Start > lastPos {
			builder.WriteString(content[lastPos:c.Start])
		}
		builder.WriteString(c.Text)
		lastPos = c.End
	}

	if int(lastPos) < len(content) {
		builder.WriteString(content[lastPos:])
	}

	return builder.String()
}
