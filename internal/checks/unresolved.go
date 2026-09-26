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
	// IsNonLiteral marks `import(x)`: Request is then the expression as written in the code, not a
	// module path, so a consumer must not treat it as one.
	IsNonLiteral bool
	// RequestStart/RequestEnd are the byte range of the import in the file, which reports use to
	// order issues by where they appear and to quote the source.
	RequestStart uint32
	RequestEnd   uint32
}

type UnresolvedFilterOptions struct {
	Ignore        globutil.FileValueIgnoreMap
	IgnoreFiles   []string
	IgnoreImports []string
}

// DetectUnresolvedImports collects imports the resolver could not resolve. reportNonLiteral adds
// the `import(x)` records, whose specifier is an expression: they are off by default because a
// project that passes today would start failing on code that was never reported before.
func DetectUnresolvedImports(minimalTree MinimalDependencyTree, ignoredNodeModules map[string]bool, reportNonLiteral bool) []UnresolvedImport {
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
			if dep.ResolvedType == NonLiteralModule {
				if reportNonLiteral {
					unresolved = append(unresolved, UnresolvedImport{
						FilePath:     filePath,
						Request:      dep.Request,
						IsNonLiteral: true,
						RequestStart: dep.RequestStart,
						RequestEnd:   dep.RequestEnd,
					})
				}
				continue
			}
			if dep.ResolvedType == NotResolvedModule && dep.Request != "" && !ignoredNodeModules[module.GetNodeModuleName(dep.Request)] {
				unresolved = append(unresolved, UnresolvedImport{
					FilePath:     filePath,
					Request:      dep.Request,
					RequestStart: dep.RequestStart,
					RequestEnd:   dep.RequestEnd,
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
