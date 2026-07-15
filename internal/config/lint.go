package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gobwas/glob"

	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/model"
	"rev-dep-go/internal/module"
	"rev-dep-go/internal/pathutil"
	"rev-dep-go/internal/resolve"
)

// LintConfig ("config lint") reports glob/path patterns declared in a rev-dep config
// that match zero discovered files or modules — "dead patterns" that only bloat the
// config. It is the counterpart of the metadata already gathered by
// findUnmatchedEntryPointPatterns, generalized to every pattern-bearing option.

// PatternKind describes what universe a pattern is matched against.
type PatternKind string

const (
	KindFile   PatternKind = "file"   // matched against discovered files
	KindModule PatternKind = "module" // matched against node module names/requests
	KindMixed  PatternKind = "mixed"  // matched against files OR modules (ignoreMatches)
	KindDir    PatternKind = "dir"    // a directory/rule path that resolves to no files
)

// DeadPattern is a single config pattern that matched nothing.
type DeadPattern struct {
	RuleIndex     int    // index into config.Rules, or -1 for top-level options
	RulePath      string // rule path (empty for top-level)
	DetectorType  string // JSON detector key (e.g. "orphanFilesDetection"); "" for top-level/rule-level
	DetectorIndex int    // index when the detector uses the array form; 0 for single-object form
	BoundaryIndex int    // index into moduleBoundaries; -1 when not applicable
	OptionKey     string // JSON option key (e.g. "validEntryPoints")
	ElementIndex  int    // index within the option array; -1 for scalar options
	Value         string // the dead pattern text
	Kind          PatternKind
	Removable     bool // whether --fix may auto-remove it (load-bearing patterns are report-only)
}

// LintRuleName identifies a selectable lint rule. The linter is a registry of rules so
// more can be added over time; callers pick a subset via LintConfig's `rules` argument.
type LintRuleName string

const (
	// RuleOrphanFileGlobs flags file/path globs (ignore patterns, entry points, graph
	// excludes, rule paths, module-boundary selectors) that match no discovered file.
	// It works from file discovery alone and does NOT parse the dependency tree.
	RuleOrphanFileGlobs LintRuleName = "orphan-file-globs"
	// RuleOrphanModuleGlobs flags node-module name globs (denyModules, include/exclude
	// modules, restricted modules, and the file-or-module ignoreMatches) that match no
	// module. It requires the parsed dependency tree to know which modules are imported.
	RuleOrphanModuleGlobs LintRuleName = "orphan-module-globs"
)

// AllLintRules is the default set run when no selection is given, in output order.
var AllLintRules = []LintRuleName{RuleOrphanFileGlobs, RuleOrphanModuleGlobs}

