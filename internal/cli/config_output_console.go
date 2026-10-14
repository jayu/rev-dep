package cli

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/config"
	"rev-dep-go/internal/dupcode"
	"rev-dep-go/internal/emoji"
	"rev-dep-go/internal/node"
	"rev-dep-go/internal/plural"
)

// Each detector returns its part of the report. Current and obsolete findings
// compose these same functions; the caller supplies the surrounding summary.
func formatConfigRuleChecks(rule config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	for _, detector := range rule.EnabledChecks {
		switch detector {
		case "circular-imports":
			out.WriteString(formatCircularDependenciesConsole(rule, cwd, listAll))
		case "duplicated-code":
			out.WriteString(formatDuplicatedCodeConsole(rule, cwd, listAll))
		case "orphan-files":
			out.WriteString(formatOrphanFilesConsole(rule, cwd, listAll))
		case "module-boundaries":
			out.WriteString(formatModuleBoundariesConsole(rule, cwd, listAll))
		case "unused-node-modules":
			out.WriteString(formatUnusedNodeModulesConsole(rule, cwd, listAll))
		case "missing-node-modules":
			out.WriteString(formatMissingNodeModulesConsole(rule, cwd, listAll))
		case "import-conventions":
			out.WriteString(formatImportConventionsConsole(rule, cwd, listAll))
		case "unresolved-imports":
			out.WriteString(formatUnresolvedImportsConsole(rule, cwd, listAll))
		case "unused-exports":
			out.WriteString(formatUnusedExportsConsole(rule, cwd, listAll))
		case "dev-deps-usage-on-prod":
			out.WriteString(formatDevDepsUsageOnProdConsole(rule, cwd, listAll))
		case "restricted-imports":
			out.WriteString(formatRestrictedImportsConsole(rule, cwd, listAll))
		case "restricted-importers":
			out.WriteString(formatRestrictedImportersConsole(rule, cwd, listAll))
		case "restricted-direct-importers":
			out.WriteString(formatRestrictedDirectImportersConsole(rule, cwd, listAll))
		}
	}
	return out.String()
}

func formatConfigWorkspaceHeading(path string, fileCount *int) string {
	if path == "" {
		return ""
	}
	count := ""
	if fileCount != nil {
		count = fmt.Sprintf(" (%d files)", *fileCount)
	}
	return fmt.Sprintf("\n%s Workspace: %s%s\n", emoji.Rule, workspaceLabel(path), count)
}

func configOutputRelativePath(cwd, path string) string {
	if path == "" {
		return ""
	}
	relative, err := filepath.Rel(cwd, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relative)
}

func limitConfigIssues[T any](issues []T, listAll bool) ([]T, int) {
	if !listAll && len(issues) > maxIssuesToList {
		return issues[:maxIssuesToList], len(issues) - maxIssuesToList
	}
	return issues, 0
}

func formatCircularDependenciesConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	findings := ruleResult.CircularDependencies
	componentsOnly := len(findings) == 0
	if componentsOnly {
		findings = ruleResult.CircularComponents
	}
	if len(findings) > 0 {
		fmt.Fprintf(&out, "  %s Circular Dependencies Issues (%d):\n\n", emoji.Error, len(findings))

		toDisplay, remaining := limitConfigIssues(findings, listAll)

		if componentsOnly {
			for _, files := range toDisplay {
				relative := make([]string, len(files))
				for i, file := range files {
					relative[i] = configOutputRelativePath(cwd, file)
				}
				fmt.Fprintf(&out, "    - %s\n", circularComponentLabel(relative))
			}
		} else {
			out.WriteString(checks.FormatCircularDependenciesWithoutHeader(toDisplay, cwd, ruleResult.DependencyTree, 2))
		}

		if remaining > 0 {
			fmt.Fprintf(&out, "    ... and %d more circular dependency issues\n", remaining)
		}
	} else {
		fmt.Fprintf(&out, "  %s Circular Dependencies\n", emoji.Success)
	}
	return out.String()
}

func formatDuplicatedCodeConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	for _, detection := range ruleResult.DuplicatedCode {
		out.WriteString(formatDuplicatedCodeResult(detection, cwd))
	}
	return out.String()
}

func formatOrphanFilesConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	getRelativePath := func(path string) string { return configOutputRelativePath(cwd, path) }
	if len(ruleResult.OrphanFiles) > 0 {
		fmt.Fprintf(&out, "  %s Orphan Files Issues (%d):\n", emoji.Error, len(ruleResult.OrphanFiles))

		orphanFiles := slices.Clone(ruleResult.OrphanFiles)
		slices.Sort(orphanFiles)
		orphanFilesToDisplay, remaining := limitConfigIssues(orphanFiles, listAll)

		for _, file := range orphanFilesToDisplay {
			fmt.Fprintf(&out, "    - %s\n", getRelativePath(file))
		}

		if remaining > 0 {
			fmt.Fprintf(&out, "    ... and %d more orphan file issues\n", remaining)
		}
	} else {
		fmt.Fprintf(&out, "  %s Orphan Files\n", emoji.Success)
	}
	return out.String()
}

func formatModuleBoundariesConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	getRelativePath := func(path string) string { return configOutputRelativePath(cwd, path) }
	if len(ruleResult.ModuleBoundaryViolations) > 0 {
		fmt.Fprintf(&out, "  %s Module Boundary Issues (%d):\n", emoji.Error, len(ruleResult.ModuleBoundaryViolations))

		violations := slices.Clone(ruleResult.ModuleBoundaryViolations)
		slices.SortStableFunc(violations, func(a, b checks.ModuleBoundaryViolation) int {
			if a.FilePath != b.FilePath {
				return strings.Compare(a.FilePath, b.FilePath)
			}
			return strings.Compare(a.ImportPath, b.ImportPath)
		})
		violationsToDisplay, remaining := limitConfigIssues(violations, listAll)

		for _, violation := range violationsToDisplay {
			ruleName, violationType := "", ""
			if violation.RuleName != "" {
				ruleName = "[" + violation.RuleName + "] "
			}
			if violation.ViolationType != "" {
				violationType = " (NOT ALLOWED)"
				if violation.ViolationType == "denied" {
					violationType = " (DENIED)"
				}
			}
			fmt.Fprintf(&out, "    - %s%s -> %s%s\n",
				ruleName,
				getRelativePath(violation.FilePath),
				getRelativePath(violation.ImportPath),
				violationType)
		}

		if remaining > 0 {
			fmt.Fprintf(&out, "    ... and %d more module boundary issues\n", remaining)
		}
	} else {
		fmt.Fprintf(&out, "  %s Module Boundaries\n", emoji.Success)
	}
	return out.String()
}

func formatUnusedNodeModulesConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	getRelativePath := func(path string) string { return configOutputRelativePath(cwd, path) }
	if len(ruleResult.UnusedNodeModules) > 0 {
		fmt.Fprintf(&out, "  %s Unused Node Modules Issues (%d):\n", emoji.Error, len(ruleResult.UnusedNodeModules))

		modules := slices.Clone(ruleResult.UnusedNodeModules)
		slices.SortStableFunc(modules, func(a, b node.UnusedNodeModuleIssue) int {
			if a.ModuleName != b.ModuleName {
				return strings.Compare(a.ModuleName, b.ModuleName)
			}
			return strings.Compare(a.PackageJsonPath, b.PackageJsonPath)
		})
		modulesToDisplay, remaining := limitConfigIssues(modules, listAll)

		for _, module := range modulesToDisplay {
			packageJsonPath := getRelativePath(module.PackageJsonPath)
			if packageJsonPath != "" {
				fmt.Fprintf(&out, "    - %s (%s)\n", module.ModuleName, packageJsonPath)
			} else {
				fmt.Fprintf(&out, "    - %s\n", module.ModuleName)
			}
		}

		if remaining > 0 {
			fmt.Fprintf(&out, "    ... and %d more unused node module issues\n", remaining)
		}
	} else {
		fmt.Fprintf(&out, "  %s Unused Node Modules\n", emoji.Success)
	}
	return out.String()
}

func formatMissingNodeModulesConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	getRelativePath := func(path string) string { return configOutputRelativePath(cwd, path) }
	if len(ruleResult.MissingNodeModules) > 0 {
		fmt.Fprintf(&out, "  %s Missing Node Modules Issues (%d):\n", emoji.Error, len(ruleResult.MissingNodeModules))

		missingToDisplay, remaining := limitConfigIssues(ruleResult.MissingNodeModules, listAll)

		if ruleResult.MissingNodeModulesOutputType != "" {
			groupByModule := ruleResult.MissingNodeModulesOutputType == "groupByModule"
			groupByFile := ruleResult.MissingNodeModulesOutputType == "groupByFile"
			groupByModuleFilesCount := ruleResult.MissingNodeModulesOutputType == "groupByModuleFilesCount"

			formatted, _ := node.FormatMissingNodeModulesResults(missingToDisplay, cwd, false, groupByModule, groupByFile, groupByModuleFilesCount)
			lines := strings.Split(strings.TrimSpace(formatted), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				if groupByModuleFilesCount || ruleResult.MissingNodeModulesOutputType == "list" {
					fmt.Fprintf(&out, "    - %s\n", line)
				} else {
					fmt.Fprintf(&out, "    %s\n", line)
				}
			}
		} else {
			for _, missing := range missingToDisplay {
				// Convert imported from paths to relative paths
				relativeImportedFrom := make([]string, len(missing.ImportedFrom))
				for j, path := range missing.ImportedFrom {
					relativeImportedFrom[j] = getRelativePath(path)
				}

				if len(relativeImportedFrom) == 1 {
					fmt.Fprintf(&out, "    - %s (imported from: %s)\n", missing.ModuleName, relativeImportedFrom[0])
				} else if len(relativeImportedFrom) > 1 {
					fmt.Fprintf(&out, "    - %s (imported from: %s and %d more files)\n", missing.ModuleName, relativeImportedFrom[0], len(relativeImportedFrom)-1)
				}
			}
		}

		if remaining > 0 {
			fmt.Fprintf(&out, "    ... and %d more missing node module issues\n", remaining)
		}
	} else {
		fmt.Fprintf(&out, "  %s Missing Node Modules\n", emoji.Success)
	}
	return out.String()
}

func formatImportConventionsConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	getRelativePath := func(path string) string { return configOutputRelativePath(cwd, path) }
	if len(ruleResult.ImportConventionViolations) > 0 {
		fmt.Fprintf(&out, "  %s Import Convention Issues (%d):\n", emoji.Error, len(ruleResult.ImportConventionViolations))

		violationsToDisplay := slices.Clone(ruleResult.ImportConventionViolations)

		// Sort violations by file path and then by import index
		slices.SortStableFunc(violationsToDisplay, func(a, b checks.ImportConventionViolation) int {
			if a.FilePath != b.FilePath {
				return strings.Compare(a.FilePath, b.FilePath)
			}
			if a.ImportIndex != b.ImportIndex {
				return a.ImportIndex - b.ImportIndex
			}
			if a.ImportIndex < 0 {
				if a.ImportRequest != b.ImportRequest {
					return strings.Compare(a.ImportRequest, b.ImportRequest)
				}
				return strings.Compare(a.ViolationType, b.ViolationType)
			}
			return 0
		})

		violationsToDisplay, remaining := limitConfigIssues(violationsToDisplay, listAll)

		// Group violations by file path
		violationsByFile := make(map[string][]checks.ImportConventionViolation)
		for _, violation := range violationsToDisplay {
			violationsByFile[violation.FilePath] = append(violationsByFile[violation.FilePath], violation)
		}

		// Sort file paths for consistent output
		var sortedFilePaths []string
		for filePath := range violationsByFile {
			sortedFilePaths = append(sortedFilePaths, filePath)
		}
		slices.Sort(sortedFilePaths)

		for _, filePath := range sortedFilePaths {
			fmt.Fprintf(&out, "    %s\n", getRelativePath(filePath))
			fileViolations := violationsByFile[filePath]
			for _, violation := range fileViolations {
				fmt.Fprintf(&out, "     - [%s] : \"%s\"\n", violation.ViolationType, singleLineIssueText(violation.ImportRequest))
			}
			fmt.Fprintf(&out, "\n")
		}

		if remaining > 0 {
			fmt.Fprintf(&out, "    ... and %d more import convention issues\n", remaining)
		}
	} else {
		fmt.Fprintf(&out, "  %s Import Conventions\n", emoji.Success)
	}
	return out.String()
}

func formatUnresolvedImportsConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	getRelativePath := func(path string) string { return configOutputRelativePath(cwd, path) }
	if len(ruleResult.UnresolvedImports) > 0 {
		fmt.Fprintf(&out, "  %s Unresolved Imports (%d):\n", emoji.Error, len(ruleResult.UnresolvedImports))

		// Sort all results before limiting
		unresolvedToDisplay := slices.Clone(ruleResult.UnresolvedImports)
		// Within a file the issues follow the source, so the report reads in the order
		// someone scrolls through the file rather than alphabetically by request.
		slices.SortFunc(unresolvedToDisplay, func(a, b checks.UnresolvedImport) int {
			if a.FilePath != b.FilePath {
				return strings.Compare(a.FilePath, b.FilePath)
			}
			if a.RequestStart != b.RequestStart {
				return int(a.RequestStart) - int(b.RequestStart)
			}
			return strings.Compare(a.Request, b.Request)
		})

		unresolvedToDisplay, remaining := limitConfigIssues(unresolvedToDisplay, listAll)

		// Group by file
		labeller := newUnresolvedLabeller(ruleResult.RulePath)
		unresolvedByFile := make(map[string][]string)
		for _, u := range unresolvedToDisplay {
			unresolvedByFile[u.FilePath] = append(unresolvedByFile[u.FilePath], singleLineIssueText(labeller.Label(u)))
		}

		var sortedFilePaths []string
		for fp := range unresolvedByFile {
			sortedFilePaths = append(sortedFilePaths, fp)
		}
		slices.Sort(sortedFilePaths)

		for _, filePath := range sortedFilePaths {
			fmt.Fprintf(&out, "    %s\n", getRelativePath(filePath))
			for _, req := range unresolvedByFile[filePath] {
				fmt.Fprintf(&out, "     - %s\n", req)
			}
		}

		if remaining > 0 {
			fmt.Fprintf(&out, "    ... and %d more unresolved import issues\n", remaining)
		}
	} else {
		fmt.Fprintf(&out, "  %s Unresolved Imports\n", emoji.Success)
	}
	return out.String()
}

func formatUnusedExportsConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	getRelativePath := func(path string) string { return configOutputRelativePath(cwd, path) }
	if len(ruleResult.UnusedExports) > 0 {
		fmt.Fprintf(&out, "  %s Unused Exports Issues (%d):\n", emoji.Error, len(ruleResult.UnusedExports))

		exportsToDisplay := slices.Clone(ruleResult.UnusedExports)

		// Sort exports by file path and then by export name
		slices.SortFunc(exportsToDisplay, func(a, b checks.UnusedExport) int {
			if a.FilePath != b.FilePath {
				return strings.Compare(a.FilePath, b.FilePath)
			}
			return strings.Compare(a.ExportName, b.ExportName)
		})

		exportsToDisplay, remaining := limitConfigIssues(exportsToDisplay, listAll)

		// Group by file path
		exportsByFile := make(map[string][]checks.UnusedExport)
		for _, ue := range exportsToDisplay {
			exportsByFile[ue.FilePath] = append(exportsByFile[ue.FilePath], ue)
		}

		var sortedFilePaths []string
		for filePath := range exportsByFile {
			sortedFilePaths = append(sortedFilePaths, filePath)
		}
		slices.Sort(sortedFilePaths)

		for _, filePath := range sortedFilePaths {
			fmt.Fprintf(&out, "    %s\n", getRelativePath(filePath))
			for _, ue := range exportsByFile[filePath] {
				if ue.IsType {
					fmt.Fprintf(&out, "     - %s (type)\n", ue.ExportName)
				} else {
					fmt.Fprintf(&out, "     - %s\n", ue.ExportName)
				}
			}
		}

		if remaining > 0 {
			fmt.Fprintf(&out, "    ... and %d more unused export issues\n", remaining)
		}
	} else {
		fmt.Fprintf(&out, "  %s Unused Exports\n", emoji.Success)
	}
	return out.String()
}

func formatDevDepsUsageOnProdConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	getRelativePath := func(path string) string { return configOutputRelativePath(cwd, path) }
	if len(ruleResult.RestrictedDevDependenciesUsageViolations) > 0 {
		fmt.Fprintf(&out, "  %s Dev Deps Usage On Prod Issues (%d):\n", emoji.Error, len(ruleResult.RestrictedDevDependenciesUsageViolations))

		violations := slices.Clone(ruleResult.RestrictedDevDependenciesUsageViolations)
		slices.SortFunc(violations, func(a, b checks.RestrictedDevDependenciesUsageViolation) int {
			if a.DevDependency != b.DevDependency {
				return strings.Compare(a.DevDependency, b.DevDependency)
			}
			if a.FilePath != b.FilePath {
				return strings.Compare(a.FilePath, b.FilePath)
			}
			return strings.Compare(a.EntryPoint, b.EntryPoint)
		})
		violationsToDisplay, remaining := limitConfigIssues(violations, listAll)

		// Group by dev dependency
		violationsByDep := make(map[string][]checks.RestrictedDevDependenciesUsageViolation)
		for _, violation := range violationsToDisplay {
			violationsByDep[violation.DevDependency] = append(violationsByDep[violation.DevDependency], violation)
		}

		// Sort dev dependencies for consistent output
		var sortedDeps []string
		for dep := range violationsByDep {
			sortedDeps = append(sortedDeps, dep)
		}
		slices.Sort(sortedDeps)

		for _, dep := range sortedDeps {
			fmt.Fprintf(&out, "    %s\n", dep)
			depViolations := violationsByDep[dep]
			for _, violation := range depViolations {
				entryPoint := ""
				if violation.EntryPoint != "" {
					entryPoint = " (from entry point: " + getRelativePath(violation.EntryPoint) + ")"
				}
				fmt.Fprintf(&out, "     - %s%s\n", getRelativePath(violation.FilePath), entryPoint)
			}
		}

		if remaining > 0 {
			fmt.Fprintf(&out, "    ... and %d more Dev Deps Usage On Prod Issues\n", remaining)
		}
	} else {
		fmt.Fprintf(&out, "  %s Dev Deps Usage On Prod\n", emoji.Success)
	}
	return out.String()
}

func formatRestrictedImportsConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	getRelativePath := func(path string) string { return configOutputRelativePath(cwd, path) }
	if len(ruleResult.RestrictedImportsViolations) > 0 {
		fmt.Fprintf(&out, "  %s Restricted Imports Issues (%d):\n", emoji.Error, len(ruleResult.RestrictedImportsViolations))

		violationsToDisplay := slices.Clone(ruleResult.RestrictedImportsViolations)
		slices.SortFunc(violationsToDisplay, func(a, b checks.RestrictedImportViolation) int {
			if a.EntryPoint != b.EntryPoint {
				return strings.Compare(a.EntryPoint, b.EntryPoint)
			}
			if a.ViolationType != b.ViolationType {
				return strings.Compare(a.ViolationType, b.ViolationType)
			}

			aItem := a.DeniedFile
			if a.ViolationType == "module" {
				if a.ImportRequest != "" {
					aItem = a.ImportRequest
				} else {
					aItem = a.DeniedModule
				}
			}
			bItem := b.DeniedFile
			if b.ViolationType == "module" {
				if b.ImportRequest != "" {
					bItem = b.ImportRequest
				} else {
					bItem = b.DeniedModule
				}
			}
			if aItem != bItem {
				return strings.Compare(aItem, bItem)
			}

			return strings.Compare(a.ImporterFile, b.ImporterFile)
		})
		// Hints use the same sorted findings as the report without modifying the
		// original result, which may subsequently be rendered as JSON.
		ruleResult.RestrictedImportsViolations = violationsToDisplay

		violationsToDisplay, remaining := limitConfigIssues(violationsToDisplay, listAll)

		violationsByEntryPoint := make(map[string][]string)
		seenByEntryPoint := make(map[string]map[string]bool)

		for _, violation := range violationsToDisplay {
			entryPoint := getRelativePath(violation.EntryPoint)
			if _, ok := seenByEntryPoint[entryPoint]; !ok {
				seenByEntryPoint[entryPoint] = make(map[string]bool)
			}

			item := ""
			if violation.ViolationType == "file" {
				item = getRelativePath(violation.DeniedFile)
			} else if violation.ImportRequest != "" {
				item = violation.ImportRequest
			} else {
				item = violation.DeniedModule
			}

			if !seenByEntryPoint[entryPoint][item] {
				violationsByEntryPoint[entryPoint] = append(violationsByEntryPoint[entryPoint], item)
				seenByEntryPoint[entryPoint][item] = true
			}
		}

		var sortedEntryPoints []string
		for entryPoint := range violationsByEntryPoint {
			sortedEntryPoints = append(sortedEntryPoints, entryPoint)
		}
		slices.Sort(sortedEntryPoints)

		for _, entryPoint := range sortedEntryPoints {
			if entryPoint != "" {
				fmt.Fprintf(&out, "    %s\n", entryPoint)
			}
			items := violationsByEntryPoint[entryPoint]
			slices.Sort(items)
			for _, item := range items {
				fmt.Fprintf(&out, "     ➞ %s\n", item)
			}
		}

		if remaining > 0 {
			fmt.Fprintf(&out, "    ... and %d more restricted import issues\n", remaining)
		}

		out.WriteString(formatRestrictedImportsResolveHint(ruleResult, cwd))
	} else {
		fmt.Fprintf(&out, "  %s Restricted Imports\n", emoji.Success)
	}
	return out.String()
}

func formatRestrictedImportersConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	getRelativePath := func(path string) string { return configOutputRelativePath(cwd, path) }
	if len(ruleResult.RestrictedImportersViolations) > 0 {
		fmt.Fprintf(&out, "  %s Restricted Importers Issues (%d):\n", emoji.Error, len(ruleResult.RestrictedImportersViolations))

		violationsToDisplay, remaining := limitConfigIssues(ruleResult.RestrictedImportersViolations, listAll)

		violationsByEntryPoint := make(map[string][]string)
		seenByEntryPoint := make(map[string]map[string]bool)
		var sortedEntryPoints []string

		for _, violation := range violationsToDisplay {
			entryPoint := getRelativePath(violation.EntryPoint)
			if _, ok := seenByEntryPoint[entryPoint]; !ok {
				seenByEntryPoint[entryPoint] = make(map[string]bool)
				sortedEntryPoints = append(sortedEntryPoints, entryPoint)
			}

			item := violation.Module
			if item == "" {
				item = getRelativePath(violation.File)
			}

			if !seenByEntryPoint[entryPoint][item] {
				violationsByEntryPoint[entryPoint] = append(violationsByEntryPoint[entryPoint], item)
				seenByEntryPoint[entryPoint][item] = true
			}
		}

		slices.Sort(sortedEntryPoints)
		for _, entryPoint := range sortedEntryPoints {
			fmt.Fprintf(&out, "    %s\n", entryPoint)
			items := violationsByEntryPoint[entryPoint]
			slices.Sort(items)
			for _, item := range items {
				fmt.Fprintf(&out, "     ➞ %s\n", item)
			}
		}

		if remaining > 0 {
			fmt.Fprintf(&out, "    ... and %d more restricted importer issues\n", remaining)
		}

		out.WriteString(formatRestrictedImportersResolveHint(ruleResult, cwd))
	} else {
		fmt.Fprintf(&out, "  %s Restricted Importers\n", emoji.Success)
	}
	return out.String()
}

func formatRestrictedDirectImportersConsole(ruleResult config.RuleResult, cwd string, listAll bool) string {
	var out strings.Builder
	getRelativePath := func(path string) string { return configOutputRelativePath(cwd, path) }
	if len(ruleResult.RestrictedDirectImportersViolations) > 0 {
		fmt.Fprintf(&out, "  %s Restricted Direct Importers Issues (%d):\n", emoji.Error, len(ruleResult.RestrictedDirectImportersViolations))

		violationsToDisplay, remaining := limitConfigIssues(ruleResult.RestrictedDirectImportersViolations, listAll)

		// Group by importer file (the actionable location), listing the targets it imports.
		targetsByImporter := make(map[string][]string)
		seenByImporter := make(map[string]map[string]bool)
		var sortedImporters []string

		for _, violation := range violationsToDisplay {
			importer := getRelativePath(violation.ImporterFile)
			if _, ok := seenByImporter[importer]; !ok {
				seenByImporter[importer] = make(map[string]bool)
				sortedImporters = append(sortedImporters, importer)
			}

			item := violation.Module
			if item == "" && violation.File == "" {
				item = violation.ImportRequest
			}
			if item == "" {
				item = getRelativePath(violation.File)
			}

			if !seenByImporter[importer][item] {
				targetsByImporter[importer] = append(targetsByImporter[importer], item)
				seenByImporter[importer][item] = true
			}
		}

		slices.Sort(sortedImporters)
		for _, importer := range sortedImporters {
			fmt.Fprintf(&out, "    %s\n", importer)
			items := targetsByImporter[importer]
			slices.Sort(items)
			for _, item := range items {
				fmt.Fprintf(&out, "     ➞ %s\n", item)
			}
		}

		if remaining > 0 {
			fmt.Fprintf(&out, "    ... and %d more restricted direct importer issues\n", remaining)
		}
	} else {
		fmt.Fprintf(&out, "  %s Restricted Direct Importers\n", emoji.Success)
	}
	return out.String()
}

