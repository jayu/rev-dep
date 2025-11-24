package main

import "slices"

func GetEntryPoints(minimalTree MinimalDependencyTree, resultExclude []string, resultInclude []string, cwd string) []string {
	referencedFiles := map[string]byte{}

	for _, imports := range minimalTree {
		for _, dependency := range imports {
			referencedFiles[*dependency.ID] = 0
		}
	}

	notReferencedFiles := []string{}

	excludeGlobs := CreateGlobMatchers(resultExclude, cwd)
	includeGlobs := CreateGlobMatchers(resultInclude, cwd)

	for filePath := range minimalTree {
		_, wasReferenced := referencedFiles[filePath]
		if !wasReferenced {
			if len(includeGlobs) == 0 || MatchesAnyGlobMatcher(filePath, includeGlobs, false) {
				isExcluded := MatchesAnyGlobMatcher(filePath, excludeGlobs, false)

				if !isExcluded {
					notReferencedFiles = append(notReferencedFiles, filePath)
				}
			}
		}
	}

	slices.Sort(notReferencedFiles)

	return notReferencedFiles
}
