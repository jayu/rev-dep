package main

import (
	"os"
	"sort"
)

// UnusedExport represents a single unused export found during analysis
type UnusedExport struct {
	FilePath   string
	ExportName string
	IsType     bool
	Fix        *Change // Pre-computed fix, nil if not auto-fixable
}

// exportEntry tracks a single exported name with its source dependency
type exportEntry struct {
	Name   string
	IsType bool
	Dep    *MinimalDependency
}

// FindUnusedExports finds exports that are not imported by any other file
func FindUnusedExports(
	ruleFiles []string,
	ruleTree MinimalDependencyTree,
	validEntryPoints []string,
	graphExclude []string,
	ignoreTypeExports bool,
	autofix bool,
	cwd string,
	moduleSuffixVariants map[string]bool,
) []UnusedExport {
	entryPointGlobs := CreateGlobMatchers(validEntryPoints, cwd)
	graphExcludeGlobs := CreateGlobMatchers(graphExclude, cwd)

	// Step 1: Build export map — file -> exportName -> exportEntry
	exportMap := make(map[string]map[string]exportEntry)

	for _, file := range ruleFiles {
		if MatchesAnyGlobMatcher(file, graphExcludeGlobs, false) {
			continue
		}
		if moduleSuffixVariants != nil && moduleSuffixVariants[file] {
			continue
		}

		deps := ruleTree[file]
		for i := range deps {
			dep := &deps[i]

			// Local export (export const X, export function Y, export { A as B }, etc.)
			if dep.IsLocalExport && dep.Keywords != nil {
				if exportMap[file] == nil {
					exportMap[file] = make(map[string]exportEntry)
				}
				for _, kw := range dep.Keywords.Keywords {
					name := kw.Name
					if kw.Alias != "" {
						name = kw.Alias
					}
					exportMap[file][name] = exportEntry{
						Name:   name,
						IsType: kw.IsType,
						Dep:    dep,
					}
				}
				continue
			}

			// Re-export (export { A } from './file' or export * from './file')
			if dep.ExportKeyEnd > 0 && !dep.IsLocalExport {
				if dep.Keywords != nil {
					for _, kw := range dep.Keywords.Keywords {
						// Plain star re-exports (export * from './file') are passthrough —
						// they don't define named exports in the current file.
						// export * as name from './file' DOES define a named export (the alias).
						if kw.Name == "*" && kw.Alias == "" {
							continue
						}
						if exportMap[file] == nil {
							exportMap[file] = make(map[string]exportEntry)
						}
						name := kw.Name
						if kw.Alias != "" {
							name = kw.Alias
						}
						exportMap[file][name] = exportEntry{
							Name:   name,
							IsType: kw.IsType,
							Dep:    dep,
						}
					}
				}
			}
		}
	}

	// Step 2: Build usage map — file -> set of used export names
	usedExports := make(map[string]map[string]bool)

	markAllUsed := func(targetFile string) {
		if usedExports[targetFile] == nil {
			usedExports[targetFile] = make(map[string]bool)
		}
		usedExports[targetFile]["*"] = true // sentinel for "all used"
	}

	markUsed := func(targetFile, name string) {
		if usedExports[targetFile] == nil {
			usedExports[targetFile] = make(map[string]bool)
		}
		usedExports[targetFile][name] = true
	}

	for file, deps := range ruleTree {
		if MatchesAnyGlobMatcher(file, graphExcludeGlobs, false) {
			continue
		}

		for i := range deps {
			dep := &deps[i]

			targetFile := ""
			if dep.ID != "" {
				targetFile = dep.ID
			}

			if targetFile == "" {
				continue
			}

			// Re-export: marks source file's exports as used
			if dep.ExportKeyEnd > 0 && !dep.IsLocalExport {
				if dep.Keywords != nil {
					for _, kw := range dep.Keywords.Keywords {
						if kw.Name == "*" {
							// export * from './file' — marks all of source's exports as used
							markAllUsed(targetFile)
						} else {
							// export { A, B } from './file' — marks A, B as used in source
							markUsed(targetFile, kw.Name)
						}
					}
				} else {
					// No keywords (basic parse mode) — treat as star re-export
					markAllUsed(targetFile)
				}
				continue
			}

			// Regular import
			if dep.Keywords == nil {
				if dep.IsDynamicImport {
					// Dynamic import: import('./file') or require('./file') — marks all used
					markAllUsed(targetFile)
				}
				// Side-effect import: import './file' — doesn't mark any exports used
				continue
			}

			for _, kw := range dep.Keywords.Keywords {
				if kw.Name == "*" {
					// Namespace import: import * as Ns from './file'
					markAllUsed(targetFile)
				} else {
					markUsed(targetFile, kw.Name)
				}
			}
		}
	}

	// Step 3: Mark entry point files — all exports considered used
	for file := range exportMap {
		if MatchesAnyGlobMatcher(file, entryPointGlobs, false) {
			markAllUsed(file)
		}
	}

	// Step 4: Compute unused exports and generate fixes
	var results []UnusedExport

	for file, exports := range exportMap {
		used := usedExports[file]
		allUsed := used != nil && used["*"]

		if allUsed {
			continue
		}

		// Group exports by their source dependency for fix generation
		type depUnused struct {
			dep           *MinimalDependency
			unusedExports []exportEntry
		}

		depMap := make(map[*MinimalDependency]*depUnused)

		for name, entry := range exports {
			if used != nil && used[name] {
				continue
			}
			if ignoreTypeExports && entry.IsType {
				continue
			}

			if depMap[entry.Dep] == nil {
				depMap[entry.Dep] = &depUnused{
					dep: entry.Dep,
				}
			}
			depMap[entry.Dep].unusedExports = append(depMap[entry.Dep].unusedExports, entry)
		}

		for _, du := range depMap {
			dep := du.dep

			for _, entry := range du.unusedExports {
				ue := UnusedExport{
					FilePath:   file,
					ExportName: entry.Name,
					IsType:     entry.IsType,
				}

				if dep.Keywords == nil {
					results = append(results, ue)
					continue
				}

				totalKeywords := len(dep.Keywords.Keywords)
				unusedCount := len(du.unusedExports)

				// Compute fix if autofix is enabled
				if autofix {
					if dep.IsLocalExport && dep.ExportBraceStart == 0 {
						// Strategy 1: Single-declaration — remove "export [default] " prefix
						if entry.Name == "default" {
							// For default exports, only safe when followed by a named declaration
							ue.Fix = computeDefaultExportFix(file, dep)
						} else {
							ue.Fix = &Change{
								Start: int32(dep.ExportKeyStart),
								End:   int32(dep.ExportDeclStart),
								Text:  "",
							}
						}
					} else if dep.ExportBraceStart > 0 && unusedCount == totalKeywords {
						// Strategy 3: All keywords unused — remove entire statement
						ue.Fix = computeFullStatementRemoval(file, dep)
					} else if dep.ExportBraceStart > 0 && unusedCount < totalKeywords {
						// Strategy 2: Surgical removal — only assign fix to first unused export
						// to avoid duplicate fixes for same statement
						ue.Fix = computeSurgicalBraceFix(file, dep, du.unusedExports)
					} else if !dep.IsLocalExport && dep.ExportBraceStart == 0 && dep.ExportStatementEnd > 0 {
						// Strategy 4: Named star re-export (export * as X from 'y') — remove entire statement
						ue.Fix = computeFullStatementRemoval(file, dep)
					}
					// Star re-exports (no keywords) or other cases — Fix remains nil
				}

				results = append(results, ue)
			}
		}
	}

	return results
}

