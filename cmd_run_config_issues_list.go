package main

import (
	"fmt"
	"os"
	"strings"
)

func runConfigWithIssuesListOutput(config RevDepConfig, cwd string, packageJsonPath string, tsconfigJsonPath string, runConfigFix bool) error {
	if err := filterRunConfigRules(&config, runConfigRules); err != nil {
		return err
	}

	result, err := ProcessConfig(&config, cwd, packageJsonPath, tsconfigJsonPath, runConfigFix, true)
	if err != nil {
		return fmt.Errorf("Error processing config: %v", err)
	}

	locator := newFileLocationResolver(cwd, result.FullTree)
	rules := make([]jsonRuleResult, 0, len(result.RuleResults))
	for _, ruleResult := range result.RuleResults {
		rules = append(rules, buildJSONRuleResult(ruleResult, cwd, locator))
	}

	output := formatIssuesListOutput(rules)
	if output != "" {
		fmt.Print(output)
	}

	if result.HasFailures {
		os.Exit(1)
	}

	return nil
}

type issuesListGroup struct {
	Title string
	Items []issuesListItem
}

type issuesListItem struct {
	Label    string
	Location string
}

func formatIssuesListOutput(rules []jsonRuleResult) string {
	groups := buildIssuesListGroups(rules)
	if len(groups) == 0 {
		return "No issues found\n"
	}

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

func buildIssuesListGroups(rules []jsonRuleResult) []issuesListGroup {
	byType := map[string][]issuesListItem{}

	add := func(title string, label string, location string) {
		if label == "" {
			return
		}
		byType[title] = append(byType[title], issuesListItem{
			Label:    label,
			Location: location,
		})
	}

	for _, rule := range rules {
		if rule.Checks.CircularDependencies != nil {
			for _, issue := range rule.Checks.CircularDependencies.Issues {
				if v, ok := issue.(jsonCircularDependencyIssue); ok {
					add("Circular dependencies", strings.Join(v.Cycle, " -> "), "")
				}
			}
		}
		if rule.Checks.OrphanFiles != nil {
			for _, issue := range rule.Checks.OrphanFiles.Issues {
				if v, ok := issue.(jsonOrphanFileIssue); ok {
					add("Orphan files", v.FilePath, "")
				}
			}
		}
		if rule.Checks.ModuleBoundaries != nil {
			for _, issue := range rule.Checks.ModuleBoundaries.Issues {
				if v, ok := issue.(jsonModuleBoundaryIssue); ok {
					label := v.ImportPath
					if label == "" {
						label = v.RuleName
					}
					add("Module boundaries", label, formatIssueLocationWithFields(v.FilePath, v.jsonLocationFields))
				}
			}
		}
		if rule.Checks.UnusedNodeModules != nil {
			for _, issue := range rule.Checks.UnusedNodeModules.Issues {
				if v, ok := issue.(jsonUnusedNodeModuleIssue); ok {
					add("Unused dependencies", v.ModuleName, formatIssueLocationWithFields(v.PackageJsonPath, v.jsonLocationFields))
				}
			}
		}
		if rule.Checks.MissingNodeModules != nil {
			for _, issue := range rule.Checks.MissingNodeModules.Issues {
				if v, ok := issue.(jsonMissingNodeModuleIssue); ok {
					if len(v.Locations) > 0 {
						for _, loc := range v.Locations {
							add("Unlisted dependencies", v.ModuleName, formatIssueLocation(loc.FilePath, loc.StartLine, loc.StartCol))
						}
					} else if len(v.ImportedFrom) > 0 {
						for _, filePath := range v.ImportedFrom {
							add("Unlisted dependencies", v.ModuleName, formatIssueLocation(filePath, 0, 0))
						}
					} else {
						add("Unlisted dependencies", v.ModuleName, formatIssueLocation("unknown", 0, 0))
					}
				}
			}
		}
		if rule.Checks.ImportConventions != nil {
			for _, issue := range rule.Checks.ImportConventions.Issues {
				if v, ok := issue.(jsonImportConventionIssue); ok {
					add("Import conventions", v.ImportRequest, formatIssueLocationWithFields(v.FilePath, v.jsonLocationFields))
				}
			}
		}
		if rule.Checks.UnresolvedImports != nil {
			for _, issue := range rule.Checks.UnresolvedImports.Issues {
				if v, ok := issue.(jsonUnresolvedImportIssue); ok {
					add("Unresolved imports", v.Request, formatIssueLocationWithFields(v.FilePath, v.jsonLocationFields))
				}
			}
		}
		if rule.Checks.UnusedExports != nil {
			for _, issue := range rule.Checks.UnusedExports.Issues {
				if v, ok := issue.(jsonUnusedExportIssue); ok {
					add("Unused exports", v.ExportName, formatIssueLocationWithFields(v.FilePath, v.jsonLocationFields))
				}
			}
		}
		if rule.Checks.RestrictedDevDependenciesUsage != nil {
			for _, issue := range rule.Checks.RestrictedDevDependenciesUsage.Issues {
				if v, ok := issue.(jsonRestrictedDevDepsIssue); ok {
					add("Dev deps usage on prod", v.DevDependency, formatIssueLocationWithFields(v.FilePath, v.jsonLocationFields))
				}
			}
		}
		if rule.Checks.RestrictedImports != nil {
			for _, issue := range rule.Checks.RestrictedImports.Issues {
				if v, ok := issue.(jsonRestrictedImportIssue); ok {
					label := v.ImportRequest
					if label == "" {
						if v.DeniedModule != "" {
							label = v.DeniedModule
						} else {
							label = v.DeniedFile
						}
					}
					add("Restricted imports", label, formatIssueLocationWithFields(v.ImporterFile, v.jsonLocationFields))
				}
			}
		}
	}

	order := []string{
		"Circular dependencies",
		"Orphan files",
		"Module boundaries",
		"Unused dependencies",
		"Unlisted dependencies",
		"Import conventions",
		"Unresolved imports",
		"Unused exports",
		"Dev deps usage on prod",
		"Restricted imports",
	}

	groups := make([]issuesListGroup, 0, len(order))
	for _, title := range order {
		if items, ok := byType[title]; ok && len(items) > 0 {
			groups = append(groups, issuesListGroup{Title: title, Items: items})
		}
	}

	return groups
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
		if len(item.Label) > maxLabel {
			maxLabel = len(item.Label)
		}
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		if item.Location == "" {
			lines = append(lines, item.Label)
			continue
		}
		lines = append(lines, fmt.Sprintf("%-*s  %s", maxLabel, item.Label, item.Location))
	}
	return lines
}
