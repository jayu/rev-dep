package monorepo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/gobwas/glob"
	"github.com/tidwall/jsonc"
	"gopkg.in/yaml.v3"

	"rev-dep-go/internal/fs"
	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/pathutil"
)

type PackageJsonConfig struct {
	Name    string                 `json:"name"`
	Version string                 `json:"version"`
	Exports interface{}            `json:"exports"`
	Imports map[string]interface{} `json:"imports"`
	// We might need dependencies to check versions
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Main            string            `json:"main"`
	Module          string            `json:"module"`
}

type MonorepoContext struct {
	WorkspaceRoot       string
	PackageToPath       map[string]string
	PackageConfigCache  map[string]*PackageJsonConfig
	PackageExportsCache map[string]*PackageJsonExports
}

func containsGlobMeta(s string) bool {
	return strings.ContainsAny(s, "*?[]{}")
}

func getStaticPrefixBeforeGlob(pattern string) string {
	if pattern == "" {
		return ""
	}
	parts := strings.Split(pattern, "/")
	staticParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || containsGlobMeta(part) {
			break
		}
		staticParts = append(staticParts, part)
	}
	if len(staticParts) == 0 {
		return ""
	}
	return strings.Join(staticParts, "/")
}

func NewMonorepoContext(root string) *MonorepoContext {
	return &MonorepoContext{
		WorkspaceRoot:       root,
		PackageToPath:       make(map[string]string),
		PackageConfigCache:  make(map[string]*PackageJsonConfig),
		PackageExportsCache: make(map[string]*PackageJsonExports),
	}
}

func DetectMonorepo(cwd string) *MonorepoContext {
	currentDir := pathutil.NormalizePathForInternal(filepath.Clean(cwd))
	for {
		// pnpm workspace files: prefer singular pnpm-workspace.yaml/.yml over package.json
		pnpmFilenames := []string{"pnpm-workspace.yaml", "pnpm-workspace.yml"}
		for _, fname := range pnpmFilenames {
			pnpmWorkspacePath := filepath.Join(currentDir, fname)
			if _, err := os.Stat(pnpmWorkspacePath); err == nil {
				// Only treat as monorepo root if pnpm-workspace.* contains non-empty "packages"
				content, err := os.ReadFile(pnpmWorkspacePath)
				if err == nil {
					var pnpmWorkspace struct {
						Packages []string `yaml:"packages"`
					}
					if err := yaml.Unmarshal(content, &pnpmWorkspace); err == nil {
						if len(pnpmWorkspace.Packages) > 0 {
							return NewMonorepoContext(currentDir)
						}
					}
				}
			}
		}

		// Fallback: check package.json workspaces key
		pkgJsonPath := filepath.Join(currentDir, "package.json")
		if _, err := os.Stat(pkgJsonPath); err == nil {
			content, err := os.ReadFile(pkgJsonPath)
			if err == nil {
				var pkgJson map[string]interface{}
				if err := json.Unmarshal(jsonc.ToJSON(content), &pkgJson); err == nil {
					if hasValidWorkspaces(pkgJson) {
						return NewMonorepoContext(currentDir)
					}
				}
			}
		}

		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}
	return nil
}

// hasValidWorkspaces checks if a package.json has valid workspace configuration.
// Valid means: workspaces is a non-empty array OR an object with non-empty "packages" array
func hasValidWorkspaces(pkgJson map[string]interface{}) bool {
	workspaces, ok := pkgJson["workspaces"]
	if !ok {
		return false
	}

	// Check if workspaces is an array
	if list, ok := workspaces.([]interface{}); ok {
		return len(list) > 0
	}

	// Check if workspaces is an object with "packages" array
	if obj, ok := workspaces.(map[string]interface{}); ok {
		if packages, ok := obj["packages"].([]interface{}); ok {
			return len(packages) > 0
		}
	}

	return false
}

