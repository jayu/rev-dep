package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gobwas/glob"
	"github.com/tidwall/jsonc"
	"gopkg.in/yaml.v3"
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

func NewMonorepoContext(root string) *MonorepoContext {
	return &MonorepoContext{
		WorkspaceRoot:       root,
		PackageToPath:       make(map[string]string),
		PackageConfigCache:  make(map[string]*PackageJsonConfig),
		PackageExportsCache: make(map[string]*PackageJsonExports),
	}
}

func DetectMonorepo(cwd string) *MonorepoContext {
	currentDir := NormalizePathForInternal(filepath.Clean(cwd))
	for {
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

		pnpmWorkspacePath := filepath.Join(currentDir, "pnpm-workspace.yaml")
		if _, err := os.Stat(pnpmWorkspacePath); err == nil {
			return NewMonorepoContext(currentDir)
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

func (ctx *MonorepoContext) FindWorkspacePackages(root string, excludeFilePatterns []GlobMatcher) {
	pkgJsonPath := filepath.Join(DenormalizePathForOS(ctx.WorkspaceRoot), "package.json")
	content, err := os.ReadFile(pkgJsonPath)
	if err != nil {
		return
	}

	var pkgJson struct {
		Workspaces interface{} `json:"workspaces"` // can be []string or { packages: []string }
	}

	if err := json.Unmarshal(jsonc.ToJSON(content), &pkgJson); err != nil {
		return
	}

	var patterns []string
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

	if len(patterns) == 0 {
		pnpmWorkspacePath := filepath.Join(DenormalizePathForOS(ctx.WorkspaceRoot), "pnpm-workspace.yaml")
		if pnpmContent, err := os.ReadFile(pnpmWorkspacePath); err == nil {
			var pnpmWorkspace struct {
				Packages []string `yaml:"packages"`
			}
			if err := yaml.Unmarshal(pnpmContent, &pnpmWorkspace); err == nil {
				patterns = append(patterns, pnpmWorkspace.Packages...)
			}
		}
	}

	type positivePattern struct {
		basePath string
		isDeep   bool
		isDir    bool
	}

	var positive []positivePattern
	type negativeMatcher struct {
		g glob.Glob
	}
	var negative []negativeMatcher

	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "!") {
			cleanP := strings.TrimPrefix(pattern, "!")
			cleanP = NormalizeGlobPattern(cleanP)
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

		if strings.HasSuffix(pattern, "/**") {
			positive = append(positive, positivePattern{
				basePath: strings.TrimSuffix(pattern, "/**"),
				isDeep:   true,
			})
		} else if strings.HasSuffix(pattern, "/*") {
			positive = append(positive, positivePattern{
				basePath: strings.TrimSuffix(pattern, "/*"),
				isDir:    true,
			})
		} else {
			positive = append(positive, positivePattern{
				basePath: pattern,
			})
		}
	}

	candidateDirs := make(map[string]bool)

	for _, pos := range positive {
		fullBasePath := filepath.Join(DenormalizePathForOS(ctx.WorkspaceRoot), DenormalizePathForOS(pos.basePath))

		if pos.isDeep {
			// Recursive walk, stop at package.json
			ctx.walkForPackages(fullBasePath, excludeFilePatterns, candidateDirs)
		} else if pos.isDir {
			// One star: check immediate subdirectories
			entries, err := os.ReadDir(fullBasePath)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					dirPath := filepath.Join(fullBasePath, entry.Name())
					if _, err := os.Stat(filepath.Join(dirPath, "package.json")); err == nil {
						candidateDirs[NormalizePathForInternal(dirPath)] = true
					}
				}
			}
		} else {
			// Direct path
			if _, err := os.Stat(filepath.Join(fullBasePath, "package.json")); err == nil {
				candidateDirs[NormalizePathForInternal(fullBasePath)] = true
			}
		}
	}

	// Filter and process
	for dirPath := range candidateDirs {
		rel, err := filepath.Rel(ctx.WorkspaceRoot, dirPath)
		if err != nil {
			continue
		}
		rel = NormalizePathForInternal(rel)
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

func (ctx *MonorepoContext) walkForPackages(basePath string, excludeFilePatterns []GlobMatcher, candidateDirs map[string]bool) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return
	}

	hasPkgJson := false
	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() == "package.json" {
			hasPkgJson = true
			break
		}
	}

	if hasPkgJson {
		// Stop recursing in this branch
		candidateDirs[NormalizePathForInternal(basePath)] = true
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == ".idea" || name == ".vscode" || name == "node_modules" {
				continue
			}

			dirPath := filepath.Join(basePath, name)
			if MatchesAnyGlobMatcher(dirPath, excludeFilePatterns, false) {
				continue
			}

			ctx.walkForPackages(dirPath, excludeFilePatterns, candidateDirs)
		}
	}
}

func (ctx *MonorepoContext) processPossiblePackage(path string) {
	path = NormalizePathForInternal(path)
	pkgJsonPath := filepath.Join(DenormalizePathForOS(path), "package.json")
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
	packageRoot = NormalizePathForInternal(packageRoot)
	if config, ok := ctx.PackageConfigCache[packageRoot]; ok {
		return config, nil
	}

	pkgJsonPath := filepath.Join(DenormalizePathForOS(packageRoot), "package.json")
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

func (ctx *MonorepoContext) GetPackageExports(packageRoot string, conditionNames []string) (*PackageJsonExports, error) {
	packageRoot = NormalizePathForInternal(packageRoot)
	if exports, ok := ctx.PackageExportsCache[packageRoot]; ok {
		return exports, nil
	}

	config, err := ctx.GetPackageConfig(packageRoot)
	if err != nil {
		return nil, err
	}

	exports := &PackageJsonExports{
		exports:          make(map[string]interface{}),
		wildcardPatterns: []WildcardPattern{},
		parsedTargets:    make(map[string]*ImportTargetTreeNode),
		hasDotPrefix:     false,
	}

	if config.Exports != nil {
		if exportsString, ok := config.Exports.(string); ok {
			exports.exports = map[string]interface{}{
				".": exportsString,
			}
			exports.hasDotPrefix = true
		} else if exportsMap, ok := config.Exports.(map[string]interface{}); ok {
			exports.exports = exportsMap

			// Check if any key starts with "."
			for k := range exportsMap {
				if strings.HasPrefix(k, ".") {
					exports.hasDotPrefix = true
					break
				}
			}

			// Pre-process and cache wildcard patterns for keys
			for key, target := range exportsMap {
				if strings.Count(key, "*") > 1 {
					continue // Skip invalid keys with multiple wildcards
				}

				// Parse target into tree structure
				parsedTarget := parseImportTarget(target, conditionNames)
				if parsedTarget != nil {
					exports.parsedTargets[key] = parsedTarget
				}

				if strings.Contains(key, "*") {
					// Extract prefix and suffix for faster string matching
					wildcardIndex := strings.Index(key, "*")
					prefix := key[:wildcardIndex]
					suffix := key[wildcardIndex+1:]
					exports.wildcardPatterns = append(exports.wildcardPatterns, WildcardPattern{
						key:    key,
						prefix: prefix,
						suffix: suffix,
					})
				}
			}

			// Sort wildcard patterns by key length descending for specificity
			slices.SortFunc(exports.wildcardPatterns, func(patternA, patternB WildcardPattern) int {
				return len(patternB.key) - len(patternA.key)
			})
		}
	}

	ctx.PackageExportsCache[packageRoot] = exports
	return exports, nil
}
