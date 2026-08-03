package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"rev-dep-go/internal/config"
	"rev-dep-go/internal/dupcode"
)

// ---------------- JSON output types ----------------

type jsonOutput struct {
	Version     string           `json:"version"`
	HasFailures bool             `json:"hasFailures"`
	Rules       []jsonRuleResult `json:"workspaces"`
	FixSummary  jsonFixSummary   `json:"fixSummary"`
}

type jsonRuleResult struct {
	Path      string     `json:"path"`
	FileCount int        `json:"fileCount"`
	Checks    jsonChecks `json:"checks"`
}

type jsonChecks struct {
	CircularDependencies           *jsonCheckResult `json:"circularDependencies,omitempty"`
	OrphanFiles                    *jsonCheckResult `json:"orphanFiles,omitempty"`
	ModuleBoundaries               *jsonCheckResult `json:"moduleBoundaries,omitempty"`
	UnusedNodeModules              *jsonCheckResult `json:"unusedNodeModules,omitempty"`
	MissingNodeModules             *jsonCheckResult `json:"missingNodeModules,omitempty"`
	ImportConventions              *jsonCheckResult `json:"importConventions,omitempty"`
	UnresolvedImports              *jsonCheckResult `json:"unresolvedImports,omitempty"`
	UnusedExports                  *jsonCheckResult `json:"unusedExports,omitempty"`
	RestrictedDevDependenciesUsage *jsonCheckResult `json:"restrictedDevDependenciesUsage,omitempty"`
	RestrictedImports              *jsonCheckResult `json:"restrictedImports,omitempty"`
	RestrictedImporters            *jsonCheckResult `json:"restrictedImporters,omitempty"`
	RestrictedDirectImporters      *jsonCheckResult `json:"restrictedDirectImporters,omitempty"`

	// DuplicatedCode is an array because a workspace may configure several detections,
	// each with its own blinding and its own snapshot. It does not use jsonCheckResult:
	// see jsonDuplicatedCodeResult for why it reports counts rather than issues.
	DuplicatedCode []jsonDuplicatedCodeResult `json:"duplicatedCode,omitempty"`
}

type jsonCheckResult struct {
	Status string        `json:"status"`
	Issues []interface{} `json:"issues"`
}

// jsonDuplicatedCodeResult is one duplicated-code detection.
//
// Unlike every other check it carries counts rather than an issues array, which is the same
// choice the console output makes and for the same reason: a real project has thousands of
// duplicated blocks, listing them would dwarf the rest of the report, and the number is what
// the rule actually acts on. Command reproduces the detection, so a consumer that does want
// the places has an exact way to get them - with `--format json` for the full detail.
type jsonDuplicatedCodeResult struct {
	Status string `json:"status"`
	// Blinding names what the comparison ignored, so the counts are interpretable and two
	// detections on the same workspace can be told apart.
	Blinding string `json:"blinding"`
	// Snippets is the number of distinct duplicated patterns; Occurrences is how many
	// places they appear in; Files is how many files hold at least one.
	Snippets    int `json:"snippets"`
	Occurrences int `json:"occurrences"`
	Files       int `json:"files"`
	// Scanned and Ignored describe the file set the counts came from, so a disagreement
	// with a direct `rev-dep duplicated-code` run can be traced to the scope.
	Scanned int `json:"scanned"`
	Ignored int `json:"ignored"`
	// ConfigIndex ties this outcome to the settings that produced it. The settings themselves are
	// not copied here: the consumer already has the config, and a second copy could only drift.
	ConfigIndex int `json:"configIndex"`
	// Command is NOT published: a shell string is no use to a consumer that wants to act, and no
	// other check here carries one. It stays on the struct because the issues list renders from
	// these values.
	Command string `json:"-"`
	// Error separates "nothing found" from "the check never ran" - both otherwise show a count of
	// zero beside status "fail".
	Error string `json:"error,omitempty"`
	// Snapshot is present only when the detection is baselined. With a baseline the totals
	// above are still reported, but the status is decided by the change.
	Snapshot *jsonDuplicatedCodeSnapshot `json:"snapshot,omitempty"`
}

