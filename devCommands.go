//go:build dev
// +build dev

package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
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
		cwd := ResolveAbsoluteCwd(browserCwd)
		absolutePathToEntryPoint := JoinWithCwd(cwd, browserEntryPoint)
		excludeFiles := []string{}

		minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, browserIgnoreType, excludeFiles, []string{absolutePathToEntryPoint}, packageJsonPath, tsconfigJsonPath, conditionNames, followMonorepoPackages)

		StartServer(minimalTree, absolutePathToEntryPoint, cwd)
		return nil
	},
}

type MinimalDependencyWithLabels struct {
	ID                *string            `json:"id"`
	Request           string             `json:"request"`
	ResolvedType      ResolvedImportType `json:"resolvedType"`
	ResolvedTypeLabel string             `json:"resolvedTypeLabel"`
	ImportKind        *ImportKind        `json:"importKind"`
	ImportKindLabel   string             `json:"importKindLabel"`
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
		cwd := ResolveAbsoluteCwd(debugFileCwd)
		path := JoinWithCwd(cwd, debugFile)
		excludeFiles := []string{}

		minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, debugTreeIgnoreType, excludeFiles, []string{path}, packageJsonPath, tsconfigJsonPath, conditionNames, followMonorepoPackages)

		fmt.Println(path)

		minimalDep := minimalTree[path]
		for _, dep := range minimalDep {
			depWithLabels := MinimalDependencyWithLabels{
				ID:                dep.ID,
				Request:           dep.Request,
				ResolvedType:      dep.ResolvedType,
				ResolvedTypeLabel: ResolvedImportTypeToString(dep.ResolvedType),
				ImportKind:        dep.ImportKind,
			}
			if dep.ImportKind != nil {
				depWithLabels.ImportKindLabel = ImportKindToString(*dep.ImportKind)
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
		cwd := ResolveAbsoluteCwd(debugTreeCwd)
		excludeFiles := []string{}

		minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, debugTreeIgnoreType, excludeFiles, []string{}, packageJsonPath, tsconfigJsonPath, conditionNames, followMonorepoPackages)

		treeWithLabels := make(map[string][]MinimalDependencyWithLabels)
		for key, deps := range minimalTree {
			var depsWithLabels []MinimalDependencyWithLabels
			for _, dep := range deps {
				dwl := MinimalDependencyWithLabels{
					ID:                dep.ID,
					Request:           dep.Request,
					ResolvedType:      dep.ResolvedType,
					ResolvedTypeLabel: ResolvedImportTypeToString(dep.ResolvedType),
					ImportKind:        dep.ImportKind,
				}
				if dep.ImportKind != nil {
					dwl.ImportKindLabel = ImportKindToString(*dep.ImportKind)
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

	rootCmd.AddCommand(browserCmd, debugParseFileCmd, debugGetTreeCmd)
}
