package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	WorkspaceRoot      string
	PackageToPath      map[string]string
	PackageConfigCache map[string]*PackageJsonConfig
}

func NewMonorepoContext(root string) *MonorepoContext {
	return &MonorepoContext{
		WorkspaceRoot:      root,
		PackageToPath:      make(map[string]string),
		PackageConfigCache: make(map[string]*PackageJsonConfig),
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
					if _, hasWorkspaces := pkgJson["workspaces"]; hasWorkspaces {
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

func (ctx *MonorepoContext) FindWorkspacePackages(cwd string) {
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

	/**
	Sub packages discovery logic could be simplified if we would ignore the pnpm ability to to have exclude patterns
	Then we could just build the allowlist of dirs to scan, rather than walking the whole workspace and matching against patterns and excluding negations
	*/

	type matcher struct {
		g          glob.Glob
		isNegative bool
	}

	var matchers []matcher

	for _, p := range patterns {
		isNegative := strings.HasPrefix(p, "!")
		cleanP := p
		if isNegative {
			cleanP = strings.TrimPrefix(p, "!")
		}
		// Ensure pattern uses forward slashes
		cleanP = NormalizeGlobPattern(cleanP)

		g, err := glob.Compile(cleanP, '/')
		if err == nil {
			matchers = append(matchers, matcher{g: g, isNegative: isNegative})
		}
	}

	filepath.WalkDir(ctx.WorkspaceRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" || d.Name() == ".idea" || d.Name() == ".vscode" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "package.json" {
			return nil
		}

		dir := filepath.Dir(path)

		rel, err := filepath.Rel(ctx.WorkspaceRoot, dir)
		if err != nil {
			return nil
		}

		rel = NormalizePathForInternal(rel)

		if rel == "." || rel == "" {
			return nil
		}

		included := false
		for _, m := range matchers {
			if m.g.Match(rel) {
				if m.isNegative {
					included = false
					break
				} else {
					included = true
				}
			}
		}

		if included {
			ctx.processPossiblePackage(dir)
		}
		return nil
	})
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
