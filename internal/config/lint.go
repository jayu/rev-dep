package config

import (
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

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

// Severity classifies how a finding affects the exit code.
type Severity string

const (
	// SeverityError fails the lint (non-zero exit) when it remains after --fix.
	SeverityError Severity = "error"
	// SeverityWarning is advisory and never affects the exit code.
	SeverityWarning Severity = "warning"
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
	Severity      Severity // negation patterns are warnings; other dead patterns are errors
	Removable     bool     // whether --fix may auto-remove it (load-bearing patterns are report-only)
}

// OverlapKind describes how two patterns' matched-file sets relate.
type OverlapKind string

const (
	// OverlapDuplicate: the two patterns match exactly the same files.
	OverlapDuplicate OverlapKind = "duplicate"
	// OverlapContained: one pattern's files are a strict subset of the other's.
	OverlapContained OverlapKind = "contained"
	// OverlapPartial: the patterns share files but neither contains the other.
	OverlapPartial OverlapKind = "partial"
)

// OverlapFinding reports two patterns in the same option array whose matched-file sets
// overlap. Findings are always warnings (empirical — the relationship can change as
// files are added) and are never auto-removed.
type OverlapFinding struct {
	RuleIndex     int
	RulePath      string
	DetectorType  string
	DetectorIndex int
	BoundaryIndex int
	OptionKey     string

	Kind OverlapKind
	// PatternA/PatternB are the two patterns. For OverlapContained, PatternA is the
	// redundant (subset) one and PatternB is the one that covers it.
	PatternA        string
	ElementIndexA   int
	PatternB        string
	ElementIndexB   int
	SharedFileCount int // number of files matched by both
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
	// RuleOverlappingGlobs flags file globs within the same option that match overlapping
	// sets of files (one contained in another, identical, or partially overlapping). It is
	// empirical (based on the current file set) so findings are always warnings and are
	// never auto-removed. File discovery only; no dependency-tree parse.
	RuleOverlappingGlobs LintRuleName = "overlapping-globs"
	// RuleTrailingCommas reports the count of redundant trailing commas in the config
	// file (a warning). It operates on the raw file only — no discovery or parse — and its
	// findings are auto-removed by --fix.
	RuleTrailingCommas LintRuleName = "trailing-commas"
	// RuleCompact reports detector declarations that can be written more compactly — a
	// redundant "enabled": true, or an enabled-only object that could be a bare boolean.
	// It is a lossless formatter (like gofmt): the fix is deterministic and semantically
	// identical, so findings are warnings, not errors. Raw file only; no discovery or parse.
	RuleCompact LintRuleName = "compact"
)

// AllLintRules is the default set run when no selection is given, in output order.
var AllLintRules = []LintRuleName{RuleOrphanFileGlobs, RuleOrphanModuleGlobs, RuleOverlappingGlobs, RuleTrailingCommas, RuleCompact}

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
	ConfigFilePath     string
	RulesRun           []LintRuleName
	DeadPatterns       []DeadPattern
	Overlaps           []OverlapFinding
	TrailingCommaCount int // redundant trailing commas in the config file (a warning)
	CompactableCount   int // detector declarations that can be written more compactly (a warning)
}

// lintCtx holds the discovered universes for one lint run.
type lintCtx struct {
	cwd            string
	moduleUniverse []string      // populated only when the module rule runs
	doc            *JSONDocument // parsed config file; used to check physical presence
	runFile        bool          // orphan-file-globs selected
	runOverlap     bool          // overlapping-globs selected
	mu             sync.Mutex    // guards deads/overlaps (checks run in parallel)
	deads          []DeadPattern
	overlaps       []OverlapFinding

	// Bounded worker pool: every per-option check is submitted as a task and run across
	// GOMAXPROCS workers, so heavy options (even within one rule) parallelize.
	sem    chan struct{}
	taskWg sync.WaitGroup
}

// submit runs fn on the bounded worker pool. Tasks must not themselves submit (would
// deadlock when the pool is full).
func (ctx *lintCtx) submit(fn func()) {
	ctx.taskWg.Add(1)
	ctx.sem <- struct{}{}
	go func() {
		defer ctx.taskWg.Done()
		defer func() { <-ctx.sem }()
		fn()
	}()
}