// computeFullStatementRemoval computes fix for removing an entire export statement (Strategy 3)
func computeFullStatementRemoval(filePath string, dep *MinimalDependency) *Change {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return &Change{
			Start: int32(dep.ExportKeyStart),
			End:   int32(dep.ExportStatementEnd),
			Text:  "",
		}
	}

	start := int(dep.ExportKeyStart)
	end := int(dep.ExportStatementEnd)

	// Extend to cover entire line if it would be empty after removal
	start, end = expandToFillLineIfEmpty(source, start, end)

	return &Change{
		Start: int32(start),
		End:   int32(end),
		Text:  "",
	}
}

// computeDefaultExportFix checks whether removing "export default " is safe.
// It's only safe when followed by a named declaration (identifier, function, class, async function).
// For expressions like objects, arrays, or arrow functions, removing the prefix would break syntax.
func computeDefaultExportFix(filePath string, dep *MinimalDependency) *Change {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	declStart := skipSpacesAndComments(source, int(dep.ExportDeclStart))
	if declStart >= len(source) {
		return nil
	}

	// Check what follows "export default "
	ch := source[declStart]
	if !isByteIdentifierChar(ch) {
		// Starts with {, [, (, digit, quote, etc. — unsafe
		return nil
	}

	// It starts with an identifier char — safe (function, class, async, or a plain identifier)
	return &Change{
		Start: int32(dep.ExportKeyStart),
		End:   int32(dep.ExportDeclStart),
		Text:  "",
	}
}