// formatDuplicatedCodeResult reports one duplicated-code detection. A workspace may have
// several, so each states its own comparison mode and its own verdict.
func formatDuplicatedCodeResult(d config.DuplicatedCodeRuleResult, cwd string) string {
	var out strings.Builder
	dup := d.Result

	if d.Err != nil {
		// The detection did not run. Saying so plainly beats reporting a count of zero,
		// which is what skipping it would have looked like.
		fmt.Fprintf(&out, "  %s Duplicated Code: could not run\n", emoji.Error)
		for _, line := range strings.Split(d.Err.Error(), "\n") {
			fmt.Fprintf(&out, "      %s\n", line)
		}
		return out.String()
	}

	if dup.SnapshotWritten {
		fmt.Fprintf(&out, "  %s Duplicated Code: snapshot updated (%s acknowledged) -> %s\n",
			emoji.Success, plural.Count(dup.Snippets, "pattern", "patterns"), configOutputRelativePath(cwd, dup.SnapshotPath))
		return out.String()
	}
	if dup.Delta != nil {
		// With a baseline the total is not the news; the change is.
		if dup.Delta.IsClean() {
			fmt.Fprintf(&out, "  %s Duplicated Code: snapshot unchanged (%s acknowledged)\n",
				emoji.Success, plural.Count(dup.Delta.Unchanged, "pattern", "patterns"))
			return out.String()
		}
		fmt.Fprintf(&out, "  %s Duplicated Code: %s\n",
			emoji.Error, dupcode.DeltaSummary(dup.Delta))
		fmt.Fprintf(&out, "      %s\n", d.Command)
		fmt.Fprintf(&out, "      Acknowledge with: rev-dep config run --update-snapshot\n")
		return out.String()
	}

	if dup.Snippets == 0 && !dup.SnapshotMissing {
		fmt.Fprintf(&out, "  %s Duplicated Code\n", emoji.Success)
		return out.String()
	}

	detail := fmt.Sprintf("%s in %s (%s)",
		plural.Count(dup.Snippets, "duplicated snippet", "duplicated snippets"),
		plural.Count(dup.Files, "file", "files"),
		plural.Count(dup.Occurrences, "occurrence", "occurrences"))
	// State what was searched as well: when this number and the command's disagree, the
	// scanned counts are what identify why.
	detail += ", scanned " + plural.Count(dup.Scanned, "file", "files")
	if dup.Ignored > 0 {
		detail += fmt.Sprintf(", %d ignored", dup.Ignored)
	}

	fmt.Fprintf(&out, "  %s Duplicated Code: %s\n", emoji.Error, detail)

	// Only reachable with a missing baseline: nothing to look at, only a baseline to record.
	if dup.Snippets == 0 {
		fmt.Fprintf(&out, "      No baseline recorded yet at %s\n", configOutputRelativePath(cwd, dup.SnapshotPath))
		fmt.Fprintf(&out, "      To record it, run: rev-dep config run --update-snapshot\n")
		return out.String()
	}

	// The baseline line belongs beside the option it enables, not between the count and the command.
	fmt.Fprintf(&out, "      To see them, run:\n        %s\n", d.Command)

	if dup.SnapshotMissing {
		fmt.Fprintf(&out, "      No baseline recorded yet at %s\n", configOutputRelativePath(cwd, dup.SnapshotPath))
		fmt.Fprintf(&out, "      To accept them as the baseline, run: rev-dep config run --update-snapshot\n")
		return out.String()
	}
	fmt.Fprintf(&out, "      To accept the current duplication, set \"snapshotPath\" on this "+
		"detection and run: rev-dep config run --update-snapshot\n")
	return out.String()
}

