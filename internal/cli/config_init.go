package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"rev-dep-go/internal/config"
	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/monorepo"
	"rev-dep-go/internal/pathutil"
)

// ---------------- config init command ----------------

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new rev-dep.config.json file",
	Long:  `Create a new rev-dep.config.json configuration file in the current directory with default settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := pathutil.ResolveAbsoluteCwd(configCwd)
		result, err := initConfigInteractive(cwd, promptIncludeStandalone, promptAutoDetectEntryPoints, promptDetectorPreset)
		if err != nil {
			return err
		}
		printInitConfigResults(result)
		return nil
	},
}

// initConfigResult captures what config init produced so callers can report it.
type initConfigResult struct {
	configPath                   string
	rules                        []config.Rule
	isMonorepo                   bool
	workspacePackagePaths        []string // relative paths of monorepo workspace packages (each got a rule)
	standalonePackagePaths       []string // relative paths of standalone packages included in the config
	createdForMonorepoSubPackage bool
	rootRuleCreated              bool // whether a "." root rule was created
	rootHasPackageJson           bool
	entryPointsDetected          bool // entry-point auto-detection was run
	entryPointPackageCount       int  // number of package rules that got entry points
}

// ---------------- project detection ----------------

// projectStructure describes how a project is laid out. It drives which init questions to ask
// and which rules to generate.
type projectStructure struct {
	cwd                   string
	isMonorepo            bool     // a workspace root was detected at/above cwd
	isMonorepoSubPackage  bool     // cwd is inside a monorepo but not at its root
	rootHasPackageJson    bool     // cwd itself has a package.json
	workspacePackageDirs  []string // internal-form absolute dirs of monorepo workspace packages (root case)
	standalonePackageDirs []string // internal-form absolute dirs of standalone subdir packages (not workspaces)
}

// detectProjectStructure inspects cwd to determine whether it is a monorepo, a single-package
// project, or neither, and which standalone subdirectory packages exist alongside it.
func detectProjectStructure(cwd string) projectStructure {
	ps := projectStructure{cwd: cwd, rootHasPackageJson: hasPackageJson(cwd)}
	excludePatterns := globutil.CreateGlobMatchers([]string{}, cwd)

	monorepoCtx := monorepo.DetectMonorepo(cwd)
	if monorepoCtx != nil {
		ps.isMonorepo = true

		// cwd is in OS form (backslash on Windows, with a trailing separator) while WorkspaceRoot
		// is in internal forward-slash form (no trailing slash). Normalize BOTH the separators and
		// the trailing slash before comparing - a plain string compare silently mismatches on
		// Windows and makes the workspace root look like a sub-package.
		cwdInternal := pathutil.StandardiseDirPathInternal(pathutil.NormalizePathForInternal(cwd))
		rootInternal := pathutil.StandardiseDirPathInternal(pathutil.NormalizePathForInternal(monorepoCtx.WorkspaceRoot))
		if cwdInternal != rootInternal {
			// Inside a workspace package, not at the root: scope to the current package only.
			ps.isMonorepoSubPackage = true
			return ps
		}

		monorepoCtx.FindWorkspacePackages(excludePatterns, nil)
		ps.workspacePackageDirs = mapValues(monorepoCtx.PackageToPath)
		ps.standalonePackageDirs = monorepoCtx.DiscoverStandalonePackages(excludePatterns)
		return ps
	}

	// No monorepo: still discover standalone subdirectory packages.
	rootCtx := monorepo.NewMonorepoContext(pathutil.NormalizePathForInternal(filepath.Clean(cwd)))
	ps.standalonePackageDirs = rootCtx.DiscoverStandalonePackages(excludePatterns)
	return ps
}

// standaloneChoiceApplies reports whether the standalone-packages question should be asked:
//   - When a base project (monorepo or single root package) was detected AND standalone
//     subdirectory packages exist alongside it, the user chooses base-only vs including them.
//   - When there is no base project (only subdir packages), there is no base-only choice, but we
//     still ask when curation would make a difference (some folders look like fixtures/tests) so
//     the user can pick all vs the curated subset.
//
// Otherwise there is nothing to choose between.
func standaloneChoiceApplies(ps projectStructure, standalone standalonePackages) bool {
	if ps.isMonorepoSubPackage || len(standalone.all) == 0 {
		return false
	}
	if ps.isMonorepo || ps.rootHasPackageJson {
		return true // base-only vs including standalone packages is always a choice
	}
	// No base project: only meaningful when curation actually splits the set.
	return len(standalone.filteredOut) > 0 && len(standalone.curated) > 0
}

// ---------------- standalone package selection ----------------

// standaloneSelection is how many of the discovered standalone subdirectory packages should be
// included in the config.
type standaloneSelection int

const (
	standaloneNone    standaloneSelection = iota // base project only
	standaloneCurated                            // base + real packages (fixture-like folders filtered out)
	standaloneAll                                // base + every discovered standalone package
)

// standalonePackages holds the standalone subdirectory packages discovered for a project,
// partitioned into curated (real) packages and those filtered out by non-package
// directory-name heuristics (fixtures, __*__, tests, examples, ...).
type standalonePackages struct {
	all         []string // all standalone package rel paths (sorted)
	curated     []string // subset that does not match any non-package pattern (sorted)
	filteredOut []string // subset that matched a non-package pattern (sorted)
	patterns    []string // distinct matched patterns, in order of first appearance
}

// selected returns the standalone rel paths to include for the given selection.
func (sp standalonePackages) selected(sel standaloneSelection) []string {
	switch sel {
	case standaloneAll:
		return sp.all
	case standaloneCurated:
		return sp.curated
	default:
		return nil
	}
}

// classifyStandalonePackages converts the discovered standalone package dirs to sorted
// cwd-relative paths and partitions them into curated vs filtered-out based on non-package
// directory-name heuristics, recording the distinct patterns that triggered filtering.
func classifyStandalonePackages(cwd string, dirs []string) standalonePackages {
	sp := standalonePackages{all: packageDirsToSortedRelPaths(cwd, dirs)}
	seen := map[string]bool{}
	for _, relPath := range sp.all {
		if pattern := matchNonPackagePattern(relPath); pattern != "" {
			sp.filteredOut = append(sp.filteredOut, relPath)
			if key := strings.ToLower(pattern); !seen[key] {
				seen[key] = true
				sp.patterns = append(sp.patterns, pattern)
			}
		} else {
			sp.curated = append(sp.curated, relPath)
		}
	}
	return sp
}

// ---------------- config generation ----------------

// standaloneAsker decides which standalone subdirectory packages to include in the config. It
// is only consulted when standaloneChoiceApplies(ps) is true.
type standaloneAsker func(ps projectStructure, standalone standalonePackages) standaloneSelection

// initConfigInteractive detects the project layout, asks the standalone-packages question when
// applicable (via askStandalone) and the entry-points question (via askEntryPoints), then builds
// and writes the config file.
func initConfigInteractive(cwd string, askStandalone standaloneAsker, askEntryPoints func() bool, askDetectors func() detectorPreset) (*initConfigResult, error) {
	if existing, err := config.FindConfigFile(cwd); err == nil && existing != "" {
		return nil, fmt.Errorf("config file already exists at %s", existing)
	}

	ps := detectProjectStructure(cwd)
	standalone := classifyStandalonePackages(cwd, ps.standalonePackageDirs)

	// Default: include everything (used when no question applies and by the non-interactive core).
	selection := standaloneAll
	if standaloneChoiceApplies(ps, standalone) {
		selection = askStandalone(ps, standalone)
	}

	autoDetectEntryPoints := askEntryPoints()
	preset := askDetectors()

	result := buildConfigResult(ps, standalone, selection, preset)
	if autoDetectEntryPoints {
		result.entryPointPackageCount = applyEntryPointsToRules(cwd, ps, result.rules)
		result.entryPointsDetected = true
	}
	if err := writeInitConfig(cwd, result); err != nil {
		return nil, err
	}
	return result, nil
}

// applyEntryPointsToRules analyzes each package rule and fills in its prodEntryPoints /
// devEntryPoints. The monorepo root rule is skipped (it is a boundaries-only rule spanning all
// packages, not a package). Returns the number of package rules processed.
func applyEntryPointsToRules(cwd string, ps projectStructure, rules []config.Rule) int {
	count := 0
	for i := range rules {
		rule := &rules[i]
		if ps.isMonorepo && !ps.isMonorepoSubPackage && rule.Path == "." {
			continue // monorepo root: boundaries only, not a package
		}
		pkgDir := pathutil.StandardiseDirPath(filepath.Join(cwd, rule.Path))
		rule.ProdEntryPoints, rule.DevEntryPoints, rule.IgnoreEntryPoints = detectPackageEntryPoints(pkgDir)
		count++
	}
	return count
}

// buildConfigResult produces the rules and reporting metadata for the detected structure. The
// selection controls which standalone subdirectory packages get their own rules. When there is no
// base project (no monorepo, no root package.json) the standalone packages are all there is, so
// the selection is at least the curated set (never "none").
func buildConfigResult(ps projectStructure, standalone standalonePackages, selection standaloneSelection, preset detectorPreset) *initConfigResult {
	result := &initConfigResult{
		isMonorepo:         ps.isMonorepo,
		rootHasPackageJson: ps.rootHasPackageJson,
	}
	var rules []config.Rule

	switch {
	case ps.isMonorepoSubPackage:
		rules = append(rules, makePackageRule(".", preset))
		result.createdForMonorepoSubPackage = true
		result.rootRuleCreated = true
	case ps.isMonorepo:
		rules = append(rules, makeMonorepoRootRule())
		result.rootRuleCreated = true
		workspacePaths := packageDirsToSortedRelPaths(ps.cwd, ps.workspacePackageDirs)
		for _, relPath := range workspacePaths {
			rules = append(rules, makePackageRule(relPath, preset))
		}
		result.workspacePackagePaths = workspacePaths
	case ps.rootHasPackageJson:
		rules = append(rules, makeSrcRootRule(preset))
		result.rootRuleCreated = true
	default:
		// No base project: no root rule; the standalone packages are all that gets configured.
		// Guard against an empty config if a "none" selection reaches here (shouldn't, since the
		// no-base question only offers curated/all) by falling back to the curated set.
		if selection == standaloneNone {
			selection = standaloneCurated
		}
	}

	standalonePaths := standalone.selected(selection)
	for _, relPath := range standalonePaths {
		rules = append(rules, makePackageRule(relPath, preset))
	}
	result.standalonePackagePaths = standalonePaths

	// Fallback: nothing discovered at all -> a single root rule so the config isn't empty.
	if len(rules) == 0 {
		rules = append(rules, makeSrcRootRule(preset))
		result.rootRuleCreated = true
	}

	result.rules = rules
	return result
}

// writeInitConfig marshals the generated rules to the standard config file and records the path.
func writeInitConfig(cwd string, result *initConfigResult) error {
	configPath := filepath.Join(cwd, ".rev-dep.config.jsonc")
	result.configPath = configPath

	cfg := config.RevDepConfig{
		ConfigVersion: config.CurrentConfigVersion,
		Rules:         result.rules,
		Schema:        "https://github.com/jayu/rev-dep/blob/master/config-schema/" + config.CurrentConfigVersion + ".schema.json?raw=true",
		NodeModulesResolution: &config.NodeModulesResolutionConfig{
			ResolutionType:         config.NodeModulesResolutionEntryPackage,
			IncludeDevDepsFromRoot: false,
		},
	}

	configJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}
	return nil
}

// initConfigFileCore detects the project layout and writes a config that includes every
// discovered package (standalone subdirectory packages included). This is the non-interactive
// entry point used by tests and the exported wrapper.
func initConfigFileCore(cwd string) (*initConfigResult, error) {
	return initConfigInteractive(cwd,
		func(projectStructure, standalonePackages) standaloneSelection { return standaloneAll },
		func() bool { return false },
		func() detectorPreset { return detectorsScaffold },
	)
}

// ---------------- non-interactive API ----------------

// StandaloneChoice selects which discovered standalone subdirectory packages a generated config
// should cover.
type StandaloneChoice int

const (
	StandaloneNone    StandaloneChoice = iota // base project only, no subfolder packages
	StandaloneCurated                         // base + real packages (fixture/test-like folders filtered out)
	StandaloneAll                             // base + every discovered standalone package
)

func (c StandaloneChoice) toSelection() standaloneSelection {
	switch c {
	case StandaloneAll:
		return standaloneAll
	case StandaloneCurated:
		return standaloneCurated
	default:
		return standaloneNone
	}
}

// DetectorChoice selects which detectors the generated rules enable.
type DetectorChoice int

const (
	DetectorsNone               DetectorChoice = iota // no detectors
	DetectorsUnresolvedOnly                           // only unresolved imports
	DetectorsUnresolvedCircular                       // unresolved + circular imports
	DetectorsScaffold                                 // circular + unresolved enabled, others listed but disabled
	DetectorsAll                                      // all listed detectors enabled
)

func (c DetectorChoice) toPreset() detectorPreset {
	switch c {
	case DetectorsAll:
		return detectorsAll
	case DetectorsScaffold:
		return detectorsScaffold
	case DetectorsUnresolvedCircular:
		return detectorsUnresolvedCircular
	case DetectorsUnresolvedOnly:
		return detectorsUnresolvedOnly
	default:
		return detectorsNone
	}
}

// InitOptions configures a non-interactive config init run.
type InitOptions struct {
	// Standalone selects which standalone subdirectory packages to include. It is applied when a
	// standalone choice actually exists (see standaloneChoiceApplies); otherwise all discovered
	// standalone packages are included.
	Standalone StandaloneChoice
	// DetectEntryPoints analyzes each package and fills in prod/dev/ignore entry points.
	DetectEntryPoints bool
	// Detectors selects which detectors the generated rules enable.
	Detectors DetectorChoice
}

// InitConfigResult reports what a non-interactive config init produced.
type InitConfigResult struct {
	ConfigPath             string
	Rules                  []config.Rule
	IsMonorepo             bool
	MonorepoPackageCount   int
	WorkspacePackagePaths  []string
	StandalonePackagePaths []string
	EntryPointsDetected    bool
	EntryPointPackageCount int
}

// InitConfig creates a .rev-dep.config.jsonc at cwd non-interactively, applying the given options.
// It runs the full init pipeline — project detection, standalone-subfolder filtering, entry-point
// detection, and detector selection — in a single call, with no terminal prompts. It errors if a
// config file already exists at cwd.
func InitConfig(cwd string, opts InitOptions) (*InitConfigResult, error) {
	result, err := initConfigInteractive(cwd,
		func(projectStructure, standalonePackages) standaloneSelection { return opts.Standalone.toSelection() },
		func() bool { return opts.DetectEntryPoints },
		func() detectorPreset { return opts.Detectors.toPreset() },
	)
	if err != nil {
		return nil, err
	}
	return &InitConfigResult{
		ConfigPath:             result.configPath,
		Rules:                  result.rules,
		IsMonorepo:             result.isMonorepo,
		MonorepoPackageCount:   len(result.workspacePackagePaths),
		WorkspacePackagePaths:  result.workspacePackagePaths,
		StandalonePackagePaths: result.standalonePackagePaths,
		EntryPointsDetected:    result.entryPointsDetected,
		EntryPointPackageCount: result.entryPointPackageCount,
	}, nil
}

// ---------------- first question: which packages should the config cover? ----------------

// maxReportedFilterPatterns caps how many non-package patterns are named in the curated option.
const maxReportedFilterPatterns = 2

// promptIncludeStandalone asks which standalone subdirectory packages the config should cover and
// returns the chosen selection. The options depend on whether a base project was detected:
//   - With a base project: "base only" (default), an optional curated middle option (base + real
//     packages, when fixture/test-like folders were filtered out), and "base + all".
//   - Without a base project (only subfolders): "curated" (default) and "all". There is no
//     base-only option — the subfolders are all there is to configure.
//
// The default is always the first (most conservative) option, and that same default is returned
// when stdin is not a terminal so scripted runs never block. It is only called when
// standaloneChoiceApplies is true.
func promptIncludeStandalone(ps projectStructure, standalone standalonePackages) standaloneSelection {
	baseDetected := ps.isMonorepo || ps.rootHasPackageJson
	hasCuratedDistinct := len(standalone.filteredOut) > 0 && len(standalone.curated) > 0
	curatedLabelSuffix := fmt.Sprintf("%d curated %s in subfolders (excluding %s)", len(standalone.curated), packagesWord(len(standalone.curated)), previewPatterns(standalone.patterns))

	// The "all" option includes the fixture/build-output folders our heuristics flagged, so name
	// them — this is the only way the user sees, e.g., an "apps/web/.next" that is otherwise filtered.
	filteredNote := ""
	if len(standalone.filteredOut) > 0 {
		filteredNote = fmt.Sprintf(" (including filtered: %s)", previewPatterns(standalone.patterns))
	}

	var options []string
	var selections []standaloneSelection

	if baseDetected {
		baseLabel := "Root package only"
		if ps.isMonorepo {
			baseLabel = fmt.Sprintf("Monorepo only — root + %d workspace %s", len(ps.workspacePackageDirs), packagesWord(len(ps.workspacePackageDirs)))
		}
		options = append(options, baseLabel)
		selections = append(selections, standaloneNone)

		if hasCuratedDistinct {
			options = append(options, fmt.Sprintf("%s + %s", baseLabel, curatedLabelSuffix))
			selections = append(selections, standaloneCurated)
		}
		options = append(options, fmt.Sprintf("%s + all %d standalone %s in subfolders%s", baseLabel, len(standalone.all), packagesWord(len(standalone.all)), filteredNote))
		selections = append(selections, standaloneAll)
	} else {
		// No base project: choose between the curated subset and all subfolders. This is only
		// reached when curation splits the set (see standaloneChoiceApplies), so both are non-empty.
		options = append(options, strings.ToUpper(curatedLabelSuffix[:1])+curatedLabelSuffix[1:])
		selections = append(selections, standaloneCurated)
		options = append(options, fmt.Sprintf("All %d %s in subfolders%s", len(standalone.all), packagesWord(len(standalone.all)), filteredNote))
		selections = append(selections, standaloneAll)
	}

	const defaultIndex = 0
	if !stdinIsInteractive() {
		return selections[defaultIndex]
	}

	idx, _, err := selectOne(os.Stdin, os.Stdout, "Standalone packages were found in subfolders. What should the config cover?", options, defaultIndex)
	if err != nil {
		return selections[defaultIndex]
	}
	return selections[idx]
}

// nonPackageDirNames are lowercase directory basenames that typically hold fixtures, test data,
// examples, or generated artifacts rather than a shippable package. A standalone package whose
// path contains any of these segments is treated as a likely non-package.
var nonPackageDirNames = map[string]bool{
	"fixtures": true, "fixture": true,
	"mocks": true, "mock": true, "__mocks__": true,
	"test": true, "tests": true, "__tests__": true, "testdata": true, "test-data": true,
	"examples": true, "example": true,
	"demo": true, "demos": true,
	"sample": true, "samples": true,
	"snapshots": true, "__snapshots__": true,
	"stories": true,
	".next":   true, // Next.js build output
}

// matchNonPackagePattern returns the path segment of relPath that marks it as a likely
// non-package folder (a known fixture/test/example/etc. name, or any __double-underscored__
// name), or "" if relPath looks like a real package.
func matchNonPackagePattern(relPath string) string {
	for _, segment := range strings.Split(relPath, "/") {
		if segment == "" {
			continue
		}
		if nonPackageDirNames[strings.ToLower(segment)] || isUnderscoreWrapped(segment) {
			return segment
		}
	}
	return ""
}

// isUnderscoreWrapped reports whether s is a __double-underscore-wrapped__ name (e.g. the
// convention used for __fixtures__, __mocks__, __generated__).
func isUnderscoreWrapped(s string) bool {
	return len(s) >= 5 && strings.HasPrefix(s, "__") && strings.HasSuffix(s, "__")
}

// packagesWord returns the correctly pluralized noun "package"/"packages" for a count.
func packagesWord(n int) string {
	if n == 1 {
		return "package"
	}
	return "packages"
}

// previewPatterns joins up to maxReportedFilterPatterns patterns for display, appending an
// ellipsis when more were detected.
func previewPatterns(patterns []string) string {
	if len(patterns) <= maxReportedFilterPatterns {
		return strings.Join(patterns, ", ")
	}
	return strings.Join(patterns[:maxReportedFilterPatterns], ", ") + ", …"
}

// ---------------- third question: which detectors to enable? ----------------

// promptDetectorPreset asks which detectors the generated rules should enable. The default is
// "unresolved + circular imports" — the recommended starting point. When stdin is not a terminal
// it returns that default.
func promptDetectorPreset() detectorPreset {
	presets := []detectorPreset{
		detectorsNone,
		detectorsUnresolvedOnly,
		detectorsUnresolvedCircular,
		detectorsScaffold,
		detectorsAll,
	}
	options := []string{
		"No detectors",
		"Unresolved imports only (a good start to confirm your setup and that resolution works)",
		"Unresolved + circular imports (circular usually works out of the box once resolution is fine)",
		"Circular + unresolved enabled, other detectors listed but disabled",
		"All detectors enabled (likely surfaces issues to fix and needs manual config adjustment)",
	}

	const defaultIndex = 2 // unresolved + circular imports
	if !stdinIsInteractive() {
		return presets[defaultIndex]
	}
	idx, _, err := selectOne(os.Stdin, os.Stdout, "Which detectors should the config enable?", options, defaultIndex)
	if err != nil {
		return presets[defaultIndex]
	}
	return presets[idx]
}

// ---------------- rule builders ----------------

// hasPackageJson reports whether dir contains a package.json file.
func hasPackageJson(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "package.json"))
	return err == nil
}

// detectorPreset selects which detectors a generated rule enables.
type detectorPreset int

const (
	detectorsNone               detectorPreset = iota // no detectors
	detectorsUnresolvedOnly                           // only unresolved imports
	detectorsUnresolvedCircular                       // unresolved + circular imports
	detectorsScaffold                                 // circular + unresolved enabled, other detectors listed but disabled
	detectorsAll                                      // all detectors listed and enabled
)

// applyDetectorPreset sets a rule's detection fields according to preset.
func applyDetectorPreset(rule *config.Rule, preset detectorPreset) {
	if preset == detectorsNone {
		return
	}
	// Unresolved imports is enabled by every non-empty preset.
	rule.UnresolvedImportsDetections = []*config.UnresolvedImportsOptions{{Enabled: true}}
	// Circular imports is enabled by everything except the unresolved-only preset.
	if preset != detectorsUnresolvedOnly {
		rule.CircularImportsDetections = []*config.CircularImportsOptions{{Enabled: true, IgnoreTypeImports: false}}
	}
	// The remaining detectors are only listed for the scaffold (disabled) and all (enabled) presets.
	// Detectors that need extra config to do anything (restricted imports/importers, import
	// conventions, module boundaries) are intentionally left out.
	if preset == detectorsScaffold || preset == detectorsAll {
		on := preset == detectorsAll
		rule.OrphanFilesDetections = []*config.OrphanFilesOptions{{Enabled: on}}
		rule.UnusedNodeModulesDetections = []*config.UnusedNodeModulesOptions{{Enabled: on}}
		rule.MissingNodeModulesDetections = []*config.MissingNodeModulesOptions{{Enabled: on}}
		rule.UnusedExportsDetections = []*config.UnusedExportsOptions{{Enabled: on}}
		rule.DevDepsUsageOnProdDetections = []*config.RestrictedDevDependenciesUsageOptions{{Enabled: on}}
	}
}

// makePackageRule builds a per-package rule (no module boundaries) for the given detector preset,
// used for monorepo workspace packages, standalone subdirectory packages, and monorepo sub-package
// configs.
func makePackageRule(path string, preset detectorPreset) config.Rule {
	rule := config.Rule{Path: path}
	applyDetectorPreset(&rule, preset)
	return rule
}

// makeSrcRootRule builds the root rule for a plain (non-monorepo) single-package project: the
// selected detectors plus an exemplary src/**/* module boundary.
func makeSrcRootRule(preset detectorPreset) config.Rule {
	rule := makePackageRule(".", preset)
	rule.ModuleBoundaries = []config.BoundaryRule{{
		Name:    "src",
		Pattern: "src/**/*",
		Allow:   []string{"src/**/*"},
	}}
	return rule
}

// makeMonorepoRootRule builds the root rule used at a monorepo workspace root: an exemplary
// packages/**/* module boundary and nothing else (per-package checks run on package rules).
func makeMonorepoRootRule() config.Rule {
	return config.Rule{
		Path: ".",
		ModuleBoundaries: []config.BoundaryRule{{
			Name:    "packages",
			Pattern: "packages/**/*",
			Allow:   []string{"packages/**/*"},
		}},
		OrphanFilesDetections:   []*config.OrphanFilesOptions{{Enabled: false}},
		UnusedExportsDetections: []*config.UnusedExportsOptions{{Enabled: false}},
	}
}

// mapValues returns the values of m in unspecified order.
func mapValues[K comparable, V any](m map[K]V) []V {
	out := make([]V, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// packageDirsToSortedRelPaths converts internal-form absolute package directories to
// cwd-relative, forward-slash paths, dropping the root itself, and returns them sorted.
func packageDirsToSortedRelPaths(cwd string, packageDirs []string) []string {
	relPaths := make([]string, 0, len(packageDirs))
	for _, dir := range packageDirs {
		relPath, err := filepath.Rel(cwd, pathutil.DenormalizePathForOS(dir))
		if err != nil {
			continue
		}
		relPath = filepath.ToSlash(relPath)
		if relPath == "." || relPath == "" {
			continue
		}
		relPaths = append(relPaths, relPath)
	}
	slices.Sort(relPaths)
	return relPaths
}

// ---------------- reporting ----------------

// printInitConfigResults prints the results of config initialization
func printInitConfigResults(result *initConfigResult) {
	fmt.Printf("✅ Created .rev-dep.config.jsonc at %s\n", result.configPath)

	fmt.Println()
	switch {
	case result.createdForMonorepoSubPackage:
		fmt.Printf("⚠️  Created config for monorepo sub-package. This file targets the current package only.\n")
	case result.isMonorepo:
		if len(result.workspacePackagePaths) > 0 {
			fmt.Printf("📦 Monorepo detected: discovered %d workspace %s and created a rule for each:\n", len(result.workspacePackagePaths), packagesWord(len(result.workspacePackagePaths)))
			for _, relPath := range result.workspacePackagePaths {
				fmt.Printf("   - %s\n", relPath)
			}
		} else {
			fmt.Printf("📦 Monorepo detected: no workspace packages found.\n")
		}
	case result.rootHasPackageJson:
		fmt.Printf("📁 Created a rule for the root package.\n")
	case len(result.standalonePackagePaths) == 0:
		fmt.Printf("📁 No package.json found; created a single rule for the root directory.\n")
	default:
		fmt.Printf("📁 No root package.json found; created rules for standalone packages only.\n")
	}

	// Separate section for standalone packages discovered in subdirectories.
	if len(result.standalonePackagePaths) > 0 {
		fmt.Println()
		fmt.Printf("🧩 Discovered %d standalone %s in subdirectories (not part of a monorepo) and created a rule for each:\n", len(result.standalonePackagePaths), packagesWord(len(result.standalonePackagePaths)))
		for _, relPath := range result.standalonePackagePaths {
			fmt.Printf("   - %s\n", relPath)
		}
	}

	if result.entryPointsDetected {
		fmt.Println()
		fmt.Printf("🔎 Auto-detected entry points for %d %s (production/development classified by path).\n", result.entryPointPackageCount, packagesWord(result.entryPointPackageCount))
	}

	fmt.Println()
	fmt.Println("Adjust rules to make them relevant to your project setup.")

	integrationGuide := "https://rev-dep.com/init/single-workspace"
	if result.isMonorepo {
		integrationGuide = "https://rev-dep.com/init/monorepo"
	}

	fmt.Println()
	fmt.Printf("📖 Integration guide: %s\n", integrationGuide)
	fmt.Printf("🛟  Troubleshooting: https://rev-dep.com/troubleshooting\n\n")
}
