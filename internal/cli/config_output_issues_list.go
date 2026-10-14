package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"rev-dep-go/internal/config"
	"rev-dep-go/internal/dupcode"
	"rev-dep-go/internal/plural"
)

func runConfigWithIssuesListOutput(cfg config.RevDepConfig, cwd string, runConfigFix bool, runConfigRecheck bool) error {
	if err := filterRunConfigRules(&cfg, runConfigRules); err != nil {
		return err
	}

	result, err := processConfigRun(&cfg, cwd, runConfigFix, runConfigRecheck, true)
	if err != nil {
		return fmt.Errorf("Error processing config: %v", err)
	}

	locator := newFileLocationResolver(cwd, result.FullTree)
	rules := make([]jsonRuleResult, 0, len(result.RuleResults))
	for _, ruleResult := range result.RuleResults {
		rules = append(rules, buildJSONRuleResult(ruleResult, cwd, locator))
	}

	output := formatIssuesListOutput(rules)
	if result.Baseline != nil && output == "No issues found\n" {
		output = ""
	}
	output = formatFindingsBaselineIssuesList(result.Baseline, cwd) + output
	if output != "" {
		fmt.Print(output)
	}

	lintHasErrors := false
	if configRunLintWanted() {
		counts, err := lintConfigAfterRun(cwd, result)
		if err != nil {
			return err
		}
		printConfigLintSummary(counts)
		lintHasErrors = counts.Errors > 0
	}

	// Same exit rule as the default output: with --fix, issues that were all fixed don't fail the run.
	if shouldConfigRunExitNonZero(result, runConfigFix) || lintHasErrors {
		os.Exit(1)
	}

	return nil
}

type issuesListGroup struct {
	Title string
	Items []issuesListItem
}

// duplicatedCodeIssueLabel states what failed in one line. It mirrors the console wording
// so the two formats describe the same run the same way.
func duplicatedCodeIssueLabel(d jsonDuplicatedCodeResult) string {
	if d.Error != "" {
		// The first line only: the full explanation belongs in the console output, and an
		// issues list is one line per issue.
		return "could not run: " + strings.SplitN(d.Error, "\n", 2)[0]
	}
	if s := d.Snapshot; s != nil {
		if s.Missing {
			return fmt.Sprintf("snapshot %s does not exist", s.Path)
		}
		// With a baseline the total is not the news; the change is. The counts come straight
		// off the embedded struct rather than being copied field by field, which is how a
		// category went missing here before.
		return dupcode.DeltaCountsSummary(s.DeltaCounts)
	}
	return fmt.Sprintf("%s in %s (%s)",
		plural.Count(d.Snippets, "duplicated snippet", "duplicated snippets"),
		plural.Count(d.Files, "file", "files"),
		plural.Count(d.Occurrences, "occurrence", "occurrences"))
}

type issuesListItem struct {
	Label    string
	Location string
}

// A component identifies circular findings when a diagnostic cycle is absent.
type circularComponentIssue struct {
	Files []string
}

func circularComponentLabel(files []string) string {
	return "Files: " + strings.Join(files, ", ")
}

func formatIssuesListOutput(rules []jsonRuleResult) string {
	groups := buildIssuesListGroups(rules)
	if len(groups) == 0 {
		return "No issues found\n"
	}
	return formatIssuesListGroups(groups)
}

