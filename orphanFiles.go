package main

import (
	"slices"
)

// FindOrphanFiles returns a list of orphan files from a pre-built dependency tree
func FindOrphanFiles(
	minimalTree MinimalDependencyTree,
	validEntryPoints []string,
	graphExclude []string,
	ignoreTypeImports bool,
	cwd string,
	moduleSuffixVariants map[string]bool,
) []string {
	// Create glob matchers for valid entry points and graph exclusions
	entryPointGlobs := CreateGlobMatchers(validEntryPoints, cwd)
	excludeGlobs := CreateGlobMatchers(graphExclude, cwd)

	// Build referenced files map from the dependency tree
	referencedFiles := map[string]bool{}

	for filePath, fileDeps := range minimalTree {
		// Skip excluded files from being considered as references
		if MatchesAnyGlobMatcher(filePath, excludeGlobs, false) {
			continue
		}

		for _, dependency := range fileDeps {
			if dependency.ID == "" {
				continue
			}

			// Skip type-only imports if ignoreTypeImports is enabled
			if ignoreTypeImports && dependency.ImportKind == OnlyTypeImport {
				continue
			}

			depPath := dependency.ID
			// Only mark as referenced if the dependency file exists and is not excluded
			if _, exists := minimalTree[depPath]; exists && !MatchesAnyGlobMatcher(depPath, excludeGlobs, false) {
				referencedFiles[depPath] = true
			}
		}
	}

	// Find orphan files (files that are not referenced and not valid entry points)
	orphanFiles := []string{}
	for filePath := range minimalTree {
		// Skip excluded files entirely
		if MatchesAnyGlobMatcher(filePath, excludeGlobs, false) {
			continue
		}

		isReferenced := referencedFiles[filePath]
		isEntryPoint := len(entryPointGlobs) > 0 && MatchesAnyGlobMatcher(filePath, entryPointGlobs, false)
		isVariant := moduleSuffixVariants != nil && moduleSuffixVariants[filePath]

		// A file is orphan if it's not referenced by other files AND it's not a valid entry point
		// AND it's not a module-suffix variant (platform-specific sibling)
		if !isReferenced && !isEntryPoint && !isVariant {
			orphanFiles = append(orphanFiles, filePath)
		}
	}

	slices.Sort(orphanFiles)
	return orphanFiles
}
