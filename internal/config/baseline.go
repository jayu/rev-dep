package config

import (
	"fmt"
	"path/filepath"
	"slices"

	"rev-dep-go/internal/baseline"
	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/node"
	"rev-dep-go/internal/pathutil"
)

// BaselineSnapshotFile resolves the findings snapshot relative to the config.
func BaselineSnapshotFile(cfg *RevDepConfig, cwd string) (string, error) {
	root := cwd
	if cfg.SourcePath != "" {
		root = filepath.Dir(cfg.SourcePath)
	}
	filename := resolveSnapshotPath(cfg.BaselineSnapshotPath, root)
	if cfg.SourcePath != "" && filepath.Clean(filename) == filepath.Clean(cfg.SourcePath) {
		return "", fmt.Errorf("baselineSnapshotPath must not overwrite the config file")
	}
	for _, rule := range cfg.Rules {
		for _, detection := range rule.DuplicatedCodeDetections {
			if detection.IsEnabled() && detection.SnapshotPath != "" && filepath.Clean(filename) == filepath.Clean(resolveSnapshotPath(detection.SnapshotPath, pathutil.JoinWithCwd(cwd, rule.Path))) {
				return "", fmt.Errorf("baselineSnapshotPath and duplicated-code snapshotPath must use different files")
			}
		}
	}
	return filename, nil
}

// ApplyFindingsBaseline is a post-processing layer: capture a sorted semantic
// set from completed checks, then retain only new issues in the result. Obsolete
// issues live in Baseline.Obsolete because they have no current diagnostic row.
func ApplyFindingsBaseline(cfg *RevDepConfig, cwd string, result *ConfigProcessingResult, previous *baseline.Snapshot, update bool, selected []string) error {
	filename, err := BaselineSnapshotFile(cfg, cwd)
	if err != nil {
		return err
	}
	root := cwd
	if cfg.SourcePath != "" {
		root = filepath.Dir(cfg.SourcePath)
	}
	relative := func(value string) string {
		if value == "" {
			return ""
		}
		value = pathutil.DenormalizePathForOS(value)
		if !filepath.IsAbs(value) {
			value = filepath.Join(root, value)
		}
		rel, err := filepath.Rel(root, value)
		if err != nil {
			return filepath.ToSlash(value)
		} // canonical validation rejects absolute fallbacks
		return filepath.ToSlash(rel)
	}
	current := baseline.New()
	for i := range result.RuleResults {
		rule := &result.RuleResults[i]
		for _, detector := range rule.EnabledChecks {
			if detector != "duplicated-code" && !slices.Contains(baseline.Detectors(), detector) {
				return fmt.Errorf("baseline identity adapter missing for %s", detector)
			}
		}
		workspace := relative(rule.RulePath)
		if err := rewriteBaselineFindings(rule, relative, func(detector string, issue baseline.Issue) bool {
			current.Add(workspace, detector, issue)
			return true
		}); err != nil {
			return err
		}
	}
	if err := current.Canonicalize(); err != nil {
		return err
	}
	scopes := make([]string, 0, len(selected))
	for _, workspace := range selected {
		scopes = append(scopes, relative(workspace))
	}
	var outcome *baseline.Result
	if update {
		for _, rule := range result.RuleResults {
			if rule.EmptyWorkspaceFailed() {
				return fmt.Errorf("cannot update baseline snapshot: workspace %s matched no files (failOnEmptyWorkspace)", rule.RulePath)
			}
			for _, d := range rule.DuplicatedCode {
				if d.Err != nil {
					return fmt.Errorf("cannot update baseline snapshot: duplicated-code analysis failed: %w", d.Err)
				}
			}
		}
		if len(selected) > 0 {
			return fmt.Errorf("--update-snapshot cannot be combined with --workspaces when baselineSnapshotPath is configured")
		}
		if err := baseline.Save(filename, current); err != nil {
			return fmt.Errorf("write baseline snapshot %s: %w", filename, err)
		}
		outcome = &baseline.Result{Path: cfg.BaselineSnapshotPath, Status: "written", Counts: baseline.Counts{Acknowledged: len(current.Findings(nil))}, New: []baseline.Finding{}, Obsolete: []baseline.Finding{}}
	} else {
		if previous == nil {
			return fmt.Errorf("baseline snapshot was not loaded")
		}
		outcome = baseline.Compare(current, previous, scopes)
		outcome.Path = cfg.BaselineSnapshotPath
	}
	newFindings := map[string]bool{}
	for _, finding := range outcome.New {
		newFindings[baseline.FindingKey(finding)] = true
	}
	for i := range result.RuleResults {
		rule := &result.RuleResults[i]
		workspace := relative(rule.RulePath)
		if err := rewriteBaselineFindings(rule, relative, func(detector string, issue baseline.Issue) bool {
			key := baseline.FindingKey(baseline.Finding{Workspace: workspace, Detector: detector, Issue: issue})
			keep := newFindings[key]
			delete(newFindings, key) // deduplicate diagnostics across repeated workspace entries too
			return keep
		}); err != nil {
			return err
		}
	}
	// Fix suggestions describe remaining violations, not acknowledged rows.
	result.FixableIssuesCount = 0
	for i := range result.RuleResults {
		rule := &result.RuleResults[i]
		rule.OrphanFilesAutofixable = slices.DeleteFunc(rule.OrphanFilesAutofixable, func(file string) bool { return !slices.Contains(rule.OrphanFiles, file) })
		for _, v := range rule.ImportConventionViolations {
			if v.Fix != nil {
				result.FixableIssuesCount++
			}
		}
		for _, v := range rule.UnusedExports {
			if v.Fix != nil {
				result.FixableIssuesCount++
			}
		}
		for _, configured := range cfg.Rules {
			if relative(configured.Path) != relative(rule.RulePath) {
				continue
			}
			orphanAutofix := false
			for _, d := range configured.OrphanFilesDetections {
				if d.IsEnabled() && d.Autofix {
					orphanAutofix = true
				}
			}
			if orphanAutofix {
				result.FixableIssuesCount += len(rule.OrphanFilesAutofixable)
				break
			}
		}
	}
	result.Baseline = outcome
	result.HasFailures = outcome.Status == "different"
	for _, rule := range result.RuleResults {
		result.HasFailures = result.HasFailures || rule.EmptyWorkspaceFailed() || anyDuplicatedCodeFailed(rule)
	}
	return nil
}