// jsonDuplicatedCodeSnapshot is the baseline comparison, counted the same way: how many
// patterns changed in each direction, not which ones.
type jsonDuplicatedCodeSnapshot struct {
	Path string `json:"path"`
	// Written reports that this run rewrote the baseline instead of checking against it,
	// which is why nothing failed.
	Written bool `json:"written"`
	// Missing reports a configured baseline that does not exist - a broken setup rather
	// than a clean one, and the reason a status can be "fail" with no changes listed.
	Missing bool `json:"missing"`

	// Embedded rather than restated, so they cannot fall behind dupcode. Restating them is how a
	// category went missing: five of six were listed.
	dupcode.DeltaCounts

	// ParameterChanges names settings that differ from the ones the baseline was taken
	// under. They do not affect the comparison, but they explain a surprising delta.
	ParameterChanges []string `json:"parameterChanges,omitempty"`
	// CanonicalFormChanged reports that the baseline's hashes came from a different
	// normaliser revision, so part of the delta may be re-canonicalisation rather than
	// real change.
	CanonicalFormChanged bool `json:"canonicalFormChanged"`
}

type jsonLocationFields struct {
	StartLine *int `json:"startLine,omitempty"`
	StartCol  *int `json:"startCol,omitempty"`
	EndLine   *int `json:"endLine,omitempty"`
	EndCol    *int `json:"endCol,omitempty"`
}

type jsonLocation struct {
	FilePath  string `json:"filePath"`
	StartLine int    `json:"startLine"`
	StartCol  int    `json:"startCol"`
	EndLine   int    `json:"endLine"`
	EndCol    int    `json:"endCol"`
}

type jsonFixSummary struct {
	FixedFilesCount        int `json:"fixedFilesCount"`
	FixedImportsCount      int `json:"fixedImportsCount"`
	DeletedFilesCount      int `json:"deletedFilesCount"`
	FixableIssuesCount     int `json:"fixableIssuesCount"`
	UnfixableAliasingCount int `json:"unfixableAliasingCount"`
}

type jsonCircularDependencyIssue struct {
	Cycle []string `json:"cycle"`
}

type jsonOrphanFileIssue struct {
	FilePath string `json:"filePath"`
}

type jsonModuleBoundaryIssue struct {
	RuleName      string `json:"ruleName"`
	FilePath      string `json:"filePath"`
	ImportPath    string `json:"importPath"`
	ViolationType string `json:"violationType"`
	jsonLocationFields
}

type jsonUnusedNodeModuleIssue struct {
	ModuleName      string `json:"moduleName"`
	PackageJsonPath string `json:"filePath"`
	jsonLocationFields
}

type jsonMissingNodeModuleIssue struct {
	ModuleName   string         `json:"moduleName"`
	ImportedFrom []string       `json:"importedFrom"`
	Locations    []jsonLocation `json:"locations,omitempty"`
}

type jsonImportConventionIssue struct {
	FilePath      string `json:"filePath"`
	ImportRequest string `json:"importRequest"`
	ViolationType string `json:"violationType"`
	jsonLocationFields
}

type jsonUnresolvedImportIssue struct {
	FilePath string `json:"filePath"`
	Request  string `json:"request"`
	jsonLocationFields
}

type jsonUnusedExportIssue struct {
	FilePath   string `json:"filePath"`
	ExportName string `json:"exportName"`
	IsType     bool   `json:"isType"`
	jsonLocationFields
}

type jsonRestrictedDevDepsIssue struct {
	DevDependency string `json:"devDependency"`
	FilePath      string `json:"filePath"`
	EntryPoint    string `json:"entryPoint"`
	jsonLocationFields
}

type jsonRestrictedImportIssue struct {
	ViolationType string `json:"violationType"`
	ImporterFile  string `json:"importerFile"`
	EntryPoint    string `json:"entryPoint"`
	DeniedFile    string `json:"deniedFile,omitempty"`
	DeniedModule  string `json:"deniedModule,omitempty"`
	ImportRequest string `json:"importRequest,omitempty"`
	jsonLocationFields
}

type jsonRestrictedImporterIssue struct {
	EntryPoint string `json:"entryPoint"`
	File       string `json:"file,omitempty"`
	Module     string `json:"module,omitempty"`
}

type jsonRestrictedDirectImporterIssue struct {
	ViolationType string `json:"violationType"`
	ImporterFile  string `json:"importerFile"`
	File          string `json:"file,omitempty"`
	Module        string `json:"module,omitempty"`
	ImportRequest string `json:"importRequest,omitempty"`
}

