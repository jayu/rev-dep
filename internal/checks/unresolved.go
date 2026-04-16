package checks

import (
	"slices"

	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/module"
)

// UnresolvedImport represents a single unresolved import detected in a file.
type UnresolvedImport struct {
	FilePath string
	Request  string
}

type UnresolvedFilterOptions struct {
	Ignore        globutil.FileValueIgnoreMap
	IgnoreFiles   []string
	IgnoreImports []string
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
			if dep.ResolvedType == NotResolvedModule && dep.Request != "" && !ignoredNodeModules[module.GetNodeModuleName(dep.Request)] {
				unresolved = append(unresolved, UnresolvedImport{
					FilePath: filePath,
					Request:  dep.Request,
				})
			}
		}
	}

	return unresolved
}

func FilterUnresolvedImports(unresolved []UnresolvedImport, opts *UnresolvedFilterOptions, cwd string) []UnresolvedImport {
	if opts == nil {
		return unresolved
	}

	ignoreMatcher := globutil.NewFileValueIgnoreMatcher(opts.Ignore, opts.IgnoreFiles, opts.IgnoreImports, cwd)

	filtered := make([]UnresolvedImport, 0, len(unresolved))
	for _, u := range unresolved {
		if ignoreMatcher.ShouldIgnore(u.FilePath, u.Request) {
			continue
		}

		filtered = append(filtered, u)
	}

	return filtered
}