// formatEmptyWorkspacesSummary repeats, next to the final verdict, which workspaces matched no
// files. Their checks ran on nothing, so a pass for them proves nothing - easy to miss when the
// per-workspace warning scrolls past in CI or agent logs. Workspaces with failOnEmptyWorkspace
// are errors (they also failed the run); the rest are warnings.
func formatEmptyWorkspacesSummary(result *config.ConfigProcessingResult) string {
	var out strings.Builder
	var failed, warned []string
	for _, ruleResult := range result.RuleResults {
		if !ruleResult.IsEmpty() {
			continue
		}
		if ruleResult.EmptyWorkspaceFailed() {
			failed = append(failed, workspaceLabel(ruleResult.RulePath))
		} else {
			warned = append(warned, workspaceLabel(ruleResult.RulePath))
		}
	}

	if len(failed) > 0 {
		fmt.Fprintf(&out, "%s %s checked 0 files (failOnEmptyWorkspace): %s\n", emoji.Error, plural.Count(len(failed), "workspace", "workspaces"), strings.Join(failed, ", "))
	}
	if len(warned) > 0 {
		fmt.Fprintf(&out, "%s %s checked 0 files: %s\n", emoji.Warning, plural.Count(len(warned), "workspace", "workspaces"), strings.Join(warned, ", "))
	}
	return out.String()
}

func formatConfigResults(result *config.ConfigProcessingResult, cwd string, listAll bool) string {
	var out strings.Builder
	out.WriteString(formatFindingsBaselineConsole(result.Baseline, cwd, listAll, result.RuleResults))

	shouldWarnAboutImportConventionWithPJsonImports := false

	for _, ruleResult := range result.RuleResults {
		shouldWarnAboutImportConventionWithPJsonImports = shouldWarnAboutImportConventionWithPJsonImports || ruleResult.ShouldWarnAboutImportConventionWithPJsonImports

		if ruleResult.RulePath != "" {
			out.WriteString(formatConfigWorkspaceHeading(ruleResult.RulePath, &ruleResult.FileCount))
		}

		out.WriteString(formatConfigRuleChecks(ruleResult, cwd, listAll))

		if ruleResult.IsEmpty() {
			marker := emoji.Warning
			if ruleResult.EmptyWorkspaceFailed() {
				marker = emoji.Error
			}
			fmt.Fprintf(&out, "  %s No files found for this workspace - check if the path is correct\n", marker)
		}

		if ruleResult.MissingPackageJson {
			packageJsonPath := filepath.Join(cwd, ruleResult.RulePath, "package.json")
			fmt.Fprintf(&out, "  %s Warning: Workspace path missing package.json - some features may not work (missing: %s)\n", emoji.Warning, packageJsonPath)
		}
	}

	// Print final verdict
	if !result.HasFailures {
		fmt.Fprintf(&out, "\n%s All checks passed!\n", emoji.Success)
	} else {
		fmt.Fprintf(&out, "\n%s Checks failed! See details above.\n", emoji.Error)
	}
	out.WriteString(formatEmptyWorkspacesSummary(result))

	// Print autofix summary if any fixes were applied or unfixable issues found
	if result.FixedFilesCount > 0 || result.FixedImportsCount > 0 || result.DeletedFilesCount > 0 {
		var summary []string
		if result.FixedImportsCount > 0 || result.FixedFilesCount > 0 {
			summary = append(summary, fmt.Sprintf("fixed %d imports in %d files", result.FixedImportsCount, result.FixedFilesCount))
		}
		if result.DeletedFilesCount > 0 {
			summary = append(summary, fmt.Sprintf("removed %d orphan files", result.DeletedFilesCount))
		}

		if len(summary) > 0 {
			// Capitalize first letter of first summary part
			summary[0] = strings.ToUpper(summary[0][:1]) + summary[0][1:]
			fmt.Fprintf(&out, "%s %s\n", emoji.Fix, strings.Join(summary, ", "))
		}
	}

	if result.FixableIssuesCount > 0 {
		fmt.Fprintf(&out, "%s Fixable issues: %d. Use '--fix' flag to autofix.\n", emoji.Tip, result.FixableIssuesCount)
	}

	if result.UnfixableAliasingCount > 0 {
		fmt.Fprintf(&out, "%s Warning: %d inter-domain relative imports could not be automatically fixed because target domains lack aliases or are not defined in config.\n", emoji.Warning, result.UnfixableAliasingCount)
	}
	if shouldWarnAboutImportConventionWithPJsonImports {
		fmt.Fprintln(&out, emoji.Warning+" Warning: Support for package.json imports map aliases is not yet implemented for import conventions checks")
	}
	return out.String()
}
