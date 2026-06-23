package checks

import (
	"slices"
	"strings"

	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/graph"
	"rev-dep-go/internal/module"
	"rev-dep-go/internal/rules"
)

// RestrictedImporterViolation describes an entry point that reaches (transitively imports) a
// restricted file or node module when it is not permitted to. Exactly one of File / Module is set.
type RestrictedImporterViolation struct {
	EntryPoint string `json:"entryPoint"`
	File       string `json:"file,omitempty"`
	Module     string `json:"module,omitempty"`
}

// FindRestrictedImporters reports entry points that transitively reach a configured file or node
// module when they are not permitted to. It is the reverse of FindRestrictedImports: instead of
// "which targets may this entry point reach", it answers "which entry points may reach this target".
//
// The policy is a whitelist: allowedEntryPoints lists the entry points permitted to reach the
// targets, and any entry point NOT matching the allowlist that reaches one is a violation.
// ruleEntryPoints is the universe of entry points (the rule's prod + dev entry points); subtracting
// the allowlist yields the "suspect" entry points that must not reach a target.
//
// Implementation: the forward graph is built ONCE from the suspect entry points (a suspect's
// reachability does not depend on the allowlisted entry points), and graph.BuildEntryPointReachability
// reports which suspects reach each target in that single build. Module targets are handled by
// treating every file that imports a matching module as a file target, then attributing reachability
// of that file to the module(s) it imports.
func FindRestrictedImporters(
	ruleTree MinimalDependencyTree,
	opts *rules.RestrictedImportersDetectionOptions,
	rulePath string,
	ruleEntryPoints []string,
) []RestrictedImporterViolation {
	empty := []RestrictedImporterViolation{}
	if opts == nil || !opts.Enabled || len(opts.AllowedEntryPoints) == 0 {
		return empty
	}
	if len(opts.Files) == 0 && len(opts.Modules) == 0 {
		return empty
	}

	excludeMatchers := globutil.CreateGlobMatchers(opts.GraphExclude, rulePath)
	graphTree := filterTreeByGraphExclude(ruleTree, excludeMatchers)
	ignoreMatchers := globutil.CreateGlobMatchers(opts.IgnoreMatches, rulePath)

	// Files matching the configured patterns, present in the (excluded) tree.
	fileMatchers := globutil.CreateGlobMatchers(opts.Files, rulePath)
	targetFiles := []string{}
	targetFileSet := map[string]bool{}
	for filePath := range graphTree {
		if globutil.MatchesAnyGlobMatcher(filePath, fileMatchers, false) {
			targetFiles = append(targetFiles, filePath)
			targetFileSet[filePath] = true
		}
	}
	slices.Sort(targetFiles)

	// Module targets: a "frontier file" is any file importing a module matching a configured module
	// pattern; reaching such a file means reaching the module. moduleFrontier maps the frontier file
	// to the matched module names it imports.
	moduleMatchers := compileModuleGlobMatchers(opts.Modules)
	moduleFrontier := map[string]map[string]bool{}
	if len(moduleMatchers) > 0 {
		for filePath, deps := range graphTree {
			for _, dep := range deps {
				if dep.ResolvedType != NodeModule && dep.ResolvedType != NotResolvedModule {
					continue
				}
				if opts.IgnoreTypeImports && dep.ImportKind == OnlyTypeImport {
					continue
				}
				moduleName := module.GetNodeModuleName(dep.Request)
				if moduleName == "" || !module.IsValidNodeModuleName(moduleName) {
					continue
				}
				if !matchesAnyModulePattern(moduleMatchers, moduleName, dep.Request) {
					continue
				}
				if matchesIgnoredPattern(moduleName, ignoreMatchers) || matchesIgnoredPattern(dep.Request, ignoreMatchers) {
					continue
				}
				set := moduleFrontier[filePath]
				if set == nil {
					set = map[string]bool{}
					moduleFrontier[filePath] = set
				}
				set[moduleName] = true
			}
		}
	}

	if len(targetFiles) == 0 && len(moduleFrontier) == 0 {
		return empty
	}

	// Suspects = the rule's entry points NOT matching the allowlist; only these are graph roots.
	universe := matchFilesInTree(graphTree, ruleEntryPoints, rulePath)
	allowMatchers := globutil.CreateGlobMatchers(opts.AllowedEntryPoints, rulePath)
	suspects := make([]string, 0, len(universe))
	for filePath := range universe {
		if globutil.MatchesAnyGlobMatcher(filePath, allowMatchers, false) {
			continue
		}
		suspects = append(suspects, filePath)
	}
	if len(suspects) == 0 {
		return empty
	}
	slices.Sort(suspects)

	// Targets passed to the single build: target files + module frontier files (both file paths).
	targetSet := map[string]bool{}
	for _, tf := range targetFiles {
		targetSet[tf] = true
	}
	for ff := range moduleFrontier {
		targetSet[ff] = true
	}
	targets := make([]string, 0, len(targetSet))
	for t := range targetSet {
		targets = append(targets, t)
	}
	slices.Sort(targets)

	reach := graph.BuildEntryPointReachability(graphTree, suspects, targets, opts.IgnoreTypeImports)

	violations := []RestrictedImporterViolation{}
	seen := map[string]bool{}

	// File violations: a suspect that reaches a target file (but is not itself that file).
	for _, targetFile := range targetFiles {
		if matchesIgnoredPattern(targetFile, ignoreMatchers) {
			continue
		}
		reaches := reach.RootReachesTarget[targetFile]
		if len(reaches) == 0 {
			continue
		}
		for _, entryPoint := range suspects {
			if !reaches[entryPoint] {
				continue
			}
			if entryPoint == targetFile || targetFileSet[entryPoint] {
				continue
			}
			if matchesIgnoredPattern(entryPoint, ignoreMatchers) {
				continue
			}
			key := "file|" + entryPoint + "|" + targetFile
			if seen[key] {
				continue
			}
			seen[key] = true
			violations = append(violations, RestrictedImporterViolation{EntryPoint: entryPoint, File: targetFile})
		}
	}

	// Module violations: a suspect that reaches a file importing a target module. Unlike the file
	// case, a suspect that directly imports the module is a violation too (no self-skip).
	frontierFiles := make([]string, 0, len(moduleFrontier))
	for ff := range moduleFrontier {
		frontierFiles = append(frontierFiles, ff)
	}
	slices.Sort(frontierFiles)
	for _, frontierFile := range frontierFiles {
		reaches := reach.RootReachesTarget[frontierFile]
		if len(reaches) == 0 {
			continue
		}
		modules := make([]string, 0, len(moduleFrontier[frontierFile]))
		for m := range moduleFrontier[frontierFile] {
			modules = append(modules, m)
		}
		slices.Sort(modules)
		for _, entryPoint := range suspects {
			if !reaches[entryPoint] {
				continue
			}
			if matchesIgnoredPattern(entryPoint, ignoreMatchers) {
				continue
			}
			for _, moduleName := range modules {
				key := "module|" + entryPoint + "|" + moduleName
				if seen[key] {
					continue
				}
				seen[key] = true
				violations = append(violations, RestrictedImporterViolation{EntryPoint: entryPoint, Module: moduleName})
			}
		}
	}

	slices.SortFunc(violations, func(a, b RestrictedImporterViolation) int {
		if a.EntryPoint != b.EntryPoint {
			return strings.Compare(a.EntryPoint, b.EntryPoint)
		}
		if a.File != b.File {
			return strings.Compare(a.File, b.File)
		}
		return strings.Compare(a.Module, b.Module)
	})

	return violations
}

// matchFilesInTree returns the set of files in the tree matching any of the given glob patterns.
func matchFilesInTree(tree MinimalDependencyTree, patterns []string, rulePath string) map[string]bool {
	matchers := globutil.CreateGlobMatchers(patterns, rulePath)
	result := map[string]bool{}
	for filePath := range tree {
		if globutil.MatchesAnyGlobMatcher(filePath, matchers, false) {
			result[filePath] = true
		}
	}
	return result
}