// LintGraph carries discovery/tree artifacts a caller already built (e.g. `config run`),
// so the linter can reuse them instead of redoing the expensive discovery + dependency
// tree build. AllFiles are the discovered files (respecting the config's ignoreFiles);
// FullTree/ResolverManager are used only by the module rule.
type LintGraph struct {
	AllFiles        []string
	FullTree        model.MinimalDependencyTree
	ResolverManager *resolve.ResolverManager
	// SupersetFiles is the file set discovered WITHOUT the config's ignoreFiles, used for
	// the top-level ignoreFiles dead-check. The normal discovery prunes those dirs, so this
	// is a separate walk; a caller can run it concurrently (e.g. alongside `config run`) and
	// set SupersetComputed to have the linter reuse it instead of walking again.
	SupersetFiles    []string
	SupersetComputed bool
}

// DiscoverLintSuperset walks the workspace WITHOUT applying the config's ignoreFiles,
// returning the file set the top-level ignoreFiles dead-check needs. Callers can run this
// concurrently with other work and pass the result via LintGraph.SupersetFiles.
func DiscoverLintSuperset(cwd string, processIgnoredFiles []string) ([]string, error) {
	files, _, _, err := discoverAllFilesForConfig(cwd, nil, processIgnoredFiles)
	return files, err
}

// LintConfig analyzes cfg and returns every dead pattern for the selected rules (all
// rules when `rules` is empty). It builds its own discovery + dependency tree. Callers
// that already have those artifacts should use LintConfigWithGraph to avoid the cost.
func LintConfig(cfg *RevDepConfig, cwd, packageJson, tsconfigJson string, rules []LintRuleName) (*LintResult, error) {
	return LintConfigWithGraph(cfg, cwd, packageJson, tsconfigJson, rules, nil)
}

