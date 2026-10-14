package cli

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"rev-dep-go/internal/baseline"
	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/config"
	"rev-dep-go/internal/node"
)

const findingsBaselineHint = "Fix new issues or acknowledge the current findings with: rev-dep config run --update-snapshot\n"

func formatFindingsBaselineSummary(result *baseline.Result) string {
	if result == nil {
		return ""
	}
	scope := ""
	if len(result.Workspaces) > 0 {
		scope = " (workspaces: " + strings.Join(result.Workspaces, ", ") + ")"
	}
	switch result.Status {
	case "written":
		return fmt.Sprintf("Baseline snapshot written: %s (%d acknowledged findings)\n", result.Path, result.Counts.Acknowledged)
	case "matched":
		return fmt.Sprintf("Baseline snapshot matched%s: %s (%d acknowledged findings)\n", scope, result.Path, result.Counts.Acknowledged)
	default:
		return fmt.Sprintf("Baseline snapshot differs%s: %s (%d new, %d obsolete, %d acknowledged findings)\n", scope, result.Path, result.Counts.New, result.Counts.Obsolete, result.Counts.Acknowledged)
	}
}

// The baseline adds its summary and heading, then composes the same workspace
// and detector renderers as the current run.
func formatFindingsBaselineConsole(result *baseline.Result, cwd string, listAll bool, currentRules []config.RuleResult) string {
	var out strings.Builder
	out.WriteString(formatFindingsBaselineSummary(result))
	if result == nil || result.Status != "different" {
		return out.String()
	}
	if len(result.Obsolete) > 0 {
		out.WriteString("\nObsolete findings:\n")
		for _, rule := range obsoleteFindingRules(result.Obsolete, cwd, currentRules) {
			out.WriteString(formatConfigWorkspaceHeading(rule.RulePath, nil))
			out.WriteString(formatConfigRuleChecks(rule, cwd, listAll))
		}
	}
	out.WriteString("\n" + findingsBaselineHint)
	return out.String()
}

func formatFindingsBaselineIssuesList(result *baseline.Result, cwd string) string {
	var out strings.Builder
	out.WriteString(formatFindingsBaselineSummary(result))
	if result == nil || result.Status != "different" {
		return out.String()
	}
	if len(result.Obsolete) > 0 {
		out.WriteString("\nObsolete findings:\n\n")
		var rules []jsonRuleResult
		for _, rule := range obsoleteFindingRules(result.Obsolete, cwd, nil) {
			// No source lookup: obsolete findings may refer to deleted files,
			// and their recorded identity deliberately has no source positions.
			jsonRule := buildJSONRuleResult(rule, cwd, nil)
			if len(rule.CircularComponents) > 0 {
				componentCheck := &jsonCheckResult{Status: "fail"}
				for _, files := range rule.CircularComponents {
					relative := make([]string, len(files))
					for i, file := range files {
						relative[i] = configOutputRelativePath(cwd, file)
					}
					componentCheck.Issues = append(componentCheck.Issues, circularComponentIssue{Files: relative})
				}
				jsonRule.Checks.CircularDependencies = componentCheck
			}
			rules = append(rules, jsonRule)
		}
		out.WriteString(formatIssuesListGroups(buildIssuesListGroups(rules)))
	}
	out.WriteString(findingsBaselineHint + "\n")
	return out.String()
}

