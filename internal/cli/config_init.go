package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

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
		result, err := initConfigFileCore(cwd)
		if err != nil {
			return err
		}
		printInitConfigResults(result)
		return nil
	},
}

// initConfigResult captures what initConfigFileCore produced so callers can report it.
type initConfigResult struct {
	configPath                   string
	rules                        []config.Rule
	isMonorepo                   bool
	monorepoPackageCount         int      // workspace-package rules created (excludes the root rule)
	standalonePackagePaths       []string // relative paths of standalone packages discovered in subdirectories
	createdForMonorepoSubPackage bool
	rootRuleCreated              bool // whether a "." root rule was created
	rootHasPackageJson           bool
}

// hasPackageJson reports whether dir contains a package.json file.
func hasPackageJson(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "package.json"))
	return err == nil
}

// makePackageRule builds the standard per-package rule (circular + unresolved imports
// enabled, everything else disabled, no module boundaries) used for monorepo workspace
// packages, standalone subdirectory packages, and monorepo sub-package configs.
func makePackageRule(path string) config.Rule {
	return config.Rule{
		Path: path,
		CircularImportsDetections: []*config.CircularImportsOptions{{
			Enabled:           true,
			IgnoreTypeImports: false,
		}},
		OrphanFilesDetections:        []*config.OrphanFilesOptions{{Enabled: false}},
		UnusedNodeModulesDetections:  []*config.UnusedNodeModulesOptions{{Enabled: false}},
		MissingNodeModulesDetections: []*config.MissingNodeModulesOptions{{Enabled: false}},
		UnusedExportsDetections:      []*config.UnusedExportsOptions{{Enabled: false}},
		UnresolvedImportsDetections:  []*config.UnresolvedImportsOptions{{Enabled: true}},
		DevDepsUsageOnProdDetections: []*config.RestrictedDevDependenciesUsageOptions{{Enabled: false}},
		RestrictedImportsDetections:  []*config.RestrictedImportsDetectionOptions{{Enabled: false}},
	}
}