// LintConfigWithGraph is LintConfig with an optional prebuilt graph. When graph is
// non-nil, discovery and the dependency-tree build are skipped and its artifacts are
// reused; when nil, they are built as needed (the dependency tree only for the module
// rule; the file rule runs from discovery alone).
func LintConfigWithGraph(cfg *RevDepConfig, cwd, packageJson, tsconfigJson string, rules []LintRuleName, graph *LintGraph) (*LintResult, error) {
	if len(rules) == 0 {
		rules = AllLintRules
	}
	runFile, runModule, runOverlap, runTrailingCommas, runCompact := false, false, false, false, false
	for _, r := range rules {
		switch r {
		case RuleOrphanFileGlobs:
			runFile = true
		case RuleOrphanModuleGlobs:
			runModule = true
		case RuleOverlappingGlobs:
			runOverlap = true
		case RuleTrailingCommas:
			runTrailingCommas = true
		case RuleCompact:
			runCompact = true
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

	ctx := &lintCtx{cwd: cwd, doc: doc, runFile: runFile, runOverlap: runOverlap, sem: make(chan struct{}, runtime.GOMAXPROCS(0))}

	// Only the file/module/overlap rules need file discovery. When only trailing-commas
	// or compact is selected, skip discovery entirely — those are pure document scans.
	if runFile || runModule || runOverlap {
		// The top-level ignoreFiles dead-check needs a "superset" walk (files the normal
		// discovery prunes, e.g. under ".superset/**"). It is independent of everything else,
		// so launch it FIRST — it overlaps with the discovery + tree build below and the
		// per-rule checks, and is joined only at the very end. When the caller already
		// computed it (config run --lint runs it alongside the run), it is reused for free.
		needSuperset := (runFile || runOverlap) && (len(cfg.IgnoreFiles) > 0 || len(cfg.ProcessIgnoredFiles) > 0)
		var supersetFiles []string
		var supersetCh chan []string
		if needSuperset {
			if graph != nil && graph.SupersetComputed {
				supersetFiles = graph.SupersetFiles
			} else {
				supersetCh = make(chan []string, 1)
				go func() {
					d, _, _, _ := discoverAllFilesForConfig(cwd, nil, cfg.ProcessIgnoredFiles)
					supersetCh <- d
				}()
			}
		}

		// Discovered files (respecting gitignore + the config's ignoreFiles), needed by the
		// file/overlap rules and the module rule. Reused from the prebuilt graph when given.
		var allFiles []string
		if graph != nil {
			allFiles = graph.AllFiles
			if runModule {
				ctx.moduleUniverse = buildModuleUniverse(graph.FullTree, graph.ResolverManager, cfg, cwd)
			}
		} else {
			discovered, excludePatterns, includePatterns, err := discoverAllFilesForConfig(cwd, cfg.IgnoreFiles, cfg.ProcessIgnoredFiles)
			if err != nil {
				return nil, err
			}
			allFiles = discovered
			// The module universe requires the parsed dependency tree — build it only when the
			// module rule runs, so file-only rule selections skip parsing entirely.
			if runModule {
				universe, err := buildModuleUniverseForConfig(cfg, cwd, packageJson, tsconfigJson, allFiles, excludePatterns, includePatterns)
				if err != nil {
					return nil, err
				}
				ctx.moduleUniverse = universe
			}
		}

		// Each per-option check is submitted to the worker pool (ctx.submit), so options
		// across ALL rules run concurrently — not just rule-by-rule. Findings are collected
		// under ctx.mu; the glob matching (the expensive part) happens outside the lock.
		for i := range cfg.Rules {
			rule := cfg.Rules[i]
			fullRulePath := pathutil.StandardiseDirPath(filepath.Join(cwd, rule.Path))
			ruleFiles := filesUnderRulePath(allFiles, cwd, rule.Path)
			if runFile || runOverlap {
				ctx.checkRuleFileGlobs(i, rule, fullRulePath, ruleFiles)
			}
			if runModule {
				ctx.checkRuleModuleGlobs(i, rule, fullRulePath, ruleFiles)
			}
		}

		// Join the superset walk (overlapped with everything above) and submit the
		// top-level checks too.
		if needSuperset {
			if supersetCh != nil {
				supersetFiles = <-supersetCh
			}
			ctx.checkFileArray(patternLoc{RuleIndex: -1, BoundaryIndex: -1, OptionKey: "ignoreFiles"}, cfg.IgnoreFiles, cwd, supersetFiles, true)
			ctx.checkFileArray(patternLoc{RuleIndex: -1, BoundaryIndex: -1, OptionKey: "processIgnoredFiles"}, cfg.ProcessIgnoredFiles, cwd, supersetFiles, true)
		}

		ctx.taskWg.Wait()
	}

	sortDeadPatterns(ctx.deads)
	sortOverlaps(ctx.overlaps)

	trailingCommaCount := 0
	if runTrailingCommas && doc != nil {
		trailingCommaCount = len(findTrailingCommaPositions(doc.Original))
	}

	compactableCount := 0
	if runCompact && doc != nil {
		compactableCount = len(compactEdits(doc))
	}

	return &LintResult{
		ConfigFilePath:     configFilePath,
		RulesRun:           rules,
		DeadPatterns:       ctx.deads,
		Overlaps:           ctx.overlaps,
		TrailingCommaCount: trailingCommaCount,
		CompactableCount:   compactableCount,
	}, nil
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
	// A dead negation pattern (`!foo`) is a warning, not an error: negations exclude
	// files that legitimately may not exist, so "matches nothing" is expected and is not
	// a failure. Negations are also never auto-removed (removing an exclusion changes
	// behavior, and the file may exist later).
	severity := SeverityError
	if isNegationPattern(value) {
		severity = SeverityWarning
		removable = false
	}
	ctx.mu.Lock()
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
		Severity:      severity,
		Removable:     removable,
	})
	ctx.mu.Unlock()
}

