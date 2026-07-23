package cli

import (
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	"rev-dep-go/internal/graph"
	"rev-dep-go/internal/model"
	"rev-dep-go/internal/pathutil"
	"rev-dep-go/internal/resolve"
)

// ---------------- second question: auto-detect entry points? ----------------

// promptAutoDetectEntryPoints asks whether config init should analyze each package and populate
// its prodEntryPoints / devEntryPoints. "Yes" is the first option and the interactive default
// (pressing Enter detects). When stdin is not a terminal it returns false, since running a
// per-package dependency analysis unprompted in a scripted run would be a surprising cost.
func promptAutoDetectEntryPoints() bool {
	if !stdinIsInteractive() {
		return false
	}
	options := []string{
		"Yes — analyze each package and fill in production/development entry points",
		"No — leave entry points empty",
	}
	idx, _, err := selectOne(os.Stdin, os.Stdout, "Auto-detect entry points for each package?", options, 0)
	if err != nil {
		return false
	}
	return idx == 0
}

// ---------------- third question: how aggressively to fold near-covered directories? ----------------

// foldThresholdAsker chooses the entry-point fold coverage threshold given the options detected
// across all packages. It returns a percentage from foldThresholdLadder (100 = strict, no folding
// of partially-covered directories).
type foldThresholdAsker func(options []foldThresholdOption) int

// strictFoldThreshold is the non-interactive default: never fold partially-covered directories.
func strictFoldThreshold([]foldThresholdOption) int { return 100 }

// maxDisplayedFoldDirs caps how many example directories are listed for each threshold option.
const maxDisplayedFoldDirs = 3

// promptFoldThreshold asks how aggressively to fold near-fully-covered directories into a single
// "dir/**" glob. The first option keeps the strict rule (100%); each further option folds
// directories down to a lower coverage, previewing up to a few directories it would newly collapse.
func promptFoldThreshold(options []foldThresholdOption) int {
	if !stdinIsInteractive() || len(options) == 0 {
		return 100
	}
	labels := []string{"Fold only fully-covered directories (strict, 100%)"}
	for _, option := range options {
		labels = append(labels, foldThresholdOptionLabel(option))
	}
	question := "Some directories contain files other than entry points. " +
		"Fold each into a single \"dir/**\" glob anyway?\n" +
		"Folding keeps the config short (one glob per directory).\n" +
		"Not folding lists every entry point individually — more entries, more verbose."
	idx, _, err := selectOne(os.Stdin, os.Stdout, question, labels, 0)
	if err != nil || idx == 0 {
		return 100
	}
	return options[idx-1].threshold
}

// foldThresholdOptionLabel renders a threshold choice: its coverage cutoff, how many directories it
// would newly fold, and up to maxDisplayedFoldDirs examples with their coverage.
func foldThresholdOptionLabel(option foldThresholdOption) string {
	examples := make([]string, 0, maxDisplayedFoldDirs)
	for _, dir := range option.newDirs {
		if len(examples) == maxDisplayedFoldDirs {
			break
		}
		examples = append(examples, fmt.Sprintf("%s/** (%d%%)", dir.dir, dir.coverage))
	}
	more := ""
	if len(option.newDirs) > maxDisplayedFoldDirs {
		more = fmt.Sprintf(", +%d more", len(option.newDirs)-maxDisplayedFoldDirs)
	}
	return fmt.Sprintf("Fold directories ≥%d%% covered — %d dir(s): %s%s",
		option.threshold, len(option.newDirs), strings.Join(examples, ", "), more)
}

// ---------------- per-package entry-point detection ----------------

// packageEntryAnalysis is the folding-ready result of analyzing a package: the universe of
// analyzed files partitioned into production / development / ignored entry-point sets (plus the
// declaration files handled separately). It is produced once by analyzePackageEntryPoints and then
// folded into glob patterns at a chosen coverage threshold by foldAnalysis — the split lets config
// init pick a fold threshold interactively after seeing every package's near-covered directories.
type packageEntryAnalysis struct {
	universe  []string
	prodSet   map[string]bool
	devSet    map[string]bool
	ignoreSet map[string]bool
	dtsFiles  []string
}