type identityKeep func(string, baseline.Issue) bool

// projectAndFilter preserves diagnostic order while filtering and deduplicating
// by semantic identity. Snapshot canonicalization sorts the stored identities.
func projectAndFilter[T any](items []T, detector string, identity func(T) baseline.Issue, keep identityKeep) ([]T, error) {
	out := make([]T, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		issue, err := baseline.CanonicalIssue(detector, identity(item))
		if err != nil {
			return nil, fmt.Errorf("capture %s: %w", detector, err)
		}
		key := baseline.IssueKey(issue)
		if !seen[key] {
			seen[key] = true
			if keep(detector, issue) {
				out = append(out, item)
			}
		}
	}
	return out, nil
}

func rewriteBaselineFindings(r *RuleResult, rel func(string) string, keep identityKeep) error {
	var err error
	// Membership and display cycles must travel together through filtering.
	if len(r.CircularDependencies) != len(r.CircularComponents) {
		return fmt.Errorf("circular-import findings lack full SCC membership")
	}
	type component struct{ cycle, files []string }
	components := make([]component, 0, len(r.CircularComponents))
	for i, files := range r.CircularComponents {
		components = append(components, component{r.CircularDependencies[i], files})
	}
	components, err = projectAndFilter(components, "circular-imports", func(c component) baseline.Issue {
		files := make([]string, len(c.files))
		for i, f := range c.files {
			files[i] = rel(f)
		}
		return baseline.Issue{Files: files}
	}, keep)
	if err != nil {
		return err
	}
	r.CircularDependencies, r.CircularComponents = nil, nil
	for _, c := range components {
		r.CircularDependencies = append(r.CircularDependencies, c.cycle)
		r.CircularComponents = append(r.CircularComponents, c.files)
	}
	r.OrphanFiles, err = projectAndFilter(r.OrphanFiles, "orphan-files", func(f string) baseline.Issue { return baseline.Issue{File: rel(f)} }, keep)
	if err != nil {
		return err
	}
	r.ModuleBoundaryViolations, err = projectAndFilter(r.ModuleBoundaryViolations, "module-boundaries", func(v checks.ModuleBoundaryViolation) baseline.Issue {
		return baseline.Issue{File: rel(v.FilePath), Target: rel(v.ImportPath)}
	}, keep)
	if err != nil {
		return err
	}
	r.UnusedNodeModules, err = projectAndFilter(r.UnusedNodeModules, "unused-node-modules", func(v node.UnusedNodeModuleIssue) baseline.Issue {
		return baseline.Issue{PackageJSON: rel(v.PackageJsonPath), Module: v.ModuleName}
	}, keep)
	if err != nil {
		return err
	}
	// Flatten grouped missing dependencies into one pair per importing file.
	pairs := []node.MissingNodeModuleResult{}
	for _, v := range r.MissingNodeModules {
		if len(v.ImportedFrom) == 0 {
			return fmt.Errorf("missing dependency %s lacks an importing file", v.ModuleName)
		}
		for _, f := range v.ImportedFrom {
			pairs = append(pairs, node.MissingNodeModuleResult{ModuleName: v.ModuleName, ImportedFrom: []string{f}})
		}
	}
	pairs, err = projectAndFilter(pairs, "missing-node-modules", func(v node.MissingNodeModuleResult) baseline.Issue {
		return baseline.Issue{File: rel(v.ImportedFrom[0]), Module: v.ModuleName}
	}, keep)
	if err != nil {
		return err
	}
	grouped := map[string][]string{}
	var names []string
	for _, v := range pairs {
		if _, exists := grouped[v.ModuleName]; !exists {
			names = append(names, v.ModuleName)
		}
		grouped[v.ModuleName] = append(grouped[v.ModuleName], v.ImportedFrom...)
	}
	r.MissingNodeModules = nil
	for _, name := range names {
		r.MissingNodeModules = append(r.MissingNodeModules, node.MissingNodeModuleResult{ModuleName: name, ImportedFrom: grouped[name]})
	}
	r.UnusedExports, err = projectAndFilter(r.UnusedExports, "unused-exports", func(v checks.UnusedExport) baseline.Issue {
		isType := v.IsType
		return baseline.Issue{File: rel(v.FilePath), Export: v.ExportName, IsType: &isType}
	}, keep)
	if err != nil {
		return err
	}
	r.UnresolvedImports, err = projectAndFilter(r.UnresolvedImports, "unresolved-imports", func(v checks.UnresolvedImport) baseline.Issue {
		kind := "literal"
		if v.IsNonLiteral {
			kind = "nonliteral"
		}
		return baseline.Issue{File: rel(v.FilePath), Request: v.Request, Kind: kind}
	}, keep)
	if err != nil {
		return err
	}
	r.ImportConventionViolations, err = projectAndFilter(r.ImportConventionViolations, "import-conventions", func(v checks.ImportConventionViolation) baseline.Issue {
		return baseline.Issue{File: rel(v.FilePath), Request: v.ImportRequest, ViolationType: v.ViolationType}
	}, keep)
	if err != nil {
		return err
	}
	r.RestrictedDevDependenciesUsageViolations, err = projectAndFilter(r.RestrictedDevDependenciesUsageViolations, "dev-deps-usage-on-prod", func(v checks.RestrictedDevDependenciesUsageViolation) baseline.Issue {
		return baseline.Issue{File: rel(v.FilePath), Module: v.DevDependency}
	}, keep)
	if err != nil {
		return err
	}
	r.RestrictedImportsViolations, err = projectAndFilter(r.RestrictedImportsViolations, "restricted-imports", func(v checks.RestrictedImportViolation) baseline.Issue {
		if v.ViolationType == "file" {
			return baseline.Issue{Kind: "file", Target: rel(v.DeniedFile)}
		}
		return baseline.Issue{Kind: v.ViolationType, Request: v.ImportRequest}
	}, keep)
	if err != nil {
		return err
	}
	r.RestrictedImportersViolations, err = projectAndFilter(r.RestrictedImportersViolations, "restricted-importers", func(v checks.RestrictedImporterViolation) baseline.Issue {
		if v.File != "" {
			return baseline.Issue{EntryPoint: rel(v.EntryPoint), Kind: "file", Target: rel(v.File)}
		}
		return baseline.Issue{EntryPoint: rel(v.EntryPoint), Kind: "module", Module: v.Module}
	}, keep)
	if err != nil {
		return err
	}
	r.RestrictedDirectImportersViolations, err = projectAndFilter(r.RestrictedDirectImportersViolations, "restricted-direct-importers", func(v checks.RestrictedDirectImporterViolation) baseline.Issue {
		if v.ViolationType == "file" {
			return baseline.Issue{File: rel(v.ImporterFile), Kind: "file", Target: rel(v.File)}
		}
		return baseline.Issue{File: rel(v.ImporterFile), Kind: v.ViolationType, Request: v.ImportRequest}
	}, keep)
	return err
}
