package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// ---------------- module-boundaries ----------------
var (
	moduleBoundariesCwd        string
	moduleBoundariesConfigPath string
)

type ValidationResult struct {
	FilePath   string
	ResultType string // "allowed" | "denied" | "not_allowed"
	Target     string
	RuleName   string
}

// moduleBoundariesCmdFn checks for module boundary violations.
func moduleBoundariesCmdFn(cwd string, configPath string, packageJsonPath, tsconfigJsonPath string, conditionNames []string, followMonorepoPackages bool) (string, bool, error) {
	// 1. Load Config
	// If configPath is explicitly provided, use it. Otherwise try to find in cwd.
	pathToLoad := configPath
	if pathToLoad == "" {
		pathToLoad = cwd
	}

	configs, err := LoadConfig(pathToLoad)
	if err != nil {
		return "", false, fmt.Errorf("Could not load configuration from %s:\n%v", filepath.Join(pathToLoad, configFileName), err)
	}

	// Determine base directory for resolving relative paths in config
	configBaseDir := cwd
	fileInfo, err := os.Stat(pathToLoad)
	if err == nil {
		if !fileInfo.IsDir() {
			configBaseDir = filepath.Dir(pathToLoad)
		} else {
			configBaseDir = pathToLoad
		}
	}

	var reportBuilder strings.Builder
	hasViolations := false
	totalViolationsCount := 0

	for _, config := range configs {
		// Process each rule in the config
		for _, rule := range config.Rules {
			// Determine effective CWD for this specific rule
			// If rule.Path is set, it's relative to the config file location
			targetCwd := configBaseDir
			if rule.Path != "" {
				targetCwd = filepath.Join(configBaseDir, rule.Path)
			}

			targetCwd, _ = filepath.Abs(targetCwd)

			// 2. Get Dependency Tree for this scope
			excludeFiles := []string{}
			// Note: passing empty exclude patterns for now, could be enhanced to support per-config excludes if added to struct
			minimalTree, files, _ := GetMinimalDepsTreeForCwd(targetCwd, false, excludeFiles, []string{}, packageJsonPath, tsconfigJsonPath, conditionNames, followMonorepoPackages)

			// Compile matchers for all boundaries in this rule
			type CompiledBoundary struct {
				Rule            BoundaryRule
				PatternMatchers []GlobMatcher
				AllowMatchers   []GlobMatcher
				DenyMatchers    []GlobMatcher
			}

			compiledBoundaries := make([]CompiledBoundary, 0, len(rule.ModuleBoundaries))
			for _, boundary := range rule.ModuleBoundaries {
				cb := CompiledBoundary{
					Rule:            boundary,
					PatternMatchers: CreateGlobMatchers([]string{boundary.Pattern}, targetCwd),
					AllowMatchers:   CreateGlobMatchers(boundary.Allow, targetCwd),
					DenyMatchers:    CreateGlobMatchers(boundary.Deny, targetCwd),
				}
				compiledBoundaries = append(compiledBoundaries, cb)
			}

			// 3. Check Violations
			for _, filePath := range files {
				// Find which boundaries apply to this file
				for _, boundary := range compiledBoundaries {
					if MatchesAnyGlobMatcher(filePath, boundary.PatternMatchers, false) {
						// Check dependencies
						fileDeps, ok := minimalTree[filePath]
						if !ok {
							continue
						}

						for _, dep := range fileDeps {
							if dep.ID != nil && (dep.ResolvedType == UserModule || dep.ResolvedType == MonorepoModule) {
								resolvedPath := *dep.ID

								// Check if denied
								if len(boundary.DenyMatchers) > 0 && MatchesAnyGlobMatcher(resolvedPath, boundary.DenyMatchers, false) {
									reportBuilder.WriteString(fmt.Sprintf("Violation [%s]: %s -> %s (Matched Deny Pattern)\n", boundary.Rule.Name, filePath, resolvedPath))
									totalViolationsCount++
									hasViolations = true
									continue
								}

								// Check if allowed
								if len(boundary.AllowMatchers) > 0 {
									if !MatchesAnyGlobMatcher(resolvedPath, boundary.AllowMatchers, false) {
										reportBuilder.WriteString(fmt.Sprintf("Violation [%s]: %s -> %s (Not in Allow List)\n", boundary.Rule.Name, filePath, resolvedPath))
										totalViolationsCount++
										hasViolations = true
									}
								}
							}
						}
					}
				}
			}
		} // Close the rule loop
	} // Close the config loop

	if hasViolations {
		return reportBuilder.String(), true, nil
	}

	return "", false, nil
}

var moduleBoundariesCmd = &cobra.Command{
	Use:   "module-boundaries",
	Short: "Enforce module boundaries and import rules",
	Long:  `Check for import violations based on defined module boundaries in rev-dep.config.json.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		report, hasViolations, err := moduleBoundariesCmdFn(
			ResolveAbsoluteCwd(moduleBoundariesCwd),
			moduleBoundariesConfigPath,
			packageJsonPath,
			tsconfigJsonPath,
			conditionNames,
			followMonorepoPackages,
		)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if hasViolations {
			fmt.Print(report) // Print the report to stdout
			os.Exit(1)        // Exit with 1
		}

		return nil
	},
}

func init() {
	addSharedFlags(moduleBoundariesCmd)
	moduleBoundariesCmd.Flags().StringVarP(&moduleBoundariesCwd, "cwd", "c", currentDir, "Working directory")
	moduleBoundariesCmd.Flags().StringVar(&moduleBoundariesConfigPath, "config", "", "Path to rev-dep.config.json or directory containing it")
}