// makeSrcRootRule builds the example root rule used for a plain (non-monorepo) single-package
// project: all per-package checks plus an exemplary src/**/* module boundary.
func makeSrcRootRule() config.Rule {
	rule := makePackageRule(".")
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

// initConfigFileCore creates the config file without printing results
func initConfigFileCore(cwd string) (*initConfigResult, error) {
	currentConfigVersion := "1.11"

	// Check if any config file already exists
	existingConfig, err := config.FindConfigFile(cwd)
	if err == nil && existingConfig != "" {
		return nil, fmt.Errorf("config file already exists at %s", existingConfig)
	}

	// Define the path for the new config file (always use the standard name)
	configPath := filepath.Join(cwd, ".rev-dep.config.jsonc")

	result := &initConfigResult{configPath: configPath}
	var rules []config.Rule

	monorepoCtx := monorepo.DetectMonorepo(cwd)

	// appendStandaloneRules discovers package.json directories in subdirectories that are not
	// part of any monorepo workspace (and not gitignored) and appends a rule for each. It
	// records the discovered relative paths on the result for reporting.
	appendStandaloneRules := func(ctx *monorepo.MonorepoContext) {
		excludePatterns := globutil.CreateGlobMatchers([]string{}, cwd)
		standaloneRelPaths := packageDirsToSortedRelPaths(cwd, ctx.DiscoverStandalonePackages(excludePatterns))
		for _, relPath := range standaloneRelPaths {
			rules = append(rules, makePackageRule(relPath))
		}
		result.standalonePackagePaths = standaloneRelPaths
	}

	if monorepoCtx != nil {
		result.isMonorepo = true
		// If invoked from inside a monorepo but not at the workspace root,
		// create a config only for the current sub-package (single rule with Path '.').
		// cwd is in OS form (backslash on Windows, with a trailing separator) while WorkspaceRoot is
		// in internal forward-slash form (no trailing slash). Normalize BOTH the separators
		// (NormalizePathForInternal) and the trailing slash (StandardiseDirPathInternal) before
		// comparing - a plain string compare silently mismatches on Windows and makes the workspace
		// root look like a sub-package (no per-package rules created).
		cwdInternal := pathutil.StandardiseDirPathInternal(pathutil.NormalizePathForInternal(cwd))
		rootInternal := pathutil.StandardiseDirPathInternal(pathutil.NormalizePathForInternal(monorepoCtx.WorkspaceRoot))
		if cwdInternal != rootInternal {
			rules = append(rules, makePackageRule("."))
			result.createdForMonorepoSubPackage = true
			result.rootRuleCreated = true
			result.rootHasPackageJson = true
		} else {
			// Monorepo root: root rule (module boundaries) + one rule per workspace package.
			rules = append(rules, makeMonorepoRootRule())
			result.rootRuleCreated = true
			result.rootHasPackageJson = true

			excludePatterns := globutil.CreateGlobMatchers([]string{}, cwd)
			monorepoCtx.FindWorkspacePackages(excludePatterns, nil)

			packagePaths := packageDirsToSortedRelPaths(cwd, mapValues(monorepoCtx.PackageToPath))
			for _, relPath := range packagePaths {
				rules = append(rules, makePackageRule(relPath))
			}
			result.monorepoPackageCount = len(packagePaths)

			// Also surface standalone packages that live outside the declared workspaces.
			appendStandaloneRules(monorepoCtx)
		}
	} else {
		// No monorepo. Decide whether a root rule makes sense based on whether the root itself
		// is a package, then discover standalone packages in subdirectories.
		rootCtx := monorepo.NewMonorepoContext(pathutil.NormalizePathForInternal(filepath.Clean(cwd)))
		result.rootHasPackageJson = hasPackageJson(cwd)

		if result.rootHasPackageJson {
			// Root is a single (non-workspace) package: keep the root rule, then add any
			// standalone subdirectory packages.
			rules = append(rules, makeSrcRootRule())
			result.rootRuleCreated = true
			appendStandaloneRules(rootCtx)
		} else {
			// No root package.json: create rules only for standalone subdirectory packages.
			appendStandaloneRules(rootCtx)
			// Fallback: nothing discovered at all -> a single root rule so the config isn't empty.
			if len(result.standalonePackagePaths) == 0 {
				rules = append(rules, makeSrcRootRule())
				result.rootRuleCreated = true
			}
		}
	}

	result.rules = rules

	// Create config structure
	cfg := config.RevDepConfig{
		ConfigVersion: currentConfigVersion,
		Rules:         rules,
		Schema:        "https://github.com/jayu/rev-dep/blob/master/config-schema/" + currentConfigVersion + ".schema.json?raw=true",
		NodeModulesResolution: &config.NodeModulesResolutionConfig{
			ResolutionType:         config.NodeModulesResolutionEntryPackage,
			IncludeDevDepsFromRoot: false,
		},
	}

	// Marshal config to JSON with proper formatting
	configJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %v", err)
	}

	// Write config file
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config file: %v", err)
	}

	return result, nil
}

// printInitConfigResults prints the results of config initialization
func printInitConfigResults(result *initConfigResult) {
	fmt.Printf("✅ Created .rev-dep.config.jsonc at %s\n", result.configPath)

	switch {
	case result.createdForMonorepoSubPackage:
		fmt.Printf("⚠️  Created config for monorepo sub-package. This file targets the current package only.\n")
	case result.isMonorepo:
		if result.monorepoPackageCount > 0 {
			fmt.Printf("📦 Monorepo detected: discovered %d workspace package(s) and created a rule for each.\n", result.monorepoPackageCount)
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
		fmt.Printf("🧩 Discovered %d standalone package(s) in subdirectories (not part of a monorepo) and created a rule for each:\n", len(result.standalonePackagePaths))
		for _, relPath := range result.standalonePackagePaths {
			fmt.Printf("   - %s\n", relPath)
		}
	}

	fmt.Println("Adjust rules to make them relevant to your project setup.\nGenerated module boundaries config is exemplary and does not make much sense.")
	fmt.Println("Hint: feed LLM with config file JSON schema to get started.")
}