// ---------------- JSON output logic ----------------

func runConfigWithJSONOutput(cfg config.RevDepConfig, cwd string, runConfigFix bool, runConfigRecheck bool) error {
	output := jsonOutput{
		Version: "2.0",
		Rules:   []jsonRuleResult{},
	}

	if err := filterRunConfigRules(&cfg, runConfigRules); err != nil {
		return err
	}

	result, err := processConfigRun(&cfg, cwd, runConfigFix, runConfigRecheck, true)
	if err != nil {
		return fmt.Errorf("Error processing config: %v", err)
	}

	if result.HasFailures {
		output.HasFailures = true
	}

	output.FixSummary.FixedFilesCount += result.FixedFilesCount
	output.FixSummary.FixedImportsCount += result.FixedImportsCount
	output.FixSummary.DeletedFilesCount += result.DeletedFilesCount
	output.FixSummary.FixableIssuesCount += result.FixableIssuesCount
	output.FixSummary.UnfixableAliasingCount += result.UnfixableAliasingCount

	locator := newFileLocationResolver(cwd, result.FullTree)
	for _, ruleResult := range result.RuleResults {
		output.Rules = append(output.Rules, buildJSONRuleResult(ruleResult, cwd, locator))
	}

	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON output: %v", err)
	}

	if output.HasFailures {
		os.Exit(1)
	}

	return nil
}

