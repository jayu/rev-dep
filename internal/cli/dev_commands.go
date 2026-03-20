//go:build dev
// +build dev

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"rev-dep-go/internal/model"
	"rev-dep-go/internal/pathutil"
	"rev-dep-go/internal/resolve"
)

// ---------------- browser ----------------
var (
	browserCwd        string
	browserEntryPoint string
	browserIgnoreType bool
)

var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Launch interactive dependency visualization in browser",
	Long: `Starts a local web server with an interactive visualization
of your project's dependency graph.`,
	Example: "rev-dep browser --entry-point src/index.ts",
	RunE: func(cmd *cobra.Command, args []string) error {
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		cwd := pathutil.ResolveAbsoluteCwd(browserCwd)
		absolutePathToEntryPoint := pathutil.JoinWithCwd(cwd, browserEntryPoint)
		excludeFiles := []string{}

		minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, browserIgnoreType, excludeFiles, []string{absolutePathToEntryPoint}, packageJsonPath, tsconfigJsonPath, conditionNames, followValue, nil)

		StartServer(minimalTree, absolutePathToEntryPoint, cwd)
		return nil
	},
}

type MinimalDependencyWithLabels struct {
	ID                string                   `json:"id"`
	Request           string                   `json:"request"`
	ResolvedType      model.ResolvedImportType `json:"resolvedType"`
	ResolvedTypeLabel string                   `json:"resolvedTypeLabel"`
	ImportKind        model.ImportKind         `json:"importKind"`
	ImportKindLabel   string                   `json:"importKindLabel"`
}

// ---------------- debug-parse-file ----------------
var (
	debugFile    string
	debugFileCwd string
)

var debugParseFileCmd = &cobra.Command{
	Use:   "debug-parse-file",
	Short: "Debug: Show parsed imports for a single file",
	Long:  `Development tool to inspect how the parser processes a specific file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		cwd := pathutil.ResolveAbsoluteCwd(debugFileCwd)
		path := pathutil.JoinWithCwd(cwd, debugFile)
		excludeFiles := []string{}

		minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, debugTreeIgnoreType, excludeFiles, []string{path}, packageJsonPath, tsconfigJsonPath, conditionNames, followValue, nil)

		fmt.Println(path)

		minimalDep := minimalTree[path]
		for _, dep := range minimalDep {
			depWithLabels := MinimalDependencyWithLabels{
				ID:                dep.ID,
				Request:           dep.Request,
				ResolvedType:      dep.ResolvedType,
				ResolvedTypeLabel: model.ResolvedImportTypeToString(dep.ResolvedType),
				ImportKind:        dep.ImportKind,
				ImportKindLabel:   model.ImportKindToString(dep.ImportKind),
			}
			jsonDep, err := json.MarshalIndent(depWithLabels, "  ", "  ")
			if err == nil {
				fmt.Println(string(jsonDep))
			} else {
				fmt.Println("Error marshaling dependency:", err)
			}
		}

		return nil
	},
}

// ---------------- debug-get-tree-for-cwd ----------------
var (
	debugTreeCwd        string
	debugTreeIgnoreType bool
)

var debugGetTreeCmd = &cobra.Command{
	Use:   "debug-get-tree-for-cwd",
	Short: "Debug: Show complete dependency tree for analysis",
	Long:  `Development tool to inspect the complete dependency tree.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		cwd := pathutil.ResolveAbsoluteCwd(debugTreeCwd)
		excludeFiles := []string{}

		minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, debugTreeIgnoreType, excludeFiles, []string{}, packageJsonPath, tsconfigJsonPath, conditionNames, followValue, nil)

		treeWithLabels := make(map[string][]MinimalDependencyWithLabels)
		for key, deps := range minimalTree {
			var depsWithLabels []MinimalDependencyWithLabels
			for _, dep := range deps {
				dwl := MinimalDependencyWithLabels{
					ID:                dep.ID,
					Request:           dep.Request,
					ResolvedType:      dep.ResolvedType,
					ResolvedTypeLabel: model.ResolvedImportTypeToString(dep.ResolvedType),
					ImportKind:        dep.ImportKind,
					ImportKindLabel:   model.ImportKindToString(dep.ImportKind),
				}
				depsWithLabels = append(depsWithLabels, dwl)
			}
			treeWithLabels[key] = depsWithLabels
		}

		jsonTree, err := json.MarshalIndent(treeWithLabels, "", " ")
		if err == nil {
			fmt.Println(string(jsonTree))
		}
		return err
	},
}