func (ctx *MonorepoContext) FindWorkspacePackages(excludeFilePatterns []globutil.GlobMatcher, includeFilePatterns []globutil.GlobMatcher) {
	// Always honor .gitignore when discovering workspace packages.
	gitIgnoreExcludePatterns := fs.FindAndProcessGitIgnoreFilesUpToRepoRoot(ctx.WorkspaceRoot)
	allExcludePatterns := append(append([]globutil.GlobMatcher{}, excludeFilePatterns...), gitIgnoreExcludePatterns...)
	includePrefixes := globutil.BuildIncludePrefixes(includeFilePatterns)

	// Prefer pnpm workspace file (supports plural/singular and .yml/.yaml), fall back to package.json workspaces
	var patterns []string
	// support both .yaml and .yml; only singular filename is documented, prefer it
	pnpmFilenames := []string{"pnpm-workspace.yaml", "pnpm-workspace.yml"}
	for _, fname := range pnpmFilenames {
		pnpmWorkspacePath := filepath.Join(pathutil.DenormalizePathForOS(ctx.WorkspaceRoot), fname)
		if pnpmContent, err := os.ReadFile(pnpmWorkspacePath); err == nil {
			var pnpmWorkspace struct {
				Packages []string `yaml:"packages"`
			}
			if err := yaml.Unmarshal(pnpmContent, &pnpmWorkspace); err == nil {
				if len(pnpmWorkspace.Packages) > 0 {
					patterns = append(patterns, pnpmWorkspace.Packages...)
					break
				}
			}
		}
	}

	if len(patterns) == 0 {
		pkgJsonPath := filepath.Join(pathutil.DenormalizePathForOS(ctx.WorkspaceRoot), "package.json")
		content, err := os.ReadFile(pkgJsonPath)
		if err == nil {
			var pkgJson struct {
				Workspaces interface{} `json:"workspaces"` // can be []string or { packages: []string }
			}
			if err := json.Unmarshal(jsonc.ToJSON(content), &pkgJson); err == nil {
				if list, ok := pkgJson.Workspaces.([]interface{}); ok {
					for _, v := range list {
						if s, ok := v.(string); ok {
							patterns = append(patterns, s)
						}
					}
				} else if obj, ok := pkgJson.Workspaces.(map[string]interface{}); ok {
					if packages, ok := obj["packages"].([]interface{}); ok {
						for _, v := range packages {
							if s, ok := v.(string); ok {
								patterns = append(patterns, s)
							}
						}
					}
				}
			}
		}
	}

	type positivePattern struct {
		basePath    string
		isDeep      bool
		isDir       bool
		isComplex   bool
		complexRoot string
		complexGlob glob.Glob
	}

	var positive []positivePattern
	type negativeMatcher struct {
		g glob.Glob
	}
	var negative []negativeMatcher

	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "!") {
			cleanP := strings.TrimPrefix(pattern, "!")
			cleanP = pathutil.NormalizeGlobPattern(cleanP)
			g, err := glob.Compile(cleanP, '/')
			if err == nil {
				negative = append(negative, negativeMatcher{g: g})
			}
			continue
		}

		if pattern == "*" {
			positive = append(positive, positivePattern{
				basePath: "",
				isDir:    true,
			})
			continue
		}

		if pattern == "**" || pattern == "**/*" {
			positive = append(positive, positivePattern{
				basePath: "",
				isDeep:   true,
			})
			continue
		}

		// Pattern handling strategy:
		// 1) Fast-path common workspace shapes (*, path/*, path/**, path/**/*)
		//    to avoid per-candidate glob matching overhead.
		// 2) Fallback to compiled glob for complex patterns (for example path/**/other/*),
		//    scanning from the static prefix before the first glob segment.
		if strings.HasSuffix(pattern, "/**/*") && !containsGlobMeta(strings.TrimSuffix(pattern, "/**/*")) {
			positive = append(positive, positivePattern{
				basePath: strings.TrimSuffix(pattern, "/**/*"),
				isDeep:   true,
			})
		} else if strings.HasSuffix(pattern, "/**") && !containsGlobMeta(strings.TrimSuffix(pattern, "/**")) {
			positive = append(positive, positivePattern{
				basePath: strings.TrimSuffix(pattern, "/**"),
				isDeep:   true,
			})
		} else if strings.HasSuffix(pattern, "/*") && !containsGlobMeta(strings.TrimSuffix(pattern, "/*")) {
			positive = append(positive, positivePattern{
				basePath: strings.TrimSuffix(pattern, "/*"),
				isDir:    true,
			})
		} else if containsGlobMeta(pattern) {
			normalizedPattern := pathutil.NormalizeGlobPattern(pattern)
			compiledPattern, err := glob.Compile(normalizedPattern, '/')
			if err != nil {
				continue
			}
			positive = append(positive, positivePattern{
				isComplex:   true,
				complexRoot: getStaticPrefixBeforeGlob(normalizedPattern),
				complexGlob: compiledPattern,
			})
		} else {
			positive = append(positive, positivePattern{
				basePath: pattern,
			})
		}
	}

	candidateDirs := make(map[string]bool)

	for _, pos := range positive {
		fullBasePath := filepath.Join(pathutil.DenormalizePathForOS(ctx.WorkspaceRoot), pathutil.DenormalizePathForOS(pos.basePath))

		if pos.isComplex {
			complexRootPath := filepath.Join(pathutil.DenormalizePathForOS(ctx.WorkspaceRoot), pathutil.DenormalizePathForOS(pos.complexRoot))
			complexCandidates := make(map[string]bool)
			ctx.walkForPackagesWithWorkerPool(complexRootPath, allExcludePatterns, includeFilePatterns, includePrefixes, complexCandidates)
			for candidatePath := range complexCandidates {
				relToWorkspace, err := filepath.Rel(ctx.WorkspaceRoot, candidatePath)
				if err != nil {
					continue
				}
				relToWorkspace = pathutil.NormalizePathForInternal(relToWorkspace)
				if pos.complexGlob.Match(relToWorkspace) {
					candidateDirs[candidatePath] = true
				}
			}
		} else if pos.isDeep {
			// Recursive walk for ** patterns (can include nested workspace packages)
			ctx.walkForPackagesWithWorkerPool(fullBasePath, allExcludePatterns, includeFilePatterns, includePrefixes, candidateDirs)
		} else if pos.isDir {
			// One star: check immediate subdirectories
			entries, err := os.ReadDir(fullBasePath)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					dirPath := filepath.Join(fullBasePath, entry.Name())
					if globutil.IsExcludedByPatterns(dirPath, allExcludePatterns, includeFilePatterns) {
						continue
					}
					if _, err := os.Stat(filepath.Join(dirPath, "package.json")); err == nil {
						candidateDirs[pathutil.NormalizePathForInternal(dirPath)] = true
					}
				}
			}
		} else {
			// Direct path
			if globutil.IsExcludedByPatterns(fullBasePath, allExcludePatterns, includeFilePatterns) {
				continue
			}
			if _, err := os.Stat(filepath.Join(fullBasePath, "package.json")); err == nil {
				candidateDirs[pathutil.NormalizePathForInternal(fullBasePath)] = true
			}
		}
	}

	// Filter and process
	for dirPath := range candidateDirs {
		rel, err := filepath.Rel(ctx.WorkspaceRoot, dirPath)
		if err != nil {
			continue
		}
		rel = pathutil.NormalizePathForInternal(rel)
		if rel == "." || rel == "" {
			continue
		}

		isExcluded := false
		for _, m := range negative {
			if m.g.Match(rel) {
				isExcluded = true
				break
			}
		}

		if !isExcluded {
			ctx.processPossiblePackage(dirPath)
		}
	}
}

