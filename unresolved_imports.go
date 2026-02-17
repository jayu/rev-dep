package main

import (
	"path/filepath"
	"slices"
	"strings"
)

// UnresolvedImport represents a single unresolved import detected in a file.
type UnresolvedImport struct {
	FilePath string
	Request  string
}

func normalizeUnresolvedFilePathForMatching(path string) string {
	cleaned := filepath.Clean(DenormalizePathForOS(strings.TrimSpace(path)))
	normalized := NormalizePathForInternal(cleaned)
	return strings.TrimPrefix(normalized, "./")
}

func getUnresolvedRelativeFilePath(path string, cwd string) string {
	filePath := DenormalizePathForOS(path)
	cwdPath := DenormalizePathForOS(cwd)

	if filepath.IsAbs(filePath) {
		if relPath, err := filepath.Rel(cwdPath, filePath); err == nil {
			return normalizeUnresolvedFilePathForMatching(relPath)
		}
	}

	return normalizeUnresolvedFilePathForMatching(filePath)
}

func DetectUnresolvedImports(minimalTree MinimalDependencyTree, ignoredNodeModules map[string]bool) []UnresolvedImport {
	if ignoredNodeModules == nil {
		ignoredNodeModules = map[string]bool{}
	}

	filePaths := make([]string, 0, len(minimalTree))
	for filePath := range minimalTree {
		filePaths = append(filePaths, filePath)
	}
	slices.Sort(filePaths)

	unresolved := []UnresolvedImport{}
	for _, filePath := range filePaths {
		for _, dep := range minimalTree[filePath] {
			if dep.ResolvedType == NotResolvedModule && dep.Request != "" && !ignoredNodeModules[GetNodeModuleName(dep.Request)] {
				unresolved = append(unresolved, UnresolvedImport{
					FilePath: filePath,
					Request:  dep.Request,
				})
			}
		}
	}

	return unresolved
}

func FilterUnresolvedImports(unresolved []UnresolvedImport, opts *UnresolvedImportsOptions, cwd string) []UnresolvedImport {
	if opts == nil {
		return unresolved
	}

	ignoreFilesMatchers := CreateGlobMatchers(opts.IgnoreFiles, cwd)

	ignoreImportsSet := make(map[string]bool, len(opts.IgnoreImports))
	for _, req := range opts.IgnoreImports {
		ignoreImportsSet[req] = true
	}

	ignoredRequestsByFile := make(map[string]map[string]bool, len(opts.Ignore))
	for filePath, req := range opts.Ignore {
		normalizedFilePath := normalizeUnresolvedFilePathForMatching(filePath)
		if _, ok := ignoredRequestsByFile[normalizedFilePath]; !ok {
			ignoredRequestsByFile[normalizedFilePath] = map[string]bool{}
		}
		ignoredRequestsByFile[normalizedFilePath][req] = true
	}

	filtered := make([]UnresolvedImport, 0, len(unresolved))
	for _, u := range unresolved {
		if ignoreImportsSet[u.Request] {
			continue
		}

		if MatchesAnyGlobMatcher(u.FilePath, ignoreFilesMatchers, false) {
			continue
		}

		relativePath := getUnresolvedRelativeFilePath(u.FilePath, cwd)
		if ignoredRequests, ok := ignoredRequestsByFile[relativePath]; ok && ignoredRequests[u.Request] {
			continue
		}

		filtered = append(filtered, u)
	}

	return filtered
}
