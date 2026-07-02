package checks

import (
	"slices"
	"strings"

	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/module"
	"rev-dep-go/internal/rules"
)

// RestrictedDirectImporterViolation describes a file that directly imports a restricted target
// (file or node module) when it is not permitted to. Exactly one of File / Module is set.
type RestrictedDirectImporterViolation struct {
	ViolationType string `json:"violationType"` // "file" or "module"
	ImporterFile  string `json:"importerFile"`
	File          string `json:"file,omitempty"`
	Module        string `json:"module,omitempty"`
	ImportRequest string `json:"importRequest,omitempty"`
}

// FindRestrictedDirectImporters reports files that DIRECTLY import a configured target file or node
// module in violation of a per-target importer policy. It is the non-transitive counterpart of
// FindRestrictedImporters: it inspects only direct import edges (dep records), so it never builds a
// dependency graph.
//
// Targets are files XOR modules. The policy is one of two mutually exclusive shapes:
//   - AllowImporters (whitelist): only importers matching AllowImporters may directly import a target;
//     any other direct importer is a violation.
//   - DenyImporters (blacklist): importers matching DenyImporters may not directly import a target.
//
// (Files/Modules and AllowImporters/DenyImporters exclusivity is enforced by config validation; this
// function is defensive and simply does nothing useful when a required side is empty.)
func FindRestrictedDirectImporters(
	ruleTree MinimalDependencyTree,
	opts *rules.RestrictedDirectImportersDetectionOptions,
	rulePath string,
) []RestrictedDirectImporterViolation {
	empty := []RestrictedDirectImporterViolation{}
	if opts == nil || !opts.Enabled {
		return empty
	}
	if len(opts.Files) == 0 && len(opts.Modules) == 0 {
		return empty
	}
	if len(opts.AllowImporters) == 0 && len(opts.DenyImporters) == 0 {
		return empty
	}

	ignoreMatchers := globutil.CreateGlobMatchers(opts.IgnoreMatches, rulePath)
	fileMatchers := globutil.CreateGlobMatchers(opts.Files, rulePath)
	moduleMatchers := compileModuleGlobMatchers(opts.Modules)

	// Policy: AllowImporters is a whitelist (only these may import); DenyImporters is a blacklist
	// (these may not import). Exactly one is configured.
	useAllow := len(opts.AllowImporters) > 0
	allowMatchers := globutil.CreateGlobMatchers(opts.AllowImporters, rulePath)
	denyMatchers := globutil.CreateGlobMatchers(opts.DenyImporters, rulePath)

	isViolatingImporter := func(importerFile string) bool {
		if useAllow {
			return !globutil.MatchesAnyGlobMatcher(importerFile, allowMatchers, false)
		}
		return globutil.MatchesAnyGlobMatcher(importerFile, denyMatchers, false)
	}

	sortedFilePaths := make([]string, 0, len(ruleTree))
	for filePath := range ruleTree {
		sortedFilePaths = append(sortedFilePaths, filePath)
	}
	slices.Sort(sortedFilePaths)

	violations := []RestrictedDirectImporterViolation{}
	seen := map[string]bool{}

	for _, importerFile := range sortedFilePaths {
		if matchesIgnoredPattern(importerFile, ignoreMatchers) {
			continue
		}

		for _, dep := range ruleTree[importerFile] {
			if opts.IgnoreTypeImports && dep.ImportKind == OnlyTypeImport {
				continue
			}

			// File target: a direct import edge resolving to a user/monorepo file matching Files.
			if len(fileMatchers) > 0 &&
				dep.ID != "" &&
				(dep.ResolvedType == UserModule || dep.ResolvedType == MonorepoModule) &&
				importerFile != dep.ID &&
				globutil.MatchesAnyGlobMatcher(dep.ID, fileMatchers, false) &&
				!matchesIgnoredPattern(dep.ID, ignoreMatchers) &&
				isViolatingImporter(importerFile) {

				key := "file|" + importerFile + "|" + dep.ID
				if !seen[key] {
					seen[key] = true
					violations = append(violations, RestrictedDirectImporterViolation{
						ViolationType: "file",
						ImporterFile:  importerFile,
						File:          dep.ID,
					})
				}
				continue
			}

			// Module target: a direct import of a node module matching Modules.
			if len(moduleMatchers) > 0 &&
				(dep.ResolvedType == NodeModule || dep.ResolvedType == NotResolvedModule) {
				moduleName := module.GetNodeModuleName(dep.Request)
				if moduleName == "" || !module.IsValidNodeModuleName(moduleName) {
					continue
				}
				if !matchesAnyModulePattern(moduleMatchers, moduleName, dep.Request) {
					continue
				}
				if matchesIgnoredPattern(moduleName, ignoreMatchers) ||
					matchesIgnoredPattern(dep.Request, ignoreMatchers) {
					continue
				}
				if !isViolatingImporter(importerFile) {
					continue
				}

				key := "module|" + importerFile + "|" + dep.Request
				if !seen[key] {
					seen[key] = true
					violations = append(violations, RestrictedDirectImporterViolation{
						ViolationType: "module",
						ImporterFile:  importerFile,
						Module:        moduleName,
						ImportRequest: dep.Request,
					})
				}
			}
		}
	}

	slices.SortFunc(violations, func(a, b RestrictedDirectImporterViolation) int {
		if a.ViolationType != b.ViolationType {
			return strings.Compare(a.ViolationType, b.ViolationType)
		}
		if a.ImporterFile != b.ImporterFile {
			return strings.Compare(a.ImporterFile, b.ImporterFile)
		}
		if a.File != b.File {
			return strings.Compare(a.File, b.File)
		}
		if a.Module != b.Module {
			return strings.Compare(a.Module, b.Module)
		}
		return strings.Compare(a.ImportRequest, b.ImportRequest)
	})

	return violations
}