func (ctx *MonorepoContext) walkForPackagesWithWorkerPool(basePath string, excludeFilePatterns []globutil.GlobMatcher, includeFilePatterns []globutil.GlobMatcher, includePrefixes []string, candidateDirs map[string]bool) {
	type workspaceScanItem struct {
		dirPath      string
		excludeGlobs []globutil.GlobMatcher
	}

	workerCount := min(max(runtime.GOMAXPROCS(0), 2), 16)

	var candidateDirsMu sync.Mutex
	addCandidateDir := func(path string) {
		candidateDirsMu.Lock()
		candidateDirs[pathutil.NormalizePathForInternal(path)] = true
		candidateDirsMu.Unlock()
	}

	var queueMu sync.Mutex
	queueCond := sync.NewCond(&queueMu)
	queue := []workspaceScanItem{{
		dirPath:      basePath,
		excludeGlobs: excludeFilePatterns,
	}}
	pending := 1

	worker := func() {
		for {
			queueMu.Lock()
			for len(queue) == 0 && pending > 0 {
				queueCond.Wait()
			}
			if pending == 0 {
				queueMu.Unlock()
				return
			}
			currentItem := queue[0]
			queue = queue[1:]
			queueMu.Unlock()

			entries, err := os.ReadDir(currentItem.dirPath)
			subDirs := make([]workspaceScanItem, 0, len(entries))
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						if entry.Name() == ".git" {
							continue
						}
						dirPath := filepath.Join(currentItem.dirPath, entry.Name())
						if !globutil.ShouldTraverseDir(dirPath, currentItem.excludeGlobs, includeFilePatterns, includePrefixes) {
							continue
						}

						childExcludeGlobs := currentItem.excludeGlobs
						gitignoreFile, gitignoreError := os.ReadFile(filepath.Join(dirPath, ".gitignore"))
						if gitignoreError == nil {
							nestedGitignorePatterns := fs.ParseGitIgnore(string(gitignoreFile), dirPath)
							if len(nestedGitignorePatterns) > 0 {
								childExcludeGlobs = append(append([]globutil.GlobMatcher{}, currentItem.excludeGlobs...), nestedGitignorePatterns...)
							}
						}

						subDirs = append(subDirs, workspaceScanItem{
							dirPath:      dirPath,
							excludeGlobs: childExcludeGlobs,
						})
						continue
					}
					if entry.Name() == "package.json" {
						// For recursive workspace globs (/**), include this directory as a package
						// and continue traversing to discover nested workspace packages too.
						addCandidateDir(currentItem.dirPath)
					}
				}
			}

			queueMu.Lock()
			pending--
			if len(subDirs) > 0 {
				queue = append(queue, subDirs...)
				pending += len(subDirs)
			}
			queueCond.Broadcast()
			queueMu.Unlock()
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker()
		}()
	}
	wg.Wait()
}