// analyzePackageEntryPoints analyzes the package rooted at pkgDir (absolute, standardised path),
// finds its entry points (files with no importers within the package), and classifies each file as
// production, development, or ignored by path heuristics — without folding into patterns yet.
func analyzePackageEntryPoints(pkgDir string) packageEntryAnalysis {
	tree, _, _ := resolve.GetMinimalDepsTreeForCwd(
		pkgDir,
		false,                               // ignoreTypeImports
		nil,                                 // excludeFiles
		nil,                                 // includeFiles
		nil,                                 // upfrontFilesList (empty -> scan the dir)
		"",                                  // tsconfigJson (auto: pkgDir/tsconfig.json)
		nil,                                 // conditionNames
		model.FollowMonorepoPackagesValue{}, // don't traverse into sibling packages
		nil,                                 // customAssetExtensions
		model.NodeModulesMatchingStrategyCwdResolver,
	)

	base := pathutil.StandardiseDirPathInternal(pathutil.NormalizePathForInternal(pkgDir))
	relTo := func(abs string) string {
		return strings.TrimPrefix(pathutil.NormalizePathForInternal(abs), base)
	}

	// The universe of files is every analyzed file (each tree key). Entry points are the subset
	// with no incoming edges.
	allFiles := make([]string, 0, len(tree))
	for filePath := range tree {
		allFiles = append(allFiles, relTo(filePath))
	}

	entrySet := map[string]bool{}
	for _, entryAbs := range graph.GetEntryPoints(tree, nil, nil, pkgDir) {
		entrySet[relTo(entryAbs)] = true
	}

	// Declaration files (.d.ts) are dev entry points handled separately: several of them collapse
	// to one **/*.d.ts glob. Pull them out of the folding universe first (unless they live in an
	// ignore directory, where they belong to that directory's fold).
	analysis := packageEntryAnalysis{
		prodSet:   map[string]bool{},
		devSet:    map[string]bool{},
		ignoreSet: map[string]bool{},
	}
	for _, rel := range allFiles {
		if isDeclarationFile(rel) && !hasDirSegment(rel, ignoreEntryDirNames) {
			analysis.dtsFiles = append(analysis.dtsFiles, rel)
			continue
		}
		analysis.universe = append(analysis.universe, rel)
	}

	// Bucket every remaining file. Files inside a dev/ignore directory (tests, scripts, fixtures,
	// snapshots, ...) belong to that bucket WHOLESALE — regardless of whether they are entry
	// points — so the whole directory can fold into one glob even when its files import each other.
	// Files outside such directories are only relevant when they are actual entry points (no
	// importers): production by default, or development by filename marker (e.g. foo.test.ts).
	for _, rel := range analysis.universe {
		switch {
		case hasDirSegment(rel, ignoreEntryDirNames):
			analysis.ignoreSet[rel] = true
		case hasDirSegment(rel, devEntryDirNames):
			analysis.devSet[rel] = true
		case !entrySet[rel]:
			// internal module in a normal directory: not an entry point, so not classified.
		case hasDevFileMarker(rel):
			analysis.devSet[rel] = true
		default:
			analysis.prodSet[rel] = true
		}
	}
	return analysis
}

// foldAnalysis folds an analyzed package into (prod, dev, ignore) glob patterns at the given
// coverage threshold (percent). At threshold 100 a directory folds to "dir/**" only when every
// analyzed file under it is an entry point of that set (the strict, always-safe rule); a lower
// threshold folds directories that are only near-fully covered — e.g. a Next.js pages/ dir where a
// few files are imported by siblings and so are not entry points. Each set's fold blocks on the
// other two sets so a directory mixing, say, prod and dev files never folds one set's glob over the
// other's files.
func foldAnalysis(analysis packageEntryAnalysis, threshold int) (prod, dev, ignore []string) {
	prod = foldEntryPatterns(analysis.universe, analysis.prodSet, unionSets(analysis.devSet, analysis.ignoreSet), threshold)
	dev = foldEntryPatterns(analysis.universe, analysis.devSet, unionSets(analysis.prodSet, analysis.ignoreSet), threshold)
	ignore = foldEntryPatterns(analysis.universe, analysis.ignoreSet, unionSets(analysis.prodSet, analysis.devSet), threshold)

	dev = append(dev, declarationFilePatterns(analysis.dtsFiles)...)
	dev = collapseEntryPatterns(dev, devEntryDirNames, true)
	// Ignore entries collapse recognized directory globs (e.g. many "…/__snapshots__/**" ->
	// "**/__snapshots__/**") but NOT file suffixes: a stray ".test.ts" inside a fixtures dir must
	// not broaden into "**/*.test.ts" and swallow every real test file into ignoreEntryPoints.
	ignore = collapseEntryPatterns(ignore, ignoreEntryDirNames, false)

	return prod, dev, ignore
}