// Convert stored identities to the usual detector result types. Diagnostic
// details absent from the baseline stay empty; formatting belongs to the shared
// renderers rather than this adapter.
func obsoleteFindingRules(findings []baseline.Finding, cwd string, currentRules []config.RuleResult) []config.RuleResult {
	workspaceRank := map[string]int{}
	workspaceLabels := map[string]string{}
	configuredChecks := map[string][]string{}
	for _, rule := range currentRules {
		path := rule.RulePath
		if filepath.IsAbs(path) {
			path = configOutputRelativePath(cwd, path)
		}
		scope := filepath.ToSlash(filepath.Clean(path))
		if _, exists := workspaceRank[scope]; !exists {
			workspaceRank[scope] = len(workspaceRank)
			workspaceLabels[scope] = rule.RulePath
		}
		order := rule.DetectorOrder
		if len(order) == 0 {
			order = rule.EnabledChecks
		}
		configuredChecks[scope] = append(configuredChecks[scope], order...)
	}
	rank := func(workspace string) int {
		if index, exists := workspaceRank[workspace]; exists {
			return index
		}
		return len(workspaceRank)
	}
	detectorOrders := map[string][]string{}
	for _, finding := range findings {
		if _, exists := detectorOrders[finding.Workspace]; !exists {
			detectorOrders[finding.Workspace] = config.OrderDetectors(baseline.Detectors(), configuredChecks[finding.Workspace])
		}
	}
	ordered := slices.Clone(findings)
	slices.SortFunc(ordered, func(a, b baseline.Finding) int {
		if a.Workspace != b.Workspace {
			if difference := rank(a.Workspace) - rank(b.Workspace); difference != 0 {
				return difference
			}
			return strings.Compare(a.Workspace, b.Workspace)
		}
		if a.Detector != b.Detector {
			order := detectorOrders[a.Workspace]
			return slices.Index(order, a.Detector) - slices.Index(order, b.Detector)
		}
		return strings.Compare(baseline.FindingKey(a), baseline.FindingKey(b))
	})
	absolute := func(path string) string {
		if path == "" {
			return ""
		}
		return filepath.Join(cwd, filepath.FromSlash(path))
	}
	var rules []config.RuleResult
	lastWorkspace := ""
	for _, finding := range ordered {
		if len(rules) == 0 || lastWorkspace != finding.Workspace {
			label := workspaceLabels[finding.Workspace]
			if label == "" {
				label = finding.Workspace
			}
			rules = append(rules, config.RuleResult{RulePath: label})
			lastWorkspace = finding.Workspace
		}
		rule := &rules[len(rules)-1]
		if len(rule.EnabledChecks) == 0 || rule.EnabledChecks[len(rule.EnabledChecks)-1] != finding.Detector {
			rule.EnabledChecks = append(rule.EnabledChecks, finding.Detector)
		}
		issue := finding.Issue
		switch finding.Detector {
		case "circular-imports":
			files := make([]string, len(issue.Files))
			for i, file := range issue.Files {
				files[i] = absolute(file)
			}
			rule.CircularComponents = append(rule.CircularComponents, files)
		case "orphan-files":
			rule.OrphanFiles = append(rule.OrphanFiles, absolute(issue.File))
		case "module-boundaries":
			rule.ModuleBoundaryViolations = append(rule.ModuleBoundaryViolations, checks.ModuleBoundaryViolation{
				FilePath: absolute(issue.File), ImportPath: absolute(issue.Target),
			})
		case "unused-node-modules":
			rule.UnusedNodeModules = append(rule.UnusedNodeModules, node.UnusedNodeModuleIssue{
				ModuleName: issue.Module, PackageJsonPath: absolute(issue.PackageJSON),
			})
		case "missing-node-modules":
			// The usual detector result groups all importing files by module.
			index := slices.IndexFunc(rule.MissingNodeModules, func(m node.MissingNodeModuleResult) bool { return m.ModuleName == issue.Module })
			if index < 0 {
				index = len(rule.MissingNodeModules)
				rule.MissingNodeModules = append(rule.MissingNodeModules, node.MissingNodeModuleResult{ModuleName: issue.Module})
			}
			rule.MissingNodeModules[index].ImportedFrom = append(rule.MissingNodeModules[index].ImportedFrom, absolute(issue.File))
		case "import-conventions":
			rule.ImportConventionViolations = append(rule.ImportConventionViolations, checks.ImportConventionViolation{
				FilePath: absolute(issue.File), ImportRequest: issue.Request, ViolationType: issue.ViolationType, ImportIndex: -1,
			})
		case "unresolved-imports":
			rule.UnresolvedImports = append(rule.UnresolvedImports, checks.UnresolvedImport{
				FilePath: absolute(issue.File), Request: issue.Request, IsNonLiteral: issue.Kind == "nonliteral",
			})
		case "unused-exports":
			rule.UnusedExports = append(rule.UnusedExports, checks.UnusedExport{
				FilePath: absolute(issue.File), ExportName: issue.Export, IsType: issue.IsType != nil && *issue.IsType,
			})
		case "dev-deps-usage-on-prod":
			rule.RestrictedDevDependenciesUsageViolations = append(rule.RestrictedDevDependenciesUsageViolations, checks.RestrictedDevDependenciesUsageViolation{
				FilePath: absolute(issue.File), DevDependency: issue.Module,
			})
		case "restricted-imports":
			rule.RestrictedImportsViolations = append(rule.RestrictedImportsViolations, checks.RestrictedImportViolation{
				ViolationType: issue.Kind, DeniedFile: absolute(issue.Target), ImportRequest: issue.Request,
			})
		case "restricted-importers":
			rule.RestrictedImportersViolations = append(rule.RestrictedImportersViolations, checks.RestrictedImporterViolation{
				EntryPoint: absolute(issue.EntryPoint), File: absolute(issue.Target), Module: issue.Module,
			})
		case "restricted-direct-importers":
			rule.RestrictedDirectImportersViolations = append(rule.RestrictedDirectImportersViolations, checks.RestrictedDirectImporterViolation{
				ViolationType: issue.Kind, ImporterFile: absolute(issue.File), File: absolute(issue.Target), ImportRequest: issue.Request,
			})
		}
	}
	for i := range rules {
		rule := &rules[i]
		// Reconstructed identities use the same detector comparators as live
		// results. Historical source positions and witnesses remain unavailable.
		slices.SortFunc(rule.MissingNodeModules, node.CompareMissingNodeModules)
		for _, missing := range rule.MissingNodeModules {
			slices.Sort(missing.ImportedFrom)
		}
		slices.SortFunc(rule.RestrictedImportersViolations, checks.CompareRestrictedImporters)
		slices.SortFunc(rule.RestrictedDirectImportersViolations, checks.CompareRestrictedDirectImporters)
	}
	return rules
}
