package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// ---------------- config ----------------
var (
	configCwd string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Create and execute rev-dep configuration files",
	Long:  `Commands for creating and executing rev-dep configuration files.`,
}

// ---------------- config run ----------------
var (
	runConfigCwd string
)

var configRunCmd = &cobra.Command{
	Use:   "run",
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

// ---------------- config init ----------------
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new rev-dep.config.json file",
	Long:  `Create a new rev-dep.config.json configuration file in the current directory with default settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := ResolveAbsoluteCwd(configCwd)
		configPath, rules, createdForMonorepoSubPackage, err := initConfigFileCore(cwd)
		if err != nil {
			return err
		}
		printInitConfigResults(configPath, rules, createdForMonorepoSubPackage)
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
	// config command
	configCmd.Flags().StringVarP(&configCwd, "cwd", "c", currentDir, "Working directory")

	// config run command
	addSharedFlags(configRunCmd)
	configRunCmd.Flags().StringVarP(&runConfigCwd, "cwd", "c", currentDir, "Working directory")

	// config init command
	configInitCmd.Flags().StringVarP(&configCwd, "cwd", "c", currentDir, "Working directory")

	// Add subcommands to config
	configCmd.AddCommand(configRunCmd, configInitCmd)
}

// initConfigFileCore creates the config file without printing results
func initConfigFileCore(cwd string) (string, []Rule, bool, error) {
	currentConfigVersion := "1.0"

	// Check if any config file already exists
	existingConfig, err := findConfigFile(cwd)
	if err == nil && existingConfig != "" {
		return "", nil, false, fmt.Errorf("config file already exists at %s", existingConfig)
	}

	// Define the path for the new config file (always use the standard name)
	configPath := filepath.Join(cwd, ".rev-dep.config.jsonc")

	// Discover monorepo packages
	var rules []Rule
	monorepoCtx := DetectMonorepo(cwd)
	createdForMonorepoSubPackage := false

	if monorepoCtx != nil {
		// If invoked from inside a monorepo but not at the workspace root,
		// create a config only for the current sub-package (single rule with Path '.')
		if NormalizePathForInternal(cwd) != monorepoCtx.WorkspaceRoot {
			packageRule := Rule{
				Path: ".",
				CircularImportsDetection: &CircularImportsOptions{
					Enabled:           true,
					IgnoreTypeImports: false,
				},
				OrphanFilesDetection: &OrphanFilesOptions{
					Enabled: false,
				},
				UnusedNodeModulesDetection: &UnusedNodeModulesOptions{
					Enabled: false,
				},
				MissingNodeModulesDetection: &MissingNodeModulesOptions{
					Enabled: false,
				},
			}
			rules = append(rules, packageRule)
			createdForMonorepoSubPackage = true
		} else {
			// Monorepo root: Root rule only has module boundaries
			rootRule := Rule{
				Path: ".",
				ModuleBoundaries: []BoundaryRule{
					{
						Name:    "packages",
						Pattern: "packages/**/*",
						Allow:   []string{"packages/**/*"},
					},
				},
			}
			rules = append(rules, rootRule)

			// Find workspace packages
			excludePatterns := CreateGlobMatchers([]string{}, cwd)
			monorepoCtx.FindWorkspacePackages(cwd, excludePatterns)

			// Collect and sort package paths
			var packagePaths []string
			for _, packagePath := range monorepoCtx.PackageToPath {
				// Convert absolute path to relative path from cwd
				relPath, err := filepath.Rel(cwd, packagePath)
				if err != nil {
					continue // Skip if we can't get relative path
				}
				relPath = filepath.ToSlash(relPath)

				// Skip root package (already covered)
				if relPath == "." || relPath == "" {
					continue
				}

				packagePaths = append(packagePaths, relPath)
			}

			// Sort package paths alphabetically
			slices.Sort(packagePaths)

			// Create a rule for each discovered package in sorted order
			for _, relPath := range packagePaths {
				packageRule := Rule{
					Path: relPath,
					CircularImportsDetection: &CircularImportsOptions{
						Enabled:           true,
						IgnoreTypeImports: false,
					},
					OrphanFilesDetection: &OrphanFilesOptions{
						Enabled: false,
					},
					UnusedNodeModulesDetection: &UnusedNodeModulesOptions{
						Enabled: false,
					},
					MissingNodeModulesDetection: &MissingNodeModulesOptions{
						Enabled: false,
					},
				}
				rules = append(rules, packageRule)
			}
		}
	} else {
		// Non-monorepo: Single rule with all checks including module boundaries
		rootRule := Rule{
			Path: ".",
			ModuleBoundaries: []BoundaryRule{
				{
					Name:    "src",
					Pattern: "src/**/*",
					Allow:   []string{"src/**/*"},
				},
			},
			CircularImportsDetection: &CircularImportsOptions{
				Enabled:           true,
				IgnoreTypeImports: false,
			},
			OrphanFilesDetection: &OrphanFilesOptions{
				Enabled: false,
			},
			UnusedNodeModulesDetection: &UnusedNodeModulesOptions{
				Enabled: false,
			},
			MissingNodeModulesDetection: &MissingNodeModulesOptions{
				Enabled: false,
			},
		}
		rules = append(rules, rootRule)
	}

	// Create config structure
	config := RevDepConfig{
		ConfigVersion: currentConfigVersion,
		Rules:         rules,
		Schema:        "https://github.com/jayu/rev-dep/blob/module-boundaries/config-schema/" + currentConfigVersion + ".schema.json?raw=true",
	}

	// Add schema reference if schema file exists
	schemaPath := filepath.Join(cwd, "config-schema", "1.0.schema.json")
	if _, err := os.Stat(schemaPath); err == nil {
		// We'll add the schema field during JSON marshaling
	}

	// Marshal config to JSON with proper formatting
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", nil, false, fmt.Errorf("failed to marshal config: %v", err)
	}

	// Write config file
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		return "", nil, false, fmt.Errorf("failed to write config file: %v", err)
	}

	return configPath, rules, createdForMonorepoSubPackage, nil
}

// printInitConfigResults prints the results of config initialization
func printInitConfigResults(configPath string, rules []Rule, createdForMonorepoSubPackage bool) {
	fmt.Printf("✅ Created .rev-dep.config.jsonc at %s\n", configPath)
	if createdForMonorepoSubPackage {
		fmt.Printf("⚠️  Created config for monorepo sub-package. This file targets the current package only.\n")
	}
	if len(rules) > 1 {
		fmt.Printf("📦 Discovered %d monorepo packages and created rules for each\n", len(rules)-1)
	} else {
		fmt.Printf("📁 Created single rule for root directory\n")
	}

	fmt.Println("Adjust rules to make them relevant to your project setup.\nGenerated module boundaries config is exemplary and does not make much sense.")
	fmt.Println("Hint: feed LLM with config file JSON schema to get started.")
}

// initConfigFile initializes a new rev-dep.config.json file with minimal structure
func initConfigFile(cwd string) error {
	configPath, rules, createdForMonorepoSubPackage, err := initConfigFileCore(cwd)
	if err != nil {
		return err
	}
	printInitConfigResults(configPath, rules, createdForMonorepoSubPackage)
	return nil
}