// detectPackageEntryPoints analyzes the package rooted at pkgDir and folds its entry points into
// the minimal list of package-relative glob/file patterns using the strict (100%) coverage rule.
func detectPackageEntryPoints(pkgDir string) (prod []string, dev []string, ignore []string) {
	return foldAnalysis(analyzePackageEntryPoints(pkgDir), 100)
}

// unionSets returns a new set containing every key from both input sets.
func unionSets(first, second map[string]bool) map[string]bool {
	union := make(map[string]bool, len(first)+len(second))
	for key := range first {
		union[key] = true
	}
	for key := range second {
		union[key] = true
	}
	return union
}

// entryCollapseThreshold is the minimum number of entry patterns sharing a compound file suffix
// or a recognized directory-leaf name before they are collapsed into one glob (mirrors the .d.ts
// rule: a lone occurrence is listed as-is, two or more collapse).
const entryCollapseThreshold = 2

// collapseEntryPatterns collapses a folded entry-pattern list on two axes so scattered entries do
// not produce one line each:
//
//   - Literal files sharing a compound suffix (the last two dot-separated filename segments, e.g.
//     ".test.ts", ".stories.tsx", ".config.js") collapse into one "**/*<suffix>" glob. This is
//     suffix-general — it is not limited to the classifier's marker list — since these files are
//     already dev/ignore entry points and any shared compound suffix names a family of them.
//   - Folded "dir/**" globs whose leaf directory is a recognized dev/ignore directory name (mocks,
//     __tests__, snapshots, ...) collapse into one "**/<leaf>/**" glob, so many scattered
//     "…/mocks/**" entries become a single "**/mocks/**".
//
// Existing broad globs (e.g. "**/*.d.ts"), plain files without a compound suffix, and "dir/**"
// globs whose leaf is not a recognized directory name pass through untouched. A suffix or leaf
// carried by a single entry is left as-is. collapseFiles gates the file-suffix axis (the directory
// axis always applies): callers pass false where broadening files would be unsafe (see ignore).
func collapseEntryPatterns(patterns []string, dirNames map[string]bool, collapseFiles bool) []string {
	fileGroups := map[string][]string{} // compound suffix -> literal file patterns
	dirGroups := map[string][]string{}  // recognized leaf dir name -> "dir/**" globs
	var out []string
	for _, p := range patterns {
		if leaf, ok := recognizedDirGlobLeaf(p, dirNames); ok {
			dirGroups[leaf] = append(dirGroups[leaf], p)
			continue
		}
		if strings.Contains(p, "*") {
			out = append(out, p) // some other glob (e.g. **/*.d.ts)
			continue
		}
		if suffix, ok := compoundFileSuffix(p); ok && collapseFiles {
			fileGroups[suffix] = append(fileGroups[suffix], p)
			continue
		}
		out = append(out, p)
	}
	for suffix, files := range fileGroups {
		if len(files) >= entryCollapseThreshold {
			out = append(out, "**/*"+suffix)
		} else {
			out = append(out, files...)
		}
	}
	for leaf, globs := range dirGroups {
		if len(globs) >= entryCollapseThreshold {
			out = append(out, "**/"+leaf+"/**")
		} else {
			out = append(out, globs...)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// compoundFileSuffix returns the collapse suffix of a file — the last two dot-separated segments
// of its name (e.g. "app/foo/bar.server.test.ts" -> ".test.ts", "x/Card.stories.tsx" ->
// ".stories.tsx") — reporting false when the name has no compound extension (fewer than two dots,
// e.g. "utils.ts"), since a bare "name.ext" would collapse into an over-broad "**/*.ext".
func compoundFileSuffix(relPath string) (string, bool) {
	parts := strings.Split(path.Base(relPath), ".")
	if len(parts) < 3 {
		return "", false
	}
	return "." + parts[len(parts)-2] + "." + parts[len(parts)-1], true
}

// recognizedDirGlobLeaf reports the leaf directory name of a "dir/**" glob when that leaf is one of
// the recognized dev/ignore directory names (so "app/broadcast/mocks/**" -> "mocks"). A bare
// "**"/"leaf/**" with an unrecognized or empty leaf, or any non-"/**" pattern, returns false so it
// is not collapsed into a package-wide "**/<leaf>/**".
func recognizedDirGlobLeaf(pattern string, dirNames map[string]bool) (string, bool) {
	rest, ok := strings.CutSuffix(pattern, "/**")
	if !ok || rest == "" || strings.Contains(rest, "*") {
		return "", false
	}
	leaf := path.Base(rest)
	if !dirNames[strings.ToLower(leaf)] {
		return "", false
	}
	return leaf, true
}

// ---------------- interactive fold threshold ----------------

// foldThresholdLadder is the descending list of coverage thresholds offered during config init. 100
// is the strict rule (every file must be an entry point); lower values also fold directories that
// are only near-fully covered.
var foldThresholdLadder = []int{100, 95, 90, 85, 80}

// foldThresholdOption is one selectable threshold below 100 together with the directories it would
// newly fold — those not already folded by the next-stricter threshold.
type foldThresholdOption struct {
	threshold int
	newDirs   []coveredDir // display paths, sorted by descending coverage then path
}

// analyzedPackage pairs a package's analysis with the rule path used to display its directories.
type analyzedPackage struct {
	rulePath string
	analysis packageEntryAnalysis
}

// foldThresholdOptions computes, for each threshold below 100, the directories it would newly fold
// across all analyzed packages (deduplicated against stricter thresholds). A threshold that unlocks
// no directory beyond the one above it is omitted, so choosing it would change nothing. Returns nil
// when no directory folds below 100 — i.e. there is nothing to ask the user.
func foldThresholdOptions(pkgs []analyzedPackage) []foldThresholdOption {
	foldedByThreshold := make(map[int]map[string]int, len(foldThresholdLadder)) // threshold -> display path -> coverage
	for _, threshold := range foldThresholdLadder {
		foldedHere := map[string]int{}
		for _, pkg := range pkgs {
			for _, folded := range allFoldableDirs(pkg.analysis, threshold) {
				displayPath := joinPkgDir(pkg.rulePath, folded.dir)
				if existing, seen := foldedHere[displayPath]; !seen || folded.coverage > existing {
					foldedHere[displayPath] = folded.coverage
				}
			}
		}
		foldedByThreshold[threshold] = foldedHere
	}

	var options []foldThresholdOption
	for i := 1; i < len(foldThresholdLadder); i++ {
		threshold, stricter := foldThresholdLadder[i], foldThresholdLadder[i-1]
		var newDirs []coveredDir
		for displayPath, coverage := range foldedByThreshold[threshold] {
			if _, foldedByStricter := foldedByThreshold[stricter][displayPath]; !foldedByStricter {
				newDirs = append(newDirs, coveredDir{dir: displayPath, coverage: coverage})
			}
		}
		if len(newDirs) == 0 {
			continue
		}
		slices.SortFunc(newDirs, func(first, second coveredDir) int {
			if first.coverage != second.coverage {
				return second.coverage - first.coverage // higher coverage first
			}
			return strings.Compare(first.dir, second.dir)
		})
		options = append(options, foldThresholdOption{threshold: threshold, newDirs: newDirs})
	}
	return options
}

// allFoldableDirs returns every directory that folds at the given threshold across the package's
// production, development and ignore sets.
func allFoldableDirs(analysis packageEntryAnalysis, threshold int) []coveredDir {
	var folded []coveredDir
	folded = append(folded, foldableDirs(analysis.universe, analysis.prodSet, unionSets(analysis.devSet, analysis.ignoreSet), threshold)...)
	folded = append(folded, foldableDirs(analysis.universe, analysis.devSet, unionSets(analysis.prodSet, analysis.ignoreSet), threshold)...)
	folded = append(folded, foldableDirs(analysis.universe, analysis.ignoreSet, unionSets(analysis.prodSet, analysis.devSet), threshold)...)
	return folded
}

// joinPkgDir builds a display path for a package-relative directory, prefixing the package path
// unless it is the current directory.
func joinPkgDir(rulePath, dir string) string {
	switch {
	case rulePath == "" || rulePath == ".":
		return dir
	case dir == "":
		return rulePath
	default:
		return rulePath + "/" + dir
	}
}

// ---------------- entry-point classification ----------------

// entryClass is how an entry point is bucketed into the generated config.
type entryClass int

const (
	entryProd   entryClass = iota // production entry point (the default)
	entryDev                      // development entry point (tests, scripts, examples, ...)
	entryIgnore                   // ignored entry point (fixtures — not real entry points)
)

// ignoreEntryDirNames are lowercase directory-name segments that mark a file as an *ignored*
// entry point: fixtures and snapshots are generated/test data, not real entry points, so they go
// to ignoreEntryPoints.
var ignoreEntryDirNames = map[string]bool{
	"fixtures": true, "__fixtures__": true,
	"snapshots": true, "__snapshots__": true,
}

// devEntryDirNames are lowercase directory-name segments that mark a file as a development entry
// point (tests, scripts, examples, etc.) rather than production code.
var devEntryDirNames = map[string]bool{
	"test": true, "tests": true, "__tests__": true,
	"spec": true, "specs": true, "__specs__": true,
	"scripts": true, "script": true,
	"e2e": true, "cypress": true,
	"benchmark": true, "benchmarks": true, "__benchmarks__": true,
	"mocks": true, "__mocks__": true,
	"stories": true, ".storybook": true,
	"examples": true, "example": true, "demo": true, "demos": true,
}

// devEntryFileMarkers are filename infixes that mark a file as a development entry point.
var devEntryFileMarkers = []string{".test.", ".spec.", ".stories.", ".bench.", ".e2e.", ".cy.", ".config."}

// isDeclarationFile reports whether relPath is a TypeScript declaration (.d.ts) file. These are
// development entry points and, when a package has several, get collapsed to one **/*.d.ts glob.
func isDeclarationFile(relPath string) bool {
	return strings.HasSuffix(strings.ToLower(path.Base(relPath)), ".d.ts")
}

// declarationFilePatterns turns a package's declaration files into dev patterns: a single file is
// listed as-is, but two or more collapse to one **/*.d.ts glob.
func declarationFilePatterns(dtsFiles []string) []string {
	switch len(dtsFiles) {
	case 0:
		return nil
	case 1:
		return []string{dtsFiles[0]}
	default:
		return []string{"**/*.d.ts"}
	}
}

// hasDirSegment reports whether any path segment of relPath is in names (case-insensitive).
func hasDirSegment(relPath string, names map[string]bool) bool {
	for _, segment := range strings.Split(relPath, "/") {
		if names[strings.ToLower(segment)] {
			return true
		}
	}
	return false
}

// hasDevFileMarker reports whether relPath's filename marks it as a development entry point.
func hasDevFileMarker(relPath string) bool {
	lowerBase := strings.ToLower(path.Base(relPath))
	for _, marker := range devEntryFileMarkers {
		if strings.Contains(lowerBase, marker) {
			return true
		}
	}
	return false
}

// classifyEntryPoint buckets a package-relative entry-point path. Ignore (fixtures/snapshots)
// takes precedence over dev, and production is the default. Classification is purely path-based.
func classifyEntryPoint(relPath string) entryClass {
	if hasDirSegment(relPath, ignoreEntryDirNames) {
		return entryIgnore
	}
	if isDeclarationFile(relPath) || hasDevFileMarker(relPath) || hasDirSegment(relPath, devEntryDirNames) {
		return entryDev
	}
	return entryProd
}

// ---------------- glob folding ----------------

// coveredDir is a directory that folded to "dir/**" together with its entry-point coverage —
// the percentage of analyzed files under it that are entry points of the folded set.
type coveredDir struct {
	dir      string
	coverage int
}

// foldEntryPatterns collapses the entry paths in entrySet into the minimal set of package-relative
// patterns. A directory folds into a single "dir/**" glob when at least `threshold` percent of the
// analyzed files under it are in entrySet (100 = every file, the strict rule); otherwise its entry
// files are listed individually and its subdirectories are folded independently. A directory is
// never folded when its subtree contains a `blocked` file — a file belonging to a different entry
// set — so one set's glob never claims another set's files. allFiles is the universe of analyzed
// files (all package-relative), so coverage is decided against real siblings, not the fs. The
// package root is never folded into a bare "**" — it is always expanded one level.
func foldEntryPatterns(allFiles []string, entrySet, blocked map[string]bool, threshold int) []string {
	if len(entrySet) == 0 {
		return nil
	}
	t := newEntryFileTree(allFiles, entrySet, blocked, threshold)
	patterns, _ := t.expandRoot()
	slices.Sort(patterns)
	return slices.Compact(patterns)
}

// foldableDirs returns the directories that would fold to "dir/**" at the given threshold, each
// with its coverage. It runs the same fold as foldEntryPatterns but reports the folded directories
// instead of the pattern list, so config init can preview which near-covered directories a
// threshold would collapse.
func foldableDirs(allFiles []string, entrySet, blocked map[string]bool, threshold int) []coveredDir {
	if len(entrySet) == 0 {
		return nil
	}
	t := newEntryFileTree(allFiles, entrySet, blocked, threshold)
	_, folded := t.expandRoot()
	return folded
}

func newEntryFileTree(allFiles []string, entrySet, blocked map[string]bool, threshold int) *entryFileTree {
	t := &entryFileTree{
		directFiles: map[string][]string{},
		childDirs:   map[string][]string{},
		seenDir:     map[string]bool{"": true},
		entrySet:    entrySet,
		blocked:     blocked,
		threshold:   threshold,
	}
	for _, f := range allFiles {
		f = strings.TrimPrefix(f, "/")
		dir := path.Dir(f)
		if dir == "." {
			dir = ""
		}
		t.directFiles[dir] = append(t.directFiles[dir], f)
		t.registerDirChain(dir)
	}
	for dir := range t.childDirs {
		slices.Sort(t.childDirs[dir])
	}
	return t
}

type entryFileTree struct {
	directFiles map[string][]string // dir -> files directly in it (package-relative)
	childDirs   map[string][]string // dir -> child directory paths
	seenDir     map[string]bool
	entrySet    map[string]bool
	blocked     map[string]bool
	threshold   int
}

// registerDirChain records dir and every ancestor as a child of its parent.
func (t *entryFileTree) registerDirChain(dir string) {
	for dir != "" && !t.seenDir[dir] {
		t.seenDir[dir] = true
		parent := path.Dir(dir)
		if parent == "." {
			parent = ""
		}
		t.childDirs[parent] = append(t.childDirs[parent], dir)
		dir = parent
	}
}

type entryFoldResult struct {
	patterns   []string
	entryCount int  // entry-set files in this subtree
	fileCount  int  // entry + tolerable (non-entry, non-blocked) files in this subtree
	blocked    bool // this subtree contains a file from a different entry set
	hasEntry   bool // this subtree contains at least one entry point
	foldedDirs []coveredDir
}

// foldDir folds a non-root directory: a sufficiently-covered, block-free directory becomes one
// "dir/**" glob, otherwise it expands into its entry files plus its subdirectories' patterns.
func (t *entryFileTree) foldDir(dir string) entryFoldResult {
	files := t.directFiles[dir]
	children := t.childDirs[dir]

	entryCount, fileCount := 0, 0
	blockedHere := false
	for _, f := range files {
		switch {
		case t.entrySet[f]:
			entryCount++
			fileCount++
		case t.blocked[f]:
			blockedHere = true
		default:
			fileCount++ // tolerable non-entry (internal module of this set's area)
		}
	}

	childResults := make([]entryFoldResult, len(children))
	for i, child := range children {
		childResult := t.foldDir(child)
		childResults[i] = childResult
		entryCount += childResult.entryCount
		fileCount += childResult.fileCount
		if childResult.blocked {
			blockedHere = true
		}
	}

	hasEntry := entryCount > 0
	if hasEntry && !blockedHere && entryCount*100 >= t.threshold*fileCount {
		coverage := entryCount * 100 / fileCount
		return entryFoldResult{
			patterns:   []string{dir + "/**"},
			entryCount: entryCount, fileCount: fileCount, hasEntry: true,
			foldedDirs: []coveredDir{{dir: dir, coverage: coverage}},
		}
	}
	patterns, folded := t.expandFrom(files, children, childResults)
	return entryFoldResult{
		patterns: patterns, entryCount: entryCount, fileCount: fileCount,
		blocked: blockedHere, hasEntry: hasEntry, foldedDirs: folded,
	}
}

// expandRoot expands the package root (never folded whole) and returns its patterns plus every
// directory that folded anywhere beneath it.
func (t *entryFileTree) expandRoot() (patterns []string, folded []coveredDir) {
	children := t.childDirs[""]
	childResults := make([]entryFoldResult, len(children))
	for i, child := range children {
		childResults[i] = t.foldDir(child)
	}
	return t.expandFrom(t.directFiles[""], children, childResults)
}

// expandFrom emits the directory's own entry files plus each subdirectory's patterns, and gathers
// the folded directories from its subdirectories.
func (t *entryFileTree) expandFrom(files []string, children []string, childResults []entryFoldResult) (patterns []string, folded []coveredDir) {
	sortedFiles := append([]string(nil), files...)
	slices.Sort(sortedFiles)
	for _, f := range sortedFiles {
		if t.entrySet[f] {
			patterns = append(patterns, f)
		}
	}
	for i := range children {
		if childResults[i].hasEntry {
			patterns = append(patterns, childResults[i].patterns...)
		}
		folded = append(folded, childResults[i].foldedDirs...)
	}
	return patterns, folded
}