func formatIssuesListGroups(groups []issuesListGroup) string {
	lines := make([]string, 0, 64)
	for i, group := range groups {
		if len(group.Items) == 0 {
			continue
		}
		if i > 0 && len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("%s (%d):", group.Title, len(group.Items)))
		formatted := formatIssuesListItems(group.Items)
		for _, line := range formatted {
			lines = append(lines, "  "+line)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n") + "\n"
}

func buildIssuesListRuleGroups(rule jsonRuleResult) []issuesListGroup {
	var empty []issuesListItem
	if rule.EmptyWorkspace == "fail" {
		empty = []issuesListItem{{Label: workspaceLabel(rule.Path) + " (0 files)"}}
	}
	groups := []issuesListGroup{
		{Title: "Empty Workspaces", Items: empty},
		{Title: "Circular Dependencies Issues", Items: formatCircularDependenciesIssuesList(rule.Checks.CircularDependencies)},
		{Title: "Orphan Files Issues", Items: formatOrphanFilesIssuesList(rule.Checks.OrphanFiles)},
		{Title: "Module Boundary Issues", Items: formatModuleBoundariesIssuesList(rule.Checks.ModuleBoundaries)},
		{Title: "Unused Node Modules Issues", Items: formatUnusedNodeModulesIssuesList(rule.Checks.UnusedNodeModules)},
		{Title: "Missing Node Modules Issues", Items: formatMissingNodeModulesIssuesList(rule.Checks.MissingNodeModules)},
		{Title: "Import Convention Issues", Items: formatImportConventionsIssuesList(rule.Checks.ImportConventions)},
		{Title: "Unresolved Imports", Items: formatUnresolvedImportsIssuesList(rule.Checks.UnresolvedImports)},
		{Title: "Unused Exports Issues", Items: formatUnusedExportsIssuesList(rule.Checks.UnusedExports)},
		{Title: "Dev Deps Usage On Prod Issues", Items: formatDevDepsUsageOnProdIssuesList(rule.Checks.RestrictedDevDependenciesUsage)},
		{Title: "Restricted Imports Issues", Items: formatRestrictedImportsIssuesList(rule.Checks.RestrictedImports)},
		{Title: "Restricted Importers Issues", Items: formatRestrictedImportersIssuesList(rule.Checks.RestrictedImporters)},
		{Title: "Restricted Direct Importers Issues", Items: formatRestrictedDirectImportersIssuesList(rule.Checks.RestrictedDirectImporters)},
		{Title: "Duplicated Code Issues", Items: formatDuplicatedCodeIssuesList(rule.Checks.DuplicatedCode)},
	}
	for _, group := range groups {
		sortIssuesListItems(group.Items)
	}
	return groups
}

// Both sections retain the existing global detector groups and row ordering.
func buildIssuesListGroups(rules []jsonRuleResult) []issuesListGroup {
	var groups []issuesListGroup
	for _, rule := range rules {
		ruleGroups := buildIssuesListRuleGroups(rule)
		if groups == nil {
			groups = make([]issuesListGroup, len(ruleGroups))
		}
		for i, group := range ruleGroups {
			groups[i].Title = group.Title
			groups[i].Items = append(groups[i].Items, group.Items...)
		}
	}
	// The per-rule composition includes empty groups, keeping detector order
	// consistent even when the first workspace has no findings for a detector.
	var result []issuesListGroup
	for _, group := range groups {
		if len(group.Items) > 0 {
			sortIssuesListItems(group.Items)
			result = append(result, group)
		}
	}
	return result
}

func sortIssuesListItems(items []issuesListItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Location != items[j].Location {
			if items[i].Location == "" {
				return false
			}
			if items[j].Location == "" {
				return true
			}
			return items[i].Location < items[j].Location
		}
		return items[i].Label < items[j].Label
	})
}

// mapIssuesListItems keeps collection separate from each detector's label and
// location. Most detectors produce one row per issue.
func mapIssuesListItems[T any](check *jsonCheckResult, item func(T) issuesListItem) []issuesListItem {
	var items []issuesListItem
	if check == nil {
		return items
	}
	for _, issue := range check.Issues {
		if value, ok := issue.(T); ok {
			row := item(value)
			if row.Label != "" {
				items = append(items, row)
			}
		}
	}
	return items
}

func formatCircularDependenciesIssuesList(check *jsonCheckResult) []issuesListItem {
	var items []issuesListItem
	if check == nil {
		return items
	}
	for _, issue := range check.Issues {
		switch value := issue.(type) {
		case jsonCircularDependencyIssue:
			items = append(items, issuesListItem{Label: strings.Join(value.Cycle, " -> ")})
		case circularComponentIssue:
			items = append(items, issuesListItem{Label: circularComponentLabel(value.Files)})
		}
	}
	return items
}

func formatOrphanFilesIssuesList(check *jsonCheckResult) []issuesListItem {
	return mapIssuesListItems(check, func(v jsonOrphanFileIssue) issuesListItem {
		return issuesListItem{Label: v.FilePath}
	})
}

func formatModuleBoundariesIssuesList(check *jsonCheckResult) []issuesListItem {
	return mapIssuesListItems(check, func(v jsonModuleBoundaryIssue) issuesListItem {
		label := v.ImportPath
		if label == "" {
			label = v.RuleName
		}
		return issuesListItem{Label: label, Location: formatIssueLocationWithFields(v.FilePath, v.jsonLocationFields)}
	})
}

func formatUnusedNodeModulesIssuesList(check *jsonCheckResult) []issuesListItem {
	return mapIssuesListItems(check, func(v jsonUnusedNodeModuleIssue) issuesListItem {
		return issuesListItem{Label: v.ModuleName, Location: formatIssueLocationWithFields(v.PackageJsonPath, v.jsonLocationFields)}
	})
}

func formatMissingNodeModulesIssuesList(check *jsonCheckResult) []issuesListItem {
	var items []issuesListItem
	if check == nil {
		return items
	}
	for _, issue := range check.Issues {
		v, ok := issue.(jsonMissingNodeModuleIssue)
		if !ok || v.ModuleName == "" {
			continue
		}
		if len(v.Locations) > 0 {
			for _, loc := range v.Locations {
				items = append(items, issuesListItem{Label: v.ModuleName, Location: formatIssueLocation(loc.FilePath, loc.StartLine, loc.StartCol)})
			}
		} else if len(v.ImportedFrom) > 0 {
			for _, file := range v.ImportedFrom {
				items = append(items, issuesListItem{Label: v.ModuleName, Location: file})
			}
		} else {
			items = append(items, issuesListItem{Label: v.ModuleName})
		}
	}
	return items
}