// isNegationPattern reports whether a pattern is a gitignore-style negation (`!foo`).
// A backslash-escaped `\!foo` is a literal, not a negation.
func isNegationPattern(pattern string) bool {
	return strings.HasPrefix(strings.TrimSpace(pattern), "!")
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

// checkFileArray serves both file-based rules over one option array: it flags patterns
// that match no file (orphan-file-globs) and reports overlapping patterns
// (overlapping-globs). base is the glob resolution root; files is the file set to test
// against. Each half is gated on its rule being selected.
//
// When BOTH rules run (the default), the two share one matching pass: the overlap
// bitset already reveals dead-ness (an empty bitset matches nothing), so the separate
// short-circuit dead scan is skipped entirely.
func (ctx *lintCtx) checkFileArray(loc patternLoc, values []string, base string, files []string, removable bool) {
	if len(values) == 0 || !ctx.optionPresent(loc) {
		return
	}
	ctx.submit(func() {
		switch {
		case ctx.runFile && ctx.runOverlap:
			ctx.checkFileGlobsAndOverlaps(loc, values, base, files, removable)
		case ctx.runFile:
			for idx, v := range values {
				if patternMatchesAnyFile(v, base, files) {
					continue
				}
				ctx.add(loc, idx, v, KindFile, removable)
			}
		case ctx.runOverlap:
			ctx.checkOverlaps(loc, values, base, files)
		}
	})
}

// checkFileGlobsAndOverlaps computes each pattern's matched-file bitset once, then
// derives the dead-pattern finding (empty bitset) and the overlap findings from it.
func (ctx *lintCtx) checkFileGlobsAndOverlaps(loc patternLoc, values []string, base string, files []string, removable bool) {
	pats := make([]overlapPat, 0, len(values))
	for idx, v := range values {
		if strings.TrimSpace(v) == "" {
			continue // empty → never dead, never overlaps
		}
		matchers := globutil.CreateGlobMatchers([]string{v}, base)
		if len(matchers) == 0 {
			continue // uncompilable → not our concern
		}
		bs := newBitset(len(files))
		for i, f := range files {
			if globutil.MatchesAnyGlobMatcher(f, matchers, false) {
				bs.set(i)
			}
		}
		if bs.count == 0 {
			ctx.add(loc, idx, v, KindFile, removable) // dead (negations become warnings in add)
			continue
		}
		if !isNegationPattern(v) {
			pats = append(pats, overlapPat{idx, v, bs})
		}
	}
	ctx.overlapPairs(loc, pats)
}

// checkModuleArray flags each module pattern that matches nothing in the module universe.
func (ctx *lintCtx) checkModuleArray(loc patternLoc, values []string, removable bool) {
	if len(values) == 0 || !ctx.optionPresent(loc) {
		return
	}
	ctx.submit(func() {
		for idx, v := range values {
			if patternMatchesAnyModule(v, ctx.moduleUniverse) {
				continue
			}
			ctx.add(loc, idx, v, KindModule, removable)
		}
	})
}

// checkMixedArray flags each pattern that matches neither a file (at root) nor a module.
func (ctx *lintCtx) checkMixedArray(loc patternLoc, values []string, rulePath string, files []string, removable bool) {
	if len(values) == 0 || !ctx.optionPresent(loc) {
		return
	}
	ctx.submit(func() {
		for idx, v := range values {
			if patternMatchesAnyFile(v, rulePath, files) || patternMatchesAnyModule(v, ctx.moduleUniverse) {
				continue
			}
			ctx.add(loc, idx, v, KindMixed, removable)
		}
	})
}

// checkRuleFileGlobs implements the orphan-file-globs rule for one config rule: every
// file/path glob is matched (relative to the rule path) against the rule's files.
func (ctx *lintCtx) checkRuleFileGlobs(ruleIndex int, rule Rule, fullRulePath string, ruleFiles []string) {
	base := func(key string) patternLoc {
		return patternLoc{RuleIndex: ruleIndex, RulePath: rule.Path, BoundaryIndex: -1, OptionKey: key}
	}

	// Whole rule matches no files at all — report the rule path (never auto-removed).
	// This is a dead-pattern finding only (there is nothing to overlap).
	if ctx.runFile && len(ruleFiles) == 0 && strings.TrimSpace(rule.Path) != "" {
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
		if ctx.runFile && strings.TrimSpace(b.Pattern) != "" && !patternMatchesAnyFile(b.Pattern, fullRulePath, ruleFiles) {
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

// ---- overlapping-globs ----

// overlapPat is one pattern with its matched-file bitset, for overlap comparison.
type overlapPat struct {
	elemIdx int
	value   string
	bs      *bitset
}

// checkOverlaps computes each pattern's matched-file bitset and reports overlaps.
// Patterns matching no file (dead) and negations are skipped.
func (ctx *lintCtx) checkOverlaps(loc patternLoc, values []string, base string, files []string) {
	pats := make([]overlapPat, 0, len(values))
	for idx, v := range values {
		if isNegationPattern(v) || strings.TrimSpace(v) == "" {
			continue
		}
		bs := computeMatchBitset(v, base, files)
		if bs.count == 0 {
			continue
		}
		pats = append(pats, overlapPat{idx, v, bs})
	}
	ctx.overlapPairs(loc, pats)
}

// overlapPairs compares every pair of patterns by their matched-file sets and reports
// duplicates, containment, and partial overlaps.
func (ctx *lintCtx) overlapPairs(loc patternLoc, pats []overlapPat) {
	for a := 0; a < len(pats); a++ {
		for b := a + 1; b < len(pats); b++ {
			A, B := pats[a], pats[b]
			aInB := A.bs.subsetOf(B.bs)
			bInA := B.bs.subsetOf(A.bs)
			shared := A.bs.intersectionCount(B.bs)
			switch {
			case aInB && bInA:
				ctx.addOverlap(loc, OverlapDuplicate, A.value, A.elemIdx, B.value, B.elemIdx, shared)
			case aInB:
				// A's files ⊂ B's files → A is the redundant (subset) one.
				ctx.addOverlap(loc, OverlapContained, A.value, A.elemIdx, B.value, B.elemIdx, shared)
			case bInA:
				// B's files ⊂ A's files → B is redundant; list it first.
				ctx.addOverlap(loc, OverlapContained, B.value, B.elemIdx, A.value, A.elemIdx, shared)
			case shared > 0:
				ctx.addOverlap(loc, OverlapPartial, A.value, A.elemIdx, B.value, B.elemIdx, shared)
			}
		}
	}
}

func (ctx *lintCtx) addOverlap(loc patternLoc, kind OverlapKind, aVal string, aIdx int, bVal string, bIdx int, shared int) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.overlaps = append(ctx.overlaps, OverlapFinding{
		RuleIndex:       loc.RuleIndex,
		RulePath:        loc.RulePath,
		DetectorType:    loc.DetectorType,
		DetectorIndex:   loc.DetectorIndex,
		BoundaryIndex:   loc.BoundaryIndex,
		OptionKey:       loc.OptionKey,
		Kind:            kind,
		PatternA:        aVal,
		ElementIndexA:   aIdx,
		PatternB:        bVal,
		ElementIndexB:   bIdx,
		SharedFileCount: shared,
	})
}

// computeMatchBitset returns the set of file indices (into files) the pattern matches,
// using the same matcher the tool uses at runtime.
func computeMatchBitset(pattern, base string, files []string) *bitset {
	bs := newBitset(len(files))
	matchers := globutil.CreateGlobMatchers([]string{pattern}, base)
	if len(matchers) == 0 {
		return bs
	}
	for i, f := range files {
		if globutil.MatchesAnyGlobMatcher(f, matchers, false) {
			bs.set(i)
		}
	}
	return bs
}

// bitset is a fixed-size set of file indices.
type bitset struct {
	words []uint64
	count int
}

func newBitset(n int) *bitset {
	return &bitset{words: make([]uint64, (n+63)/64)}
}

func (b *bitset) set(i int) {
	w, m := i>>6, uint64(1)<<(uint(i)&63)
	if b.words[w]&m == 0 {
		b.words[w] |= m
		b.count++
	}
}

// subsetOf reports whether every bit set in b is also set in o.
func (b *bitset) subsetOf(o *bitset) bool {
	for i, w := range b.words {
		if w&^o.words[i] != 0 {
			return false
		}
	}
	return true
}

func (b *bitset) intersectionCount(o *bitset) int {
	n := 0
	for i, w := range b.words {
		n += bits.OnesCount64(w & o.words[i])
	}
	return n
}

func sortOverlaps(overlaps []OverlapFinding) {
	sort.SliceStable(overlaps, func(i, j int) bool {
		a, b := overlaps[i], overlaps[j]
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
		if a.ElementIndexA != b.ElementIndexA {
			return a.ElementIndexA < b.ElementIndexA
		}
		return a.ElementIndexB < b.ElementIndexB
	})
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