func buildJSONRuleResult(ruleResult config.RuleResult, cwd string, locator *fileLocationResolver) jsonRuleResult {
	relPath := func(absolutePath string) string {
		if absolutePath == "" {
			return absolutePath
		}
		rel, err := filepath.Rel(cwd, absolutePath)
		if err != nil {
			return absolutePath
		}
		return filepath.ToSlash(rel)
	}

	jr := jsonRuleResult{
		Path:      ruleResult.RulePath,
		FileCount: ruleResult.FileCount,
	}

	for _, check := range ruleResult.EnabledChecks {
		switch check {
		case "circular-imports":
			cr := &jsonCheckResult{Issues: []interface{}{}}
			if len(ruleResult.CircularDependencies) > 0 {
				cr.Status = "fail"
				for _, cycle := range ruleResult.CircularDependencies {
					relCycle := make([]string, len(cycle))
					for i, p := range cycle {
						relCycle[i] = relPath(p)
					}
					cr.Issues = append(cr.Issues, jsonCircularDependencyIssue{Cycle: relCycle})
				}
			} else {
				cr.Status = "pass"
			}
			jr.Checks.CircularDependencies = cr

		case "orphan-files":
			cr := &jsonCheckResult{Issues: []interface{}{}}
			if len(ruleResult.OrphanFiles) > 0 {
				cr.Status = "fail"
				for _, file := range ruleResult.OrphanFiles {
					cr.Issues = append(cr.Issues, jsonOrphanFileIssue{FilePath: relPath(file)})
				}
			} else {
				cr.Status = "pass"
			}
			jr.Checks.OrphanFiles = cr

		case "module-boundaries":
			cr := &jsonCheckResult{Issues: []interface{}{}}
			if len(ruleResult.ModuleBoundaryViolations) > 0 {
				cr.Status = "fail"
				for _, v := range ruleResult.ModuleBoundaryViolations {
					issue := jsonModuleBoundaryIssue{
						RuleName:      v.RuleName,
						FilePath:      relPath(v.FilePath),
						ImportPath:    relPath(v.ImportPath),
						ViolationType: v.ViolationType,
					}
					if locator != nil && v.ImportRequest != "" {
						issue.jsonLocationFields = locator.locationForRequest(v.FilePath, v.ImportRequest)
					}
					cr.Issues = append(cr.Issues, issue)
				}
			} else {
				cr.Status = "pass"
			}
			jr.Checks.ModuleBoundaries = cr

		case "unused-node-modules":
			cr := &jsonCheckResult{Issues: []interface{}{}}
			if len(ruleResult.UnusedNodeModules) > 0 {
				cr.Status = "fail"
				for _, module := range ruleResult.UnusedNodeModules {
					loc := jsonLocationFields{}
					if locator != nil {
						loc = locator.locationForPackageJsonDependency(module.PackageJsonPath, module.ModuleName)
					}
					cr.Issues = append(cr.Issues, jsonUnusedNodeModuleIssue{
						ModuleName:         module.ModuleName,
						PackageJsonPath:    relPath(module.PackageJsonPath),
						jsonLocationFields: loc,
					})
				}
			} else {
				cr.Status = "pass"
			}
			jr.Checks.UnusedNodeModules = cr

		case "missing-node-modules":
			cr := &jsonCheckResult{Issues: []interface{}{}}
			if len(ruleResult.MissingNodeModules) > 0 {
				cr.Status = "fail"
				for _, m := range ruleResult.MissingNodeModules {
					importedFrom := make([]string, len(m.ImportedFrom))
					for i, p := range m.ImportedFrom {
						importedFrom[i] = relPath(p)
					}
					issue := jsonMissingNodeModuleIssue{
						ModuleName:   m.ModuleName,
						ImportedFrom: importedFrom,
					}
					if locator != nil {
						for _, filePath := range m.ImportedFrom {
							fields := locator.locationForModuleName(filePath, m.ModuleName)
							if fields.StartLine != nil && fields.StartCol != nil && fields.EndLine != nil && fields.EndCol != nil {
								issue.Locations = append(issue.Locations, jsonLocation{
									FilePath:  relPath(filePath),
									StartLine: *fields.StartLine,
									StartCol:  *fields.StartCol,
									EndLine:   *fields.EndLine,
									EndCol:    *fields.EndCol,
								})
							}
						}
					}
					cr.Issues = append(cr.Issues, issue)
				}
			} else {
				cr.Status = "pass"
			}
			jr.Checks.MissingNodeModules = cr

		case "import-conventions":
			cr := &jsonCheckResult{Issues: []interface{}{}}
			if len(ruleResult.ImportConventionViolations) > 0 {
				cr.Status = "fail"
				for _, v := range ruleResult.ImportConventionViolations {
					issue := jsonImportConventionIssue{
						FilePath:      relPath(v.FilePath),
						ImportRequest: v.ImportRequest,
						ViolationType: v.ViolationType,
					}
					if locator != nil {
						issue.jsonLocationFields = locator.locationForRequest(v.FilePath, v.ImportRequest)
					}
					cr.Issues = append(cr.Issues, issue)
				}
			} else {
				cr.Status = "pass"
			}
			jr.Checks.ImportConventions = cr

		case "unresolved-imports":
			cr := &jsonCheckResult{Issues: []interface{}{}}
			if len(ruleResult.UnresolvedImports) > 0 {
				cr.Status = "fail"
				for _, u := range ruleResult.UnresolvedImports {
					issue := jsonUnresolvedImportIssue{
						FilePath: relPath(u.FilePath),
						Request:  u.Request,
					}
					if locator != nil {
						issue.jsonLocationFields = locator.locationForRequest(u.FilePath, u.Request)
					}
					cr.Issues = append(cr.Issues, issue)
				}
			} else {
				cr.Status = "pass"
			}
			jr.Checks.UnresolvedImports = cr

		case "unused-exports":
			cr := &jsonCheckResult{Issues: []interface{}{}}
			if len(ruleResult.UnusedExports) > 0 {
				cr.Status = "fail"
				for _, ue := range ruleResult.UnusedExports {
					issue := jsonUnusedExportIssue{
						FilePath:   relPath(ue.FilePath),
						ExportName: ue.ExportName,
						IsType:     ue.IsType,
					}
					if locator != nil {
						issue.jsonLocationFields = locator.locationForExport(ue.FilePath, ue.ExportName)
					}
					cr.Issues = append(cr.Issues, issue)
				}
			} else {
				cr.Status = "pass"
			}
			jr.Checks.UnusedExports = cr

		case "dev-deps-usage-on-prod":
			cr := &jsonCheckResult{Issues: []interface{}{}}
			if len(ruleResult.RestrictedDevDependenciesUsageViolations) > 0 {
				cr.Status = "fail"
				for _, v := range ruleResult.RestrictedDevDependenciesUsageViolations {
					issue := jsonRestrictedDevDepsIssue{
						DevDependency: v.DevDependency,
						FilePath:      relPath(v.FilePath),
						EntryPoint:    relPath(v.EntryPoint),
					}
					if locator != nil {
						issue.jsonLocationFields = locator.locationForModuleName(v.FilePath, v.DevDependency)
					}
					cr.Issues = append(cr.Issues, issue)
				}
			} else {
				cr.Status = "pass"
			}
			jr.Checks.RestrictedDevDependenciesUsage = cr

		case "restricted-imports":
			cr := &jsonCheckResult{Issues: []interface{}{}}
			if len(ruleResult.RestrictedImportsViolations) > 0 {
				cr.Status = "fail"
				for _, v := range ruleResult.RestrictedImportsViolations {
					issue := jsonRestrictedImportIssue{
						ViolationType: v.ViolationType,
						ImporterFile:  relPath(v.ImporterFile),
						EntryPoint:    relPath(v.EntryPoint),
						DeniedFile:    relPath(v.DeniedFile),
						DeniedModule:  v.DeniedModule,
						ImportRequest: v.ImportRequest,
					}
					if locator != nil && v.ImportRequest != "" {
						issue.jsonLocationFields = locator.locationForRequest(v.ImporterFile, v.ImportRequest)
					}
					cr.Issues = append(cr.Issues, issue)
				}
			} else {
				cr.Status = "pass"
			}
			jr.Checks.RestrictedImports = cr

		case "restricted-importers":
			cr := &jsonCheckResult{Issues: []interface{}{}}
			if len(ruleResult.RestrictedImportersViolations) > 0 {
				cr.Status = "fail"
				for _, v := range ruleResult.RestrictedImportersViolations {
					cr.Issues = append(cr.Issues, jsonRestrictedImporterIssue{
						EntryPoint: relPath(v.EntryPoint),
						File:       relPath(v.File),
						Module:     v.Module,
					})
				}
			} else {
				cr.Status = "pass"
			}
			jr.Checks.RestrictedImporters = cr

		case "restricted-direct-importers":
			cr := &jsonCheckResult{Issues: []interface{}{}}
			if len(ruleResult.RestrictedDirectImportersViolations) > 0 {
				cr.Status = "fail"
				for _, v := range ruleResult.RestrictedDirectImportersViolations {
					cr.Issues = append(cr.Issues, jsonRestrictedDirectImporterIssue{
						ViolationType: v.ViolationType,
						ImporterFile:  relPath(v.ImporterFile),
						File:          relPath(v.File),
						Module:        v.Module,
						ImportRequest: v.ImportRequest,
					})
				}
			} else {
				cr.Status = "pass"
			}
			jr.Checks.RestrictedDirectImporters = cr

		case "duplicated-code":
			for _, d := range ruleResult.DuplicatedCode {
				jr.Checks.DuplicatedCode = append(jr.Checks.DuplicatedCode,
					buildJSONDuplicatedCodeResult(d))
			}
		}
	}

	return jr
}