// computeSurgicalBraceFix computes a fix that removes only unused keywords from a brace-list export
func computeSurgicalBraceFix(filePath string, dep *MinimalDependency, unusedExports []exportEntry) *Change {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	keywords := dep.Keywords.Keywords
	n := len(keywords)

	// Compute removal ranges for each unused keyword
	type removeRange struct {
		start, end int
	}
	var ranges []removeRange

	unusedNames := make(map[string]bool, len(unusedExports))

	for _, entry := range unusedExports {
		unusedNames[entry.Name] = true
	}

	for i, kw := range keywords {
		name := kw.Name
		if kw.Alias != "" {
			name = kw.Alias
		}
		if !unusedNames[name] {
			continue
		}

		var r removeRange

		if kw.CommaAfter > 0 && i < n-1 {
			// Has trailing comma AND a next keyword exists
			// Remove from this keyword start to next keyword start
			r = removeRange{int(kw.Start), int(keywords[i+1].Start)}
		} else if kw.CommaAfter > 0 && i == n-1 {
			// Has trailing comma AND is the last keyword (trailing comma style)
			r = removeRange{int(kw.Start), int(kw.CommaAfter) + 1}
		} else if kw.CommaAfter == 0 && i > 0 {
			// No trailing comma (last keyword) AND a previous keyword exists
			// Remove from previous keyword end to this keyword end
			r = removeRange{int(keywords[i-1].End), int(kw.End)}
		} else {
			// Only keyword — should not happen (would be Strategy 3)
			r = removeRange{int(kw.Start), int(kw.End)}
		}

		ranges = append(ranges, r)
	}

	if len(ranges) == 0 {
		return nil
	}

	// Check for empty-line cleanup: if removing a range would leave a line empty,
	// extend the range to cover the entire line
	for i := range ranges {
		r := &ranges[i]
		r.start, r.end = expandToFillLineIfEmpty(source, r.start, r.end)
	}

	// Sort ranges by start position
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].start < ranges[j].start
	})

	// Apply removals to produce new content within [BraceStart, BraceEnd]
	braceStart := int(dep.ExportBraceStart)
	braceEnd := int(dep.ExportBraceEnd)

	// Build the replacement text by copying source bytes and skipping removal ranges
	var result []byte
	pos := braceStart
	for _, r := range ranges {
		// Clamp ranges to brace boundaries
		rStart := r.start
		rEnd := r.end
		if rStart < braceStart {
			rStart = braceStart
		}
		if rEnd > braceEnd {
			rEnd = braceEnd
		}
		if rStart > pos {
			result = append(result, source[pos:rStart]...)
		}
		pos = rEnd
	}
	if pos < braceEnd {
		result = append(result, source[pos:braceEnd]...)
	}

	return &Change{
		Start: int32(braceStart),
		End:   int32(braceEnd),
		Text:  string(result),
	}
}

// expandToFillLineIfEmpty expands the removal range to cover the entire line (including newline)
// if the removal would otherwise leave an empty line (only whitespace).
func expandToFillLineIfEmpty(source []byte, start, end int) (int, int) {
	// Find line boundaries for the removal range
	lineStart := start
	for lineStart > 0 && source[lineStart-1] != '\n' {
		lineStart--
	}
	lineEnd := end
	for lineEnd < len(source) && source[lineEnd] != '\n' {
		lineEnd++
	}

	// Check if the line would be empty (only whitespace outside the removal range)
	lineEmpty := true
	for k := lineStart; k < start; k++ {
		if source[k] != ' ' && source[k] != '\t' {
			lineEmpty = false
			break
		}
	}
	if lineEmpty {
		for k := end; k < lineEnd; k++ {
			if source[k] != ' ' && source[k] != '\t' {
				lineEmpty = false
				break
			}
		}
	}

	if lineEmpty {
		// Expand to cover the entire line
		start = lineStart
		end = lineEnd
		// Also consume the trailing newline
		if end < len(source) && source[end] == '\n' {
			end++
		}
	}

	return start, end
}