// ---------------- debug-tsconfig ----------------
var (
	debugTsconfigPath string
)

var debugTsconfigCmd = &cobra.Command{
	Use:   "debug-parse-tsconfig",
	Short: "Debug: Show parsed TypeScript configuration aliases",
	Long:  `Development tool to inspect how TypeScript configuration is parsed and what aliases are extracted.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve the tsconfig file using the existing tsconfig.go functionality
		tsconfigContent, err := resolve.ParseTsConfig(debugTsconfigPath)
		if err != nil {
			return fmt.Errorf("failed to parse tsconfig: %w", err)
		}

		// Use the extracted ParseTsConfigContent function to get aliases
		tsConfigParsed := resolve.ParseTsConfigContent(tsconfigContent)

		// Create a human-readable output
		output := map[string]interface{}{
			"aliases":          tsConfigParsed.Aliases,
			"wildcardPatterns": []map[string]interface{}{},
		}

		// Convert wildcard patterns to a more readable format
		for _, pattern := range tsConfigParsed.WildcardPatterns {
			output["wildcardPatterns"] = append(output["wildcardPatterns"].([]map[string]interface{}), map[string]interface{}{
				"key":    pattern.Key,
				"prefix": pattern.Prefix,
				"suffix": pattern.Suffix,
			})
		}

		// Also include regex patterns for advanced debugging
		regexPatterns := []map[string]interface{}{}
		for _, regexItem := range tsConfigParsed.AliasesRegexps {
			regexPatterns = append(regexPatterns, map[string]interface{}{
				"aliasKey": regexItem.AliasKey,
				"pattern":  regexItem.RegExp.String(),
			})
		}
		output["regexPatterns"] = regexPatterns

		// Marshal and print the result
		jsonOutput, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal output: %w", err)
		}

		fmt.Println(string(jsonOutput))
		return nil
	},
}

func init() {
	// browser flags
	addSharedFlags(browserCmd)
	browserCmd.Flags().StringVarP(&browserCwd, "cwd", "c", currentDir, "Working directory for the command")
	browserCmd.Flags().StringVar(&browserEntryPoint, "entry-point", "", "entry point file")
	browserCmd.Flags().BoolVarP(&browserIgnoreType, "ignore-types-imports", "t", false, "Exclude type imports from the analysis")
	browserCmd.MarkFlagRequired("entry-point")

	// debug-parse-file flags
	addSharedFlags(debugParseFileCmd)
	debugParseFileCmd.Flags().StringVar(&debugFile, "file", "", "file to parse")
	debugParseFileCmd.Flags().StringVar(&debugFileCwd, "cwd", currentDir, "Working directory for the command")
	debugParseFileCmd.MarkFlagRequired("file")

	// debug-get-tree-for-cwd flags
	addSharedFlags(debugGetTreeCmd)
	debugGetTreeCmd.Flags().StringVar(&debugTreeCwd, "cwd", currentDir, "Working directory for the command")
	debugGetTreeCmd.Flags().BoolVarP(&debugTreeIgnoreType, "ignore-type-imports", "t", false, "Exclude type imports from the analysis")

	// debug-tsconfig flags
	debugTsconfigCmd.Flags().StringVar(&debugTsconfigPath, "tsconfig", "", "Path to TypeScript configuration file")
	debugTsconfigCmd.MarkFlagRequired("tsconfig")

	rootCmd.AddCommand(browserCmd, debugParseFileCmd, debugGetTreeCmd, debugTsconfigCmd)
}