func formatImportConventionsIssuesList(check *jsonCheckResult) []issuesListItem {
	return mapIssuesListItems(check, func(v jsonImportConventionIssue) issuesListItem {
		return issuesListItem{Label: v.ImportRequest, Location: formatIssueLocationWithFields(v.FilePath, v.jsonLocationFields)}
	})
}

func formatUnresolvedImportsIssuesList(check *jsonCheckResult) []issuesListItem {
	return mapIssuesListItems(check, func(v jsonUnresolvedImportIssue) issuesListItem {
		request := v.Request
		if v.NonLiteral {
			call := v.callText
			if call == "" {
				call = v.Request
			}
			request = nonLiteralLabelFor(call)
		}
		return issuesListItem{Label: request, Location: formatIssueLocationWithFields(v.FilePath, v.jsonLocationFields)}
	})
}

func formatUnusedExportsIssuesList(check *jsonCheckResult) []issuesListItem {
	return mapIssuesListItems(check, func(v jsonUnusedExportIssue) issuesListItem {
		return issuesListItem{Label: v.ExportName, Location: formatIssueLocationWithFields(v.FilePath, v.jsonLocationFields)}
	})
}

func formatDevDepsUsageOnProdIssuesList(check *jsonCheckResult) []issuesListItem {
	return mapIssuesListItems(check, func(v jsonRestrictedDevDepsIssue) issuesListItem {
		return issuesListItem{Label: v.DevDependency, Location: formatIssueLocationWithFields(v.FilePath, v.jsonLocationFields)}
	})
}

func formatRestrictedImportsIssuesList(check *jsonCheckResult) []issuesListItem {
	return mapIssuesListItems(check, func(v jsonRestrictedImportIssue) issuesListItem {
		label := v.ImportRequest
		if label == "" {
			label = v.DeniedModule
			if label == "" {
				label = v.DeniedFile
			}
		}
		return issuesListItem{Label: label, Location: formatIssueLocationWithFields(v.ImporterFile, v.jsonLocationFields)}
	})
}

func formatRestrictedImportersIssuesList(check *jsonCheckResult) []issuesListItem {
	return mapIssuesListItems(check, func(v jsonRestrictedImporterIssue) issuesListItem {
		target := v.File
		if target == "" {
			target = "module " + v.Module
		}
		return issuesListItem{Label: target, Location: v.EntryPoint}
	})
}

func formatRestrictedDirectImportersIssuesList(check *jsonCheckResult) []issuesListItem {
	return mapIssuesListItems(check, func(v jsonRestrictedDirectImporterIssue) issuesListItem {
		target := v.File
		if target == "" {
			module := v.Module
			if module == "" {
				module = v.ImportRequest
			}
			target = "module " + module
		}
		return issuesListItem{Label: target, Location: v.ImporterFile}
	})
}

// A failing duplication check produces one row, rather than one row per block.
func formatDuplicatedCodeIssuesList(detections []jsonDuplicatedCodeResult) []issuesListItem {
	var items []issuesListItem
	for _, detection := range detections {
		if detection.Status == "fail" {
			items = append(items, issuesListItem{Label: duplicatedCodeIssueLabel(detection), Location: detection.Command})
		}
	}
	return items
}

func formatIssueLocation(filePath string, line int, col int) string {
	if filePath == "" {
		filePath = "unknown"
	}
	if line < 0 {
		line = 0
	}
	if col < 0 {
		col = 0
	}
	return fmt.Sprintf("%s:%d:%d", filePath, line, col)
}

func formatIssueLocationWithFields(filePath string, fields jsonLocationFields) string {
	if fields.StartLine == nil && fields.StartCol == nil {
		return filePath
	}
	line := 0
	col := 0
	if fields.StartLine != nil {
		line = *fields.StartLine
	}
	if fields.StartCol != nil {
		col = *fields.StartCol
	}
	return formatIssueLocation(filePath, line, col)
}

func formatIssuesListItems(items []issuesListItem) []string {
	if len(items) == 0 {
		return nil
	}
	maxLabel := 0
	for _, item := range items {
		if item.Location == "" {
			continue
		}
		if width := len(singleLineIssueText(item.Label)); width > maxLabel {
			maxLabel = width
		}
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		item.Label = singleLineIssueText(item.Label)
		item.Location = singleLineIssueText(item.Location)
		if item.Location == "" {
			lines = append(lines, item.Label)
			continue
		}
		lines = append(lines, fmt.Sprintf("%-*s  %s", maxLabel, item.Label, item.Location))
	}
	return lines
}

func singleLineIssueText(text string) string {
	return strings.NewReplacer("\r", "\\r", "\n", "\\n", "\t", "\\t").Replace(text)
}