// buildJSONDuplicatedCodeResult converts one detection. The status is taken from the same
// predicate the console and the exit code use, so the three can never disagree about
// whether a workspace passed.
func buildJSONDuplicatedCodeResult(d config.DuplicatedCodeRuleResult) jsonDuplicatedCodeResult {
	dup := d.Result

	out := jsonDuplicatedCodeResult{
		Status:      "pass",
		Blinding:    dup.Blinding,
		Snippets:    dup.Snippets,
		Occurrences: dup.Occurrences,
		Files:       dup.Files,
		Scanned:     dup.Scanned,
		Ignored:     dup.Ignored,
		ConfigIndex: d.ConfigIndex,
		Command:     d.Command,
	}
	if d.Failed() {
		out.Status = "fail"
	}
	if d.Err != nil {
		out.Error = d.Err.Error()
	}

	if dup.SnapshotPath != "" {
		snap := &jsonDuplicatedCodeSnapshot{
			Path:        dup.SnapshotPath,
			Written:     dup.SnapshotWritten,
			Missing:     dup.SnapshotMissing,
			DeltaCounts: dupcode.CountsOf(dup.Delta),
		}
		if delta := dup.Delta; delta != nil {
			snap.ParameterChanges = delta.ParameterChanges
			snap.CanonicalFormChanged = delta.CanonicalFormChanged
		}
		out.Snapshot = snap
	}

	return out
}
