package cli

import (
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

// ---------------- per-package entry-point detection ----------------

// detectPackageEntryPoints analyzes the package rooted at pkgDir (absolute, standardised path),
// finds its entry points (files with no importers within the package), classifies each as
// production, development, or ignored by path heuristics, and folds each set into the minimal
// list of package-relative glob/file patterns. Returns (prodPatterns, devPatterns, ignorePatterns).
func detectPackageEntryPoints(pkgDir string) (prod []string, dev []string, ignore []string) {
	tree, _, _ := resolve.GetMinimalDepsTreeForCwd(
		pkgDir,
		false,                               // ignoreTypeImports
		nil,                                 // excludeFiles
		nil,                                 // includeFiles
		nil,                                 // upfrontFilesList (empty -> scan the dir)
		"",                                  // packageJson (auto: pkgDir/package.json)
		"",                                  // tsconfigJson (auto: pkgDir/tsconfig.json; "" avoids os.Exit on missing)
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
	var dtsFiles []string
	universe := make([]string, 0, len(allFiles))
	for _, rel := range allFiles {
		if isDeclarationFile(rel) && !hasDirSegment(rel, ignoreEntryDirNames) {
			dtsFiles = append(dtsFiles, rel)
			continue
		}
		universe = append(universe, rel)
	}

	// Bucket every remaining file. Files inside a dev/ignore directory (tests, scripts, fixtures,
	// snapshots, ...) belong to that bucket WHOLESALE — regardless of whether they are entry
	// points — so the whole directory can fold into one glob even when its files import each other.
	// Files outside such directories are only relevant when they are actual entry points (no
	// importers): production by default, or development by filename marker (e.g. foo.test.ts).
	prodSet := map[string]bool{}
	devSet := map[string]bool{}
	ignoreSet := map[string]bool{}
	for _, rel := range universe {
		switch {
		case hasDirSegment(rel, ignoreEntryDirNames):
			ignoreSet[rel] = true
		case hasDirSegment(rel, devEntryDirNames):
			devSet[rel] = true
		case !entrySet[rel]:
			// internal module in a normal directory: not an entry point, so not classified.
		case hasDevFileMarker(rel):
			devSet[rel] = true
		default:
			prodSet[rel] = true
		}
	}

	prod = foldEntryPatterns(universe, prodSet)
	dev = foldEntryPatterns(universe, devSet)
	ignore = foldEntryPatterns(universe, ignoreSet)

	dev = append(dev, declarationFilePatterns(dtsFiles)...)
	slices.Sort(dev)
	dev = slices.Compact(dev)

	return prod, dev, ignore
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

// foldEntryPatterns collapses the entry paths in entrySet into the minimal set of package-relative
// patterns. A directory whose every analyzed file (recursively) is in entrySet is folded into a
// single "dir/**" glob; otherwise its entry files are listed individually and its subdirectories
// are folded independently. allFiles is the universe of analyzed files (all package-relative), so
// "every file in this directory is an entry point" is decided against real siblings, not the fs.
// The package root is never folded into a bare "**" — it is always expanded one level.
func foldEntryPatterns(allFiles []string, entrySet map[string]bool) []string {
	if len(entrySet) == 0 {
		return nil
	}

	t := &entryFileTree{
		directFiles: map[string][]string{},
		childDirs:   map[string][]string{},
		seenDir:     map[string]bool{"": true},
		entrySet:    entrySet,
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

	patterns := t.expandDir("")
	slices.Sort(patterns)
	return slices.Compact(patterns)
}

type entryFileTree struct {
	directFiles map[string][]string // dir -> files directly in it (package-relative)
	childDirs   map[string][]string // dir -> child directory paths
	seenDir     map[string]bool
	entrySet    map[string]bool
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
	patterns []string
	covered  bool // every analyzed file under this dir is an entry point
	hasEntry bool // this dir's subtree contains at least one entry point
}

// foldDir folds a non-root directory: a fully-covered directory becomes one "dir/**" glob,
// otherwise it expands into its entry files plus its subdirectories' patterns.
func (t *entryFileTree) foldDir(dir string) entryFoldResult {
	files := t.directFiles[dir]
	children := t.childDirs[dir]

	allFilesEntries := true
	anyDirectEntry := false
	for _, f := range files {
		if t.entrySet[f] {
			anyDirectEntry = true
		} else {
			allFilesEntries = false
		}
	}

	childResults := make([]entryFoldResult, len(children))
	allChildrenCovered := true
	anyChildEntry := false
	for i, child := range children {
		childResults[i] = t.foldDir(child)
		if !childResults[i].covered {
			allChildrenCovered = false
		}
		if childResults[i].hasEntry {
			anyChildEntry = true
		}
	}

	hasEntry := anyDirectEntry || anyChildEntry
	if hasEntry && allFilesEntries && allChildrenCovered {
		return entryFoldResult{patterns: []string{dir + "/**"}, covered: true, hasEntry: true}
	}
	return entryFoldResult{patterns: t.expandFrom(files, children, childResults), covered: false, hasEntry: hasEntry}
}

// expandDir expands a directory (used for the package root, which is never folded whole).
func (t *entryFileTree) expandDir(dir string) []string {
	children := t.childDirs[dir]
	childResults := make([]entryFoldResult, len(children))
	for i, child := range children {
		childResults[i] = t.foldDir(child)
	}
	return t.expandFrom(t.directFiles[dir], children, childResults)
}

// expandFrom emits the directory's own entry files plus each subdirectory's patterns.
func (t *entryFileTree) expandFrom(files []string, children []string, childResults []entryFoldResult) []string {
	var out []string
	sortedFiles := append([]string(nil), files...)
	slices.Sort(sortedFiles)
	for _, f := range sortedFiles {
		if t.entrySet[f] {
			out = append(out, f)
		}
	}
	for i := range children {
		if childResults[i].hasEntry {
			out = append(out, childResults[i].patterns...)
		}
	}
	return out
}
