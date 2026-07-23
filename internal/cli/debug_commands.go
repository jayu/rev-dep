package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"rev-dep-go/internal/model"
	"rev-dep-go/internal/pathutil"
	"rev-dep-go/internal/resolve"
)

type MinimalDependencyWithLabels struct {
	ID                string                   `json:"id"`
	Request           string                   `json:"request"`
	ResolvedType      model.ResolvedImportType `json:"resolvedType"`
	ResolvedTypeLabel string                   `json:"resolvedTypeLabel"`
	ImportKind        model.ImportKind         `json:"importKind"`
	ImportKindLabel   string                   `json:"importKindLabel"`
}

// ---------------- debug ----------------
var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Debugging tools to inspect parser and resolver internals",
	Long:  `Debugging tools to inspect how rev-dep parses files and resolves dependencies. Output does not follow semver.`,
}

// ---------------- debug parse-file ----------------
var (
	debugFile    string
	debugFileCwd string
)

var debugParseFileCmd = &cobra.Command{
	Use:   "parse-file",
	Short: "Debug: Show parsed imports for a single file",
	Long:  `Debugging tool to inspect how the parser processes a specific file. Output does not follow semver.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		nodeModulesStrategy, err := nodeModulesResolutionStrategy()
		if err != nil {
			return err
		}
		cwd := pathutil.ResolveAbsoluteCwd(debugFileCwd)
		path := pathutil.JoinWithCwd(cwd, debugFile)
		excludeFiles := []string{}

		minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, debugTreeIgnoreType, excludeFiles, nil, []string{path}, tsconfigJsonPath, conditionNames, followValue, nil, nodeModulesStrategy)

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

// ---------------- debug get-tree-for-cwd ----------------
var (
	debugTreeCwd        string
	debugTreeIgnoreType bool
)

var debugGetTreeCmd = &cobra.Command{
	Use:   "get-tree-for-cwd",
	Short: "Debug: Show complete dependency tree for analysis",
	Long:  `Debugging tool to inspect the complete dependency tree. Output does not follow semver.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		nodeModulesStrategy, err := nodeModulesResolutionStrategy()
		if err != nil {
			return err
		}
		cwd := pathutil.ResolveAbsoluteCwd(debugTreeCwd)
		excludeFiles := []string{}

		minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, debugTreeIgnoreType, excludeFiles, nil, []string{}, tsconfigJsonPath, conditionNames, followValue, nil, nodeModulesStrategy)

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

// ---------------- debug parse-tsconfig ----------------
var (
	debugTsconfigPath string
)

var debugTsconfigCmd = &cobra.Command{
	Use:   "parse-tsconfig",
	Short: "Debug: Show parsed TypeScript configuration aliases",
	Long:  `Debugging tool to inspect how TypeScript configuration is parsed and what aliases are extracted. Output does not follow semver.`,
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

// ---------------- debug list-cwd-files ----------------
// This mirrors the root-level `list-cwd-files` command. The root command is kept
// for semver compatibility; this is the preferred home going forward. It reuses the
// same core function and flag variables (only one command executes per invocation,
// so sharing the flag vars is safe).
var debugListCwdFilesCmd = &cobra.Command{
	Use:   "list-cwd-files",
	Short: "List all files in the current working directory",
	Long: `Recursively lists all files in the specified directory,
with options to filter results.`,
	Example: "rev-dep debug list-cwd-files --include='*.ts' --exclude='*.test.ts'",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listCwdFilesCmdFn(
			pathutil.ResolveAbsoluteCwd(listFilesCwd),
			listFilesInclude,
			listFilesExclude,
			listFilesCount,
		)
	},
}

func init() {

	// debug parse-file flags
	addSharedFlags(debugParseFileCmd)
	debugParseFileCmd.Flags().StringVar(&debugFile, "file", "", "file to parse")
	debugParseFileCmd.Flags().StringVar(&debugFileCwd, "cwd", currentDir, "Working directory for the command")
	debugParseFileCmd.MarkFlagRequired("file")
	addNodeModulesResolutionFlag(debugParseFileCmd)

	// debug get-tree-for-cwd flags
	addSharedFlags(debugGetTreeCmd)
	debugGetTreeCmd.Flags().StringVar(&debugTreeCwd, "cwd", currentDir, "Working directory for the command")
	debugGetTreeCmd.Flags().BoolVarP(&debugTreeIgnoreType, "ignore-type-imports", "t", false, "Exclude type imports from the analysis")
	addNodeModulesResolutionFlag(debugGetTreeCmd)

	// debug parse-tsconfig flags
	debugTsconfigCmd.Flags().StringVar(&debugTsconfigPath, "tsconfig", "", "Path to TypeScript configuration file")
	debugTsconfigCmd.MarkFlagRequired("tsconfig")

	// debug list-cwd-files flags (mirror of the root-level command)
	debugListCwdFilesCmd.Flags().StringVar(&listFilesCwd, "cwd", currentDir, "Directory to list files from")
	debugListCwdFilesCmd.Flags().StringSliceVar(&listFilesExclude, "exclude", []string{}, "Exclude files matching these glob patterns")
	debugListCwdFilesCmd.Flags().StringSliceVar(&listFilesInclude, "include", []string{}, "Only include files matching these glob patterns")
	debugListCwdFilesCmd.Flags().BoolVar(&listFilesCount, "count", false, "Only display the count of matching files")

	debugCmd.AddCommand(debugParseFileCmd, debugGetTreeCmd, debugTsconfigCmd, debugListCwdFilesCmd)
	rootCmd.AddCommand(debugCmd)
}