// ParseLintRules validates a list of rule names (as typed on the CLI) and returns them
// as LintRuleName values. An empty input selects all rules. Unknown names are an error.
func ParseLintRules(names []string) ([]LintRuleName, error) {
	if len(names) == 0 {
		return append([]LintRuleName(nil), AllLintRules...), nil
	}
	valid := make(map[LintRuleName]bool, len(AllLintRules))
	for _, r := range AllLintRules {
		valid[r] = true
	}
	seen := make(map[LintRuleName]bool)
	out := make([]LintRuleName, 0, len(names))
	for _, raw := range names {
		name := LintRuleName(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if !valid[name] {
			return nil, fmt.Errorf("unknown lint rule %q (valid: %s)", raw, joinLintRules(AllLintRules))
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return append([]LintRuleName(nil), AllLintRules...), nil
	}
	return out, nil
}

func joinLintRules(rules []LintRuleName) string {
	parts := make([]string, len(rules))
	for i, r := range rules {
		parts[i] = string(r)
	}
	return strings.Join(parts, ", ")
}

// LintResult is the outcome of a config lint run.
type LintResult struct {
	ConfigFilePath string
	RulesRun       []LintRuleName
	DeadPatterns   []DeadPattern
}

// lintCtx holds the discovered universes for one lint run.
type lintCtx struct {
	cwd            string
	moduleUniverse []string      // populated only when the module rule runs
	doc            *JSONDocument // parsed config file; used to check physical presence
	deads          []DeadPattern
}

// LintConfig analyzes cfg and returns every dead pattern for the selected rules (all
// rules when `rules` is empty). packageJson/tsconfigJson mirror the paths threaded
// through ProcessConfig. The dependency tree is built (parsing every file) ONLY when
// the module rule is selected; the file rule runs from file discovery alone.
func LintConfig(cfg *RevDepConfig, cwd, packageJson, tsconfigJson string, rules []LintRuleName) (*LintResult, error) {
	if len(rules) == 0 {
		rules = AllLintRules
	}
	runFile, runModule := false, false
	for _, r := range rules {
		switch r {
		case RuleOrphanFileGlobs:
			runFile = true
		case RuleOrphanModuleGlobs:
			runModule = true
		}
	}

	configFilePath, _ := FindConfigFile(cwd)

	// Parse the raw config file so the analyzer can verify that a pattern physically
	// exists in the file before reporting it. This filters out values synthesized by
	// ParseConfig's rule-level entry-point inheritance (orphan/unusedExports
	// validEntryPoints and devDeps prodEntryPoints inherit the rule's entry points when
	// not explicitly set) — those are not in the file and must not be linted.
	var doc *JSONDocument
	if configFilePath != "" {
		if raw, readErr := os.ReadFile(configFilePath); readErr == nil {
			doc, _ = ParseJSONC(raw)
		}
	}

	// File discovery (respecting gitignore + the config's ignoreFiles) is needed by both
	// rules: the file rule matches globs against it, the module rule needs per-rule files
	// for the file side of ignoreMatches.
	allFiles, excludePatterns, includePatterns, err := discoverAllFilesForConfig(cwd, cfg.IgnoreFiles, cfg.ProcessIgnoredFiles)
	if err != nil {
		return nil, err
	}

	ctx := &lintCtx{cwd: cwd, doc: doc}

	// The module universe requires the parsed dependency tree — build it only when the
	// module rule runs, so `--rules orphan-file-globs` skips parsing entirely.
	if runModule {
		universe, err := buildModuleUniverseForConfig(cfg, cwd, packageJson, tsconfigJson, allFiles, excludePatterns, includePatterns)
		if err != nil {
			return nil, err
		}
		ctx.moduleUniverse = universe
	}

	if runFile {
		// Top-level file globs. Tested against a superset discovered WITHOUT the config's
		// own ignoreFiles, so a self-erasing ignoreFiles pattern is checked against files
		// it would otherwise hide.
		supersetFiles, _, _, err := discoverAllFilesForConfig(cwd, nil, cfg.ProcessIgnoredFiles)
		if err != nil {
			return nil, err
		}
		ctx.checkFileArray(patternLoc{RuleIndex: -1, BoundaryIndex: -1, OptionKey: "ignoreFiles"}, cfg.IgnoreFiles, cwd, supersetFiles, true)
		ctx.checkFileArray(patternLoc{RuleIndex: -1, BoundaryIndex: -1, OptionKey: "processIgnoredFiles"}, cfg.ProcessIgnoredFiles, cwd, supersetFiles, true)
	}

	for i := range cfg.Rules {
		rule := cfg.Rules[i]
		fullRulePath := pathutil.StandardiseDirPath(filepath.Join(cwd, rule.Path))
		ruleFiles := filesUnderRulePath(allFiles, cwd, rule.Path)
		if runFile {
			ctx.checkRuleFileGlobs(i, rule, fullRulePath, ruleFiles)
		}
		if runModule {
			ctx.checkRuleModuleGlobs(i, rule, fullRulePath, ruleFiles)
		}
	}

	sortDeadPatterns(ctx.deads)
	return &LintResult{ConfigFilePath: configFilePath, RulesRun: rules, DeadPatterns: ctx.deads}, nil
}

// filesUnderRulePath returns the discovered files that live under the rule's directory,
// using the same normalized-prefix test filterFilesForRule uses. This is a cheap,
// parse-free substitute for filterFilesForRule when only file-glob matching is needed:
// rule-path-rooted globs cannot match files outside the rule directory anyway.
func filesUnderRulePath(allFiles []string, cwd, rulePath string) []string {
	normalizedRulePath := pathutil.NormalizePathForInternal(filepath.Clean(pathutil.JoinWithCwd(cwd, rulePath)))
	prefix := pathutil.StandardiseDirPathInternal(normalizedRulePath)
	out := make([]string, 0, len(allFiles))
	for _, f := range allFiles {
		if strings.HasPrefix(pathutil.NormalizePathForInternal(f), prefix) {
			out = append(out, f)
		}
	}
	return out
}

// patternLoc identifies where a pattern lives, for later JSONC navigation.
type patternLoc struct {
	RuleIndex     int
	RulePath      string
	DetectorType  string
	DetectorIndex int
	BoundaryIndex int
	OptionKey     string
}

func (ctx *lintCtx) add(loc patternLoc, elementIndex int, value string, kind PatternKind, removable bool) {
	ctx.deads = append(ctx.deads, DeadPattern{
		RuleIndex:     loc.RuleIndex,
		RulePath:      loc.RulePath,
		DetectorType:  loc.DetectorType,
		DetectorIndex: loc.DetectorIndex,
		BoundaryIndex: loc.BoundaryIndex,
		OptionKey:     loc.OptionKey,
		ElementIndex:  elementIndex,
		Value:         value,
		Kind:          kind,
		Removable:     removable,
	})
}

// optionPresent reports whether the option at loc physically exists as an array in the
// config file. This filters out values synthesized by rule-level entry-point
// inheritance, which are not in the file and cannot be reported or removed. When the
// config file could not be parsed, it returns true (no over-filtering).
func (ctx *lintCtx) optionPresent(loc patternLoc) bool {
	if ctx.doc == nil {
		return true
	}
	owner := locateOwnerNav(ctx.doc, loc.RuleIndex, loc.DetectorType, loc.DetectorIndex, loc.BoundaryIndex)
	if owner == nil {
		return false
	}
	return owner.Get(loc.OptionKey) != nil
}

// checkFileArray flags each pattern in values that matches no file. base is the glob
// resolution root; files is the file set to test against.
func (ctx *lintCtx) checkFileArray(loc patternLoc, values []string, base string, files []string, removable bool) {
	if len(values) == 0 || !ctx.optionPresent(loc) {
		return
	}
	for idx, v := range values {
		if patternMatchesAnyFile(v, base, files) {
			continue
		}
		ctx.add(loc, idx, v, KindFile, removable)
	}
}

// checkModuleArray flags each module pattern that matches nothing in the module universe.
func (ctx *lintCtx) checkModuleArray(loc patternLoc, values []string, removable bool) {
	if len(values) == 0 || !ctx.optionPresent(loc) {
		return
	}
	for idx, v := range values {
		if patternMatchesAnyModule(v, ctx.moduleUniverse) {
			continue
		}
		ctx.add(loc, idx, v, KindModule, removable)
	}
}

// checkMixedArray flags each pattern that matches neither a file (at root) nor a module.
func (ctx *lintCtx) checkMixedArray(loc patternLoc, values []string, rulePath string, files []string, removable bool) {
	if len(values) == 0 || !ctx.optionPresent(loc) {
		return
	}
	for idx, v := range values {
		if patternMatchesAnyFile(v, rulePath, files) || patternMatchesAnyModule(v, ctx.moduleUniverse) {
			continue
		}
		ctx.add(loc, idx, v, KindMixed, removable)
	}
}

// checkRuleFileGlobs implements the orphan-file-globs rule for one config rule: every
// file/path glob is matched (relative to the rule path) against the rule's files.
func (ctx *lintCtx) checkRuleFileGlobs(ruleIndex int, rule Rule, fullRulePath string, ruleFiles []string) {
	base := func(key string) patternLoc {
		return patternLoc{RuleIndex: ruleIndex, RulePath: rule.Path, BoundaryIndex: -1, OptionKey: key}
	}

	// Whole rule matches no files at all — report the rule path (never auto-removed).
	if len(ruleFiles) == 0 && strings.TrimSpace(rule.Path) != "" {
		ctx.add(base("path"), -1, rule.Path, KindDir, false)
	}

	ctx.checkFileArray(base("prodEntryPoints"), rule.ProdEntryPoints, fullRulePath, ruleFiles, true)
	ctx.checkFileArray(base("devEntryPoints"), rule.DevEntryPoints, fullRulePath, ruleFiles, true)
	ctx.checkFileArray(base("ignoreEntryPoints"), rule.IgnoreEntryPoints, fullRulePath, ruleFiles, true)

	det := func(key string, detIndex int) patternLoc {
		return patternLoc{RuleIndex: ruleIndex, RulePath: rule.Path, DetectorType: key, DetectorIndex: detIndex, BoundaryIndex: -1}
	}

	for di, d := range rule.OrphanFilesDetections {
		l := func(k string) patternLoc { p := det("orphanFilesDetection", di); p.OptionKey = k; return p }
		ctx.checkFileArray(l("validEntryPoints"), d.ValidEntryPoints, fullRulePath, ruleFiles, true)
		ctx.checkFileArray(l("graphExclude"), d.GraphExclude, fullRulePath, ruleFiles, true)
	}
	for di, d := range rule.UnusedExportsDetections {
		l := func(k string) patternLoc { p := det("unusedExportsDetection", di); p.OptionKey = k; return p }
		ctx.checkFileArray(l("validEntryPoints"), d.ValidEntryPoints, fullRulePath, ruleFiles, true)
		ctx.checkFileArray(l("graphExclude"), d.GraphExclude, fullRulePath, ruleFiles, true)
		ctx.checkFileArray(l("ignoreFiles"), d.IgnoreFiles, fullRulePath, ruleFiles, true)
	}
	for di, d := range rule.UnresolvedImportsDetections {
		l := func(k string) patternLoc { p := det("unresolvedImportsDetection", di); p.OptionKey = k; return p }
		ctx.checkFileArray(l("ignoreFiles"), d.IgnoreFiles, fullRulePath, ruleFiles, true)
	}
	for di, d := range rule.DevDepsUsageOnProdDetections {
		l := func(k string) patternLoc { p := det("devDepsUsageOnProdDetection", di); p.OptionKey = k; return p }
		ctx.checkFileArray(l("prodEntryPoints"), d.ProdEntryPoints, fullRulePath, ruleFiles, true)
	}
	for di, d := range rule.RestrictedImportsDetections {
		l := func(k string) patternLoc { p := det("restrictedImportsDetection", di); p.OptionKey = k; return p }
		// entryPoints / denyFiles are load-bearing (required-when-enabled): report only.
		ctx.checkFileArray(l("entryPoints"), d.EntryPoints, fullRulePath, ruleFiles, false)
		ctx.checkFileArray(l("denyFiles"), d.DenyFiles, fullRulePath, ruleFiles, false)
		ctx.checkFileArray(l("graphExclude"), d.GraphExclude, fullRulePath, ruleFiles, true)
	}
	for di, d := range rule.RestrictedImportersDetections {
		l := func(k string) patternLoc { p := det("restrictedImportersDetection", di); p.OptionKey = k; return p }
		ctx.checkFileArray(l("files"), d.Files, fullRulePath, ruleFiles, false)
		ctx.checkFileArray(l("allowedEntryPoints"), d.AllowedEntryPoints, fullRulePath, ruleFiles, false)
		ctx.checkFileArray(l("graphExclude"), d.GraphExclude, fullRulePath, ruleFiles, true)
	}
	for di, d := range rule.RestrictedDirectImportersDetections {
		l := func(k string) patternLoc {
			p := det("restrictedDirectImportersDetection", di)
			p.OptionKey = k
			return p
		}
		ctx.checkFileArray(l("files"), d.Files, fullRulePath, ruleFiles, false)
		ctx.checkFileArray(l("allowImporters"), d.AllowImporters, fullRulePath, ruleFiles, false)
		ctx.checkFileArray(l("denyImporters"), d.DenyImporters, fullRulePath, ruleFiles, false)
	}

	// Module boundaries (root = rule path, like every other detector). pattern and
	// mutuallyExclusive are load-bearing.
	for bi := range rule.ModuleBoundaries {
		b := rule.ModuleBoundaries[bi]
		bl := func(k string) patternLoc {
			return patternLoc{RuleIndex: ruleIndex, RulePath: rule.Path, DetectorType: "moduleBoundaries", DetectorIndex: bi, BoundaryIndex: bi, OptionKey: k}
		}
		if strings.TrimSpace(b.Pattern) != "" && !patternMatchesAnyFile(b.Pattern, fullRulePath, ruleFiles) {
			ctx.add(bl("pattern"), -1, b.Pattern, KindFile, false)
		}
		ctx.checkFileArray(bl("allow"), b.Allow, fullRulePath, ruleFiles, true)
		ctx.checkFileArray(bl("deny"), b.Deny, fullRulePath, ruleFiles, true)
		ctx.checkFileArray(bl("denyIgnore"), b.DenyIgnore, fullRulePath, ruleFiles, true)
		ctx.checkFileArray(bl("mutuallyExclusive"), b.MutuallyExclusive, fullRulePath, ruleFiles, false)
	}
}

// checkRuleModuleGlobs implements the orphan-module-globs rule for one config rule:
// node-module name globs are matched against the module universe, and file-or-module
// ignoreMatches against both. Requires ctx.moduleUniverse to be populated.
func (ctx *lintCtx) checkRuleModuleGlobs(ruleIndex int, rule Rule, fullRulePath string, ruleFiles []string) {
	det := func(key string, detIndex int) patternLoc {
		return patternLoc{RuleIndex: ruleIndex, RulePath: rule.Path, DetectorType: key, DetectorIndex: detIndex, BoundaryIndex: -1}
	}

	for di, d := range rule.RestrictedImportsDetections {
		l := func(k string) patternLoc { p := det("restrictedImportsDetection", di); p.OptionKey = k; return p }
		ctx.checkModuleArray(l("denyModules"), d.DenyModules, false)
		ctx.checkMixedArray(l("ignoreMatches"), d.IgnoreMatches, fullRulePath, ruleFiles, true)
	}
	for di, d := range rule.RestrictedImportersDetections {
		l := func(k string) patternLoc { p := det("restrictedImportersDetection", di); p.OptionKey = k; return p }
		ctx.checkModuleArray(l("modules"), d.Modules, false)
		ctx.checkMixedArray(l("ignoreMatches"), d.IgnoreMatches, fullRulePath, ruleFiles, true)
	}
	for di, d := range rule.RestrictedDirectImportersDetections {
		l := func(k string) patternLoc {
			p := det("restrictedDirectImportersDetection", di)
			p.OptionKey = k
			return p
		}
		ctx.checkModuleArray(l("modules"), d.Modules, false)
		ctx.checkMixedArray(l("ignoreMatches"), d.IgnoreMatches, fullRulePath, ruleFiles, true)
	}
	for di, d := range rule.UnusedNodeModulesDetections {
		l := func(k string) patternLoc { p := det("unusedNodeModulesDetection", di); p.OptionKey = k; return p }
		ctx.checkModuleArray(l("excludeModules"), d.ExcludeModules, true)
		ctx.checkModuleArray(l("includeModules"), d.IncludeModules, false)
	}
	for di, d := range rule.MissingNodeModulesDetections {
		l := func(k string) patternLoc { p := det("missingNodeModulesDetection", di); p.OptionKey = k; return p }
		ctx.checkModuleArray(l("excludeModules"), d.ExcludeModules, true)
		ctx.checkModuleArray(l("includeModules"), d.IncludeModules, false)
	}
}

// patternMatchesAnyFile reports whether pattern (resolved against base) matches at
// least one file. Empty/whitespace patterns are treated as matching (never dead).
func patternMatchesAnyFile(pattern, base string, files []string) bool {
	if strings.TrimSpace(pattern) == "" {
		return true
	}
	matchers := globutil.CreateGlobMatchers([]string{pattern}, base)
	if len(matchers) == 0 {
		return true // uncompilable pattern is not our concern
	}
	for _, f := range files {
		if globutil.MatchesAnyGlobMatcher(f, matchers, false) {
			return true
		}
	}
	return false
}

// patternMatchesAnyModule reports whether pattern matches at least one entry in the
// module universe (imported module names/requests + declared dependencies).
func patternMatchesAnyModule(pattern string, universe []string) bool {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return true
	}
	g, err := glob.Compile(trimmed)
	if err != nil {
		return true
	}
	for _, m := range universe {
		if g.Match(m) {
			return true
		}
	}
	return false
}

// buildModuleUniverseForConfig builds the dependency tree (parsing every file, the
// expensive step) and derives the module universe from it. Called only when the module
// rule runs.
func buildModuleUniverseForConfig(cfg *RevDepConfig, cwd, packageJson, tsconfigJson string, allFiles []string, excludePatterns, includePatterns []globutil.GlobMatcher) ([]string, error) {
	rulePackageDirs := make([]string, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		if rule.Path == "" {
			continue
		}
		rulePackageDirs = append(rulePackageDirs, pathutil.NormalizePathForInternal(filepath.Clean(pathutil.JoinWithCwd(cwd, rule.Path))))
	}

	fullTree, resolverManager, err := buildDependencyTreeForConfig(
		allFiles,
		excludePatterns,
		includePatterns,
		cfg.ConditionNames,
		cwd,
		packageJson,
		tsconfigJson,
		cfg.CustomAssetExtensions,
		model.ParseModeBasic,
		rulePackageDirs,
	)
	if err != nil {
		return nil, err
	}
	return buildModuleUniverse(fullTree, resolverManager, cfg, cwd), nil
}

// buildModuleUniverse gathers every module name a pattern could legitimately match:
// module names/requests imported anywhere in the dependency tree, plus the
// dependencies declared in each rule package's (and the root's) package.json.
func buildModuleUniverse(tree model.MinimalDependencyTree, rm *resolve.ResolverManager, cfg *RevDepConfig, cwd string) []string {
	set := make(map[string]bool)

	for _, deps := range tree {
		for _, d := range deps {
			switch d.ResolvedType {
			case model.NodeModule, model.NotResolvedModule, model.BuiltInModule:
				if d.Request != "" {
					set[d.Request] = true
					if name := module.GetNodeModuleName(d.Request); name != "" {
						set[name] = true
					}
				}
			}
		}
	}

	addResolver := func(mr *resolve.ModuleResolver) {
		if mr == nil {
			return
		}
		for name := range mr.NodeModules() {
			set[name] = true
		}
		for name := range mr.DevNodeModules() {
			set[name] = true
		}
	}
	if rm != nil {
		addResolver(rm.RootResolver())
		for _, rule := range cfg.Rules {
			fullRulePath := pathutil.StandardiseDirPath(filepath.Join(cwd, rule.Path))
			addResolver(rm.GetResolverForFile(fullRulePath))
		}
	}

	out := make([]string, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// sortDeadPatterns produces deterministic ordering for output and fixing.
func sortDeadPatterns(deads []DeadPattern) {
	sort.SliceStable(deads, func(i, j int) bool {
		a, b := deads[i], deads[j]
		if a.RuleIndex != b.RuleIndex {
			return a.RuleIndex < b.RuleIndex
		}
		if a.DetectorType != b.DetectorType {
			return a.DetectorType < b.DetectorType
		}
		if a.DetectorIndex != b.DetectorIndex {
			return a.DetectorIndex < b.DetectorIndex
		}
		if a.OptionKey != b.OptionKey {
			return a.OptionKey < b.OptionKey
		}
		return a.ElementIndex < b.ElementIndex
	})
}
