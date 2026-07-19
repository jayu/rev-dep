package cli

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/config"
)

// resolveHint builds `rev-dep resolve` example commands scoped to a single rule. It is shared by the
// restricted-imports and restricted-importers hints: it captures the rule cwd, a path relativizer,
// and the rule-level flags (follow-monorepo-packages / process-ignored-files) that apply to every
// example, leaving only the per-violation specifics to each caller.
type resolveHint struct {
	ruleCwdArg  string
	absRulePath string
	ruleFlags   []string
}

const resolveHintHeader = "    Hint: trace resolution paths with `rev-dep resolve` from this rule cwd.\n"

func newResolveHint(ruleResult config.RuleResult, cwd string) resolveHint {
	absRulePath := filepath.Clean(filepath.Join(cwd, ruleResult.RulePath))
	ruleCwdArg, err := filepath.Rel(cwd, absRulePath)
	if err != nil || ruleCwdArg == "" {
		ruleCwdArg = ruleResult.RulePath
	}
	ruleCwdArg = filepath.ToSlash(ruleCwdArg)

	ruleFlags := []string{}
	if ruleResult.RestrictedImportsFollowMonorepoPackages.ShouldFollowAll() {
		ruleFlags = append(ruleFlags, "--follow-monorepo-packages")
	} else if len(ruleResult.RestrictedImportsFollowMonorepoPackages.Packages) > 0 {
		pkgs := make([]string, 0, len(ruleResult.RestrictedImportsFollowMonorepoPackages.Packages))
		for pkg := range ruleResult.RestrictedImportsFollowMonorepoPackages.Packages {
			pkgs = append(pkgs, pkg)
		}
		slices.Sort(pkgs)
		ruleFlags = append(ruleFlags, fmt.Sprintf("--follow-monorepo-packages \"%s\"", strings.Join(pkgs, ",")))
	}
	if len(ruleResult.ProcessIgnoredFiles) > 0 {
		patterns := append([]string(nil), ruleResult.ProcessIgnoredFiles...)
		slices.Sort(patterns)
		for _, pattern := range patterns {
			ruleFlags = append(ruleFlags, fmt.Sprintf("--process-ignored-files %q", pattern))
		}
	}

	return resolveHint{ruleCwdArg: ruleCwdArg, absRulePath: absRulePath, ruleFlags: ruleFlags}
}

// relToRule renders an absolute path relative to the rule's cwd (forward slashes).
func (h resolveHint) relToRule(absPath string) string {
	if absPath == "" {
		return absPath
	}
	rel, err := filepath.Rel(h.absRulePath, absPath)
	if err != nil {
		return filepath.ToSlash(absPath)
	}
	return filepath.ToSlash(rel)
}

// extraFlagsPart joins per-violation flags (first) with the rule-level flags into a trailing
// command-line fragment (leading space), or "" when there are none.
func (h resolveHint) extraFlagsPart(violationFlags []string) string {
	flags := append(append([]string(nil), violationFlags...), h.ruleFlags...)
	if len(flags) == 0 {
		return ""
	}
	return " " + strings.Join(flags, " ")
}

// fileExample formats a `rev-dep resolve --file ... --entry-points ... --cwd ...` example line.
func (h resolveHint) fileExample(file, entryPoint string, violationFlags []string) string {
	return fmt.Sprintf("    Example: `rev-dep resolve --file \"%s\" --entry-points \"%s\" --cwd \"%s\"%s`\n",
		h.relToRule(file), h.relToRule(entryPoint), h.ruleCwdArg, h.extraFlagsPart(violationFlags))
}

// moduleExample formats a `rev-dep resolve --module ... --entry-points ... --cwd ...` example line.
func (h resolveHint) moduleExample(module, entryPoint string, violationFlags []string) string {
	return fmt.Sprintf("    Example: `rev-dep resolve --module %s --entry-points \"%s\" --cwd \"%s\"%s`\n",
		module, h.relToRule(entryPoint), h.ruleCwdArg, h.extraFlagsPart(violationFlags))
}

func printRestrictedImportsResolveHint(ruleResult config.RuleResult, cwd string) {
	hint := newResolveHint(ruleResult, cwd)

	violationFlags := func(v *checks.RestrictedImportViolation) []string {
		flags := []string{}
		if v == nil {
			return flags
		}
		if v.IgnoreType {
			flags = append(flags, "--ignore-type-imports")
		}
		for _, pattern := range v.GraphExclude {
			flags = append(flags, fmt.Sprintf("--graph-exclude %q", pattern))
		}
		return flags
	}

	var sampleFileViolation *checks.RestrictedImportViolation
	var sampleModuleViolation *checks.RestrictedImportViolation
	for i := range ruleResult.RestrictedImportsViolations {
		v := &ruleResult.RestrictedImportsViolations[i]
		if sampleFileViolation == nil && v.ViolationType == "file" && v.DeniedFile != "" {
			sampleFileViolation = v
		}
		if sampleModuleViolation == nil && v.ViolationType == "module" {
			sampleModuleViolation = v
		}
		if sampleFileViolation != nil && sampleModuleViolation != nil {
			break
		}
	}

	fmt.Print(resolveHintHeader)
	if sampleFileViolation != nil {
		fmt.Print(hint.fileExample(sampleFileViolation.DeniedFile, sampleFileViolation.EntryPoint, violationFlags(sampleFileViolation)))
	}
	if sampleModuleViolation != nil {
		moduleArg := sampleModuleViolation.ImportRequest
		if moduleArg == "" {
			moduleArg = sampleModuleViolation.DeniedModule
		}
		fmt.Print(hint.moduleExample(moduleArg, sampleModuleViolation.EntryPoint, violationFlags(sampleModuleViolation)))
	}
}

// printRestrictedImportersResolveHint mirrors the restricted-imports hint: a restricted-importers
// violation is an entry point that reaches a restricted file or module, so the trace has the same
// shape (resolve --file <file> / --module <module> --entry-points <entryPoint>).
func printRestrictedImportersResolveHint(ruleResult config.RuleResult, cwd string) {
	if len(ruleResult.RestrictedImportersViolations) == 0 {
		return
	}
	hint := newResolveHint(ruleResult, cwd)

	var sampleFile, sampleModule *checks.RestrictedImporterViolation
	for i := range ruleResult.RestrictedImportersViolations {
		v := &ruleResult.RestrictedImportersViolations[i]
		if sampleFile == nil && v.File != "" {
			sampleFile = v
		}
		if sampleModule == nil && v.Module != "" {
			sampleModule = v
		}
		if sampleFile != nil && sampleModule != nil {
			break
		}
	}

	fmt.Print(resolveHintHeader)
	if sampleFile != nil {
		fmt.Print(hint.fileExample(sampleFile.File, sampleFile.EntryPoint, nil))
	}
	if sampleModule != nil {
		fmt.Print(hint.moduleExample(sampleModule.Module, sampleModule.EntryPoint, nil))
	}
}