func (ctx *MonorepoContext) processPossiblePackage(path string) {
	path = pathutil.NormalizePathForInternal(path)
	pkgJsonPath := filepath.Join(pathutil.DenormalizePathForOS(path), "package.json")
	content, err := os.ReadFile(pkgJsonPath)
	if err != nil {
		return
	}

	var config PackageJsonConfig
	if err := json.Unmarshal(jsonc.ToJSON(content), &config); err != nil {
		return
	}

	if config.Name == "" {
		return
	}

	ctx.PackageToPath[config.Name] = path
	ctx.PackageConfigCache[path] = &config
}

func (ctx *MonorepoContext) GetPackageConfig(packageRoot string) (*PackageJsonConfig, error) {
	packageRoot = pathutil.NormalizePathForInternal(packageRoot)
	if config, ok := ctx.PackageConfigCache[packageRoot]; ok {
		return config, nil
	}

	pkgJsonPath := filepath.Join(pathutil.DenormalizePathForOS(packageRoot), "package.json")
	content, err := os.ReadFile(pkgJsonPath)
	if err != nil {
		return nil, err
	}

	var config PackageJsonConfig
	if err := json.Unmarshal(jsonc.ToJSON(content), &config); err != nil {
		return nil, err
	}

	ctx.PackageConfigCache[packageRoot] = &config
	return &config, nil
}

// GetDevDependenciesFromConfig extracts dev dependencies from PackageJsonConfig
func GetDevDependenciesFromConfig(config *PackageJsonConfig) map[string]bool {
	if config == nil || config.DevDependencies == nil {
		return make(map[string]bool)
	}

	result := make(map[string]bool, len(config.DevDependencies))
	for dep := range config.DevDependencies {
		result[dep] = true
	}
	return result
}

// GetProductionDependenciesFromConfig extracts production dependencies from PackageJsonConfig
func GetProductionDependenciesFromConfig(config *PackageJsonConfig) map[string]bool {
	if config == nil || config.Dependencies == nil {
		return make(map[string]bool)
	}

	result := make(map[string]bool, len(config.Dependencies))
	for dep := range config.Dependencies {
		result[dep] = true
	}
	return result
}

func (ctx *MonorepoContext) GetPackageExports(packageRoot string, conditionNames []string) (*PackageJsonExports, error) {
	packageRoot = pathutil.NormalizePathForInternal(packageRoot)
	if exports, ok := ctx.PackageExportsCache[packageRoot]; ok {
		return exports, nil
	}

	config, err := ctx.GetPackageConfig(packageRoot)
	if err != nil {
		return nil, err
	}

	exports := &PackageJsonExports{
		Exports:          make(map[string]interface{}),
		WildcardPatterns: []WildcardPattern{},
		ParsedTargets:    make(map[string]*ImportTargetTreeNode),
		HasDotPrefix:     false,
	}

	if config.Exports != nil {
		if exportsString, ok := config.Exports.(string); ok {
			exports.Exports = map[string]interface{}{
				".": exportsString,
			}
			exports.HasDotPrefix = true
		} else if exportsMap, ok := config.Exports.(map[string]interface{}); ok {
			exports.Exports = exportsMap

			// Check if any key starts with "."
			for k := range exportsMap {
				if strings.HasPrefix(k, ".") {
					exports.HasDotPrefix = true
					break
				}
			}

			// Pre-process and cache wildcard patterns for keys
			for key, target := range exportsMap {
				if strings.Count(key, "*") > 1 {
					continue // Skip invalid keys with multiple wildcards
				}

				// Parse target into tree structure
				parsedTarget := ParseImportTarget(target, conditionNames)
				if parsedTarget != nil {
					exports.ParsedTargets[key] = parsedTarget
				}

				if strings.Contains(key, "*") {
					// Extract prefix and suffix for faster string matching
					wildcardIndex := strings.Index(key, "*")
					prefix := key[:wildcardIndex]
					suffix := key[wildcardIndex+1:]
					exports.WildcardPatterns = append(exports.WildcardPatterns, WildcardPattern{
						Key:    key,
						Prefix: prefix,
						Suffix: suffix,
					})
				}
			}

			// Sort wildcard patterns by key length descending for specificity
			slices.SortFunc(exports.WildcardPatterns, func(patternA, patternB WildcardPattern) int {
				return len(patternB.Key) - len(patternA.Key)
			})
		}
	}

	ctx.PackageExportsCache[packageRoot] = exports
	return exports, nil
}
