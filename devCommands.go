//go:build dev
// +build dev

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

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
		absolutePathToEntryPoint := filepath.Join(cwd, browserEntryPoint)
		excludeFiles := []string{}

		minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, browserIgnoreType, excludeFiles, []string{absolutePathToEntryPoint}, packageJsonPath, tsconfigJsonPath)

		StartServer(minimalTree, absolutePathToEntryPoint, cwd)
		return nil
	},
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
		path := filepath.Join(cwd, debugFile)
		excludeFiles := []string{}

		minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, debugTreeIgnoreType, excludeFiles, []string{path}, packageJsonPath, tsconfigJsonPath)

		fmt.Println(path)
		jsonTree, err := json.MarshalIndent(minimalTree[path], "", " ")
		if err == nil {
			fmt.Println(string(jsonTree))
		}
		return err
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

		minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, debugTreeIgnoreType, excludeFiles, []string{}, packageJsonPath, tsconfigJsonPath)

		jsonTree, err := json.MarshalIndent(minimalTree, "", " ")
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
	debugParseFileCmd.Flags().StringVar(&debugFile, "file", "", "file to parse")
	debugParseFileCmd.Flags().StringVar(&debugFileCwd, "cwd", currentDir, "Working directory for the command")
	debugParseFileCmd.MarkFlagRequired("file")

	// debug-get-tree-for-cwd flags
	debugGetTreeCmd.Flags().StringVar(&debugTreeCwd, "cwd", currentDir, "Working directory for the command")
	debugGetTreeCmd.Flags().BoolVarP(&debugTreeIgnoreType, "ignore-type-imports", "t", false, "Exclude type imports from the analysis")

	rootCmd.AddCommand(browserCmd, debugParseFileCmd, debugGetTreeCmd)
}
