package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// ---------------- run-config ----------------
var (
	runConfigCwd string
)

var runConfigCmd = &cobra.Command{
	Use:   "run-config",
	Short: "Execute all checks defined in (.)rev-dep.config.json(c)",
	Long:  `Process (.)rev-dep.config.json(c) and execute all enabled checks (circular imports, orphan files, module boundaries, node modules) per rule.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := ResolveAbsoluteCwd(runConfigCwd)

		// Auto-discover config in current working directory
		configs, err := LoadConfig(cwd)
		if err != nil {
			return fmt.Errorf("Could not load configuration from %s:\n%v", filepath.Join(cwd, configFileName), err)
		}

		// Process each config
		for i, config := range configs {
			if len(configs) > 1 {
				fmt.Printf("=== Processing config %d ===\n", i+1)
			}

			// Process the config
			result, err := ProcessConfig(&config, cwd, packageJsonPath, tsconfigJsonPath)
			if err != nil {
				return fmt.Errorf("Error processing config: %v", err)
			}

			// Format and print results
			formatAndPrintConfigResults(result, cwd)

			if result.HasFailures {
				os.Exit(1)
			}

			if len(configs) > 1 {
				fmt.Printf("=== End config %d ===\n\n", i+1)
			}
		}

		return nil
	},
}

// formatAndPrintConfigResults formats and prints the config processing results
func formatAndPrintConfigResults(result *ConfigProcessingResult, cwd string) {
	// Helper function to convert absolute paths to relative paths
	getRelativePath := func(absolutePath string) string {
		if absolutePath == "" {
			return absolutePath
		}
		relPath, err := filepath.Rel(cwd, absolutePath)
		if err != nil {
			// Fallback to absolute path if relative conversion fails
			return absolutePath
		}
		// Convert forward slashes for consistency
		return filepath.ToSlash(relPath)
	}

	for _, ruleResult := range result.RuleResults {
		if ruleResult.RulePath != "" {
			fmt.Printf("\n📁 Rule: %s (%d files)\n", ruleResult.RulePath, ruleResult.FileCount)
		}

		// Show enabled checks and their status with indentation
		for _, check := range ruleResult.EnabledChecks {
			switch check {
			case "circular-imports":
				if len(ruleResult.CircularDependencies) > 0 {
					fmt.Printf("  ❌ Circular Dependencies (%d):\n\n", len(ruleResult.CircularDependencies))

					formattedOutput := FormatCircularDependenciesWithoutHeader(ruleResult.CircularDependencies, cwd, ruleResult.DependencyTree, 2)
					fmt.Printf("%s", formattedOutput)
				} else {
					fmt.Printf("  ✅ Circular Dependencies\n")
				}
			case "orphan-files":
				if len(ruleResult.OrphanFiles) > 0 {
					fmt.Printf("  ❌  Orphan Files (%d):\n", len(ruleResult.OrphanFiles))
					for _, file := range ruleResult.OrphanFiles {
						fmt.Printf("    - %s\n", getRelativePath(file))
					}
				} else {
					fmt.Printf("  ✅ Orphan Files\n")
				}
			case "module-boundaries":
				if len(ruleResult.ModuleBoundaryViolations) > 0 {
					fmt.Printf("  ❌ Module Boundary Violations (%d):\n", len(ruleResult.ModuleBoundaryViolations))
					for _, violation := range ruleResult.ModuleBoundaryViolations {
						violationType := "NOT ALLOWED"
						if violation.ViolationType == "denied" {
							violationType = "DENIED"
						}
						fmt.Printf("    - [%s] %s -> %s (%s)\n",
							violation.RuleName,
							getRelativePath(violation.FilePath),
							getRelativePath(violation.ImportPath),
							violationType)
					}
				} else {
					fmt.Printf("  ✅ Module Boundaries\n")
				}
			case "unused-node-modules":
				if len(ruleResult.UnusedNodeModules) > 0 {
					fmt.Printf("  ❌ Unused Node Modules (%d):\n", len(ruleResult.UnusedNodeModules))
					for _, module := range ruleResult.UnusedNodeModules {
						fmt.Printf("    - %s\n", module)
					}
				} else {
					fmt.Printf("  ✅ Unused Node Modules\n")
				}
			case "missing-node-modules":
				if len(ruleResult.MissingNodeModules) > 0 {
					fmt.Printf("  ❌ Missing Node Modules (%d):\n", len(ruleResult.MissingNodeModules))
					for _, missing := range ruleResult.MissingNodeModules {
						// Convert imported from paths to relative paths
						relativeImportedFrom := make([]string, len(missing.ImportedFrom))
						for j, path := range missing.ImportedFrom {
							relativeImportedFrom[j] = getRelativePath(path)
						}
						fmt.Printf("    - %s (imported from: %s)\n", missing.ModuleName, strings.Join(relativeImportedFrom, ", "))
					}
				} else {
					fmt.Printf("  ✅ Missing Node Modules\n")
				}
			}
		}

		// Show warning if no files found for this rule
		if ruleResult.FileCount == 0 {
			fmt.Printf("  ⚠️  No files found for this rule - check if the path is correct\n")
		}

		// Show warning if package.json is missing in the rule path directory
		if ruleResult.MissingPackageJson {
			packageJsonPath := filepath.Join(cwd, ruleResult.RulePath, "package.json")
			fmt.Printf("  ⚠️  Warning: Rule path missing package.json - some features may not work (missing: %s)\n", packageJsonPath)
		}
	}

	// Print final verdict
	if !result.HasFailures {
		fmt.Printf("\n✅ All checks passed!\n")
	} else {
		fmt.Printf("\n❌ Checks failed! See details above.\n")
	}
}

func init() {
	addSharedFlags(runConfigCmd)
	runConfigCmd.Flags().StringVarP(&runConfigCwd, "cwd", "c", currentDir, "Working directory")
}
