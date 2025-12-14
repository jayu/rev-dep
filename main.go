package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var (
	currentDir, _ = os.Getwd()
	rootCmd       = &cobra.Command{
		Use:   "rev-dep",
		Short: "Analyze and visualize JavaScript/TypeScript project dependencies",
		Long: `A powerful tool for analyzing and visualizing dependencies in JavaScript and TypeScript projects. 
Helps identify circular dependencies, unused modules, and optimize project structure.`,
		Version: Version,
	}
)

var docsCmd = &cobra.Command{
	Use:   "doc-gen",
	Short: "Generate CLI documentation",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := doc.GenMarkdownTree(rootCmd, "./docs")
		if err != nil {
			log.Fatal(err)
		}
		return nil
	},
}

// ---------------- shared flags ----------------

var (
	packageJsonPath  string
	tsconfigJsonPath string
)

func addSharedFlags(command *cobra.Command) {
	command.Flags().StringVar(&packageJsonPath, "package-json", "",
		"Path to package.json (default: ./package.json)")
	command.Flags().StringVar(&tsconfigJsonPath, "tsconfig-json", "",
		"Path to tsconfig.json (default: ./tsconfig.json)")
}

// ---------------- resolve ----------------
var (
	resolveCwd            string
	resolveFile           string
	resolveEntryPoints    []string
	resolveGraphExclude   []string
	resolveIgnoreType     bool
	resolveAll            bool
	resolveCompactSummary bool
)

var resolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Trace and display the dependency path between files in your project",
	Long: `Analyze and display the dependency chain between specified files.
Helps understand how different parts of your codebase are connected.`,
	Example: "rev-dep resolve -p src/index.ts -f src/utils/helpers.ts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := ResolveAbsoluteCwd(resolveCwd)
		filePath := resolveFile
		var absolutePathToEntryPoints []string

		if len(resolveEntryPoints) > 0 {
			absolutePathToEntryPoints = make([]string, 0, len(resolveEntryPoints))
			for _, entryPoint := range resolveEntryPoints {
				absolutePathToEntryPoints = append(absolutePathToEntryPoints, filepath.Join(cwd, entryPoint))
			}
		}

		minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, resolveIgnoreType, resolveGraphExclude, absolutePathToEntryPoints, packageJsonPath, tsconfigJsonPath)

		if len(absolutePathToEntryPoints) == 0 {
			absolutePathToEntryPoints = GetEntryPoints(minimalTree, []string{}, []string{}, cwd)
		}

		absolutePathToFilePath := NormalizePathForInternal(filepath.Join(cwd, filePath))

		notFoundCount := 0
		for _, absolutePathToEntryPoint := range absolutePathToEntryPoints {
			if _, found := minimalTree[absolutePathToEntryPoint]; !found {
				notFoundCount++
				//return fmt.Errorf("could not find entry point '%s' in dependency tree", absolutePathToEntryPoints[idx])
			}
		}

		if _, found := minimalTree[absolutePathToFilePath]; !found {
			fmt.Printf("Error: Target file '%s' not found in dependency tree.\n", filePath)
			fmt.Println("Available files:")
			count := 0
			for path := range minimalTree {
				if count < 10 {
					cleanPath := strings.TrimPrefix(path, cwd)
					fmt.Printf("  %s\n", cleanPath)
					count++
				}
			}
			if len(minimalTree) > 10 {
				fmt.Printf("  ... and %d more files\n", len(minimalTree)-10)
			}
			os.Exit(1)
		}

		depsGraphs := make([]BuildDepsGraphResult, 0, len(absolutePathToEntryPoints))

		var wg sync.WaitGroup
		var mu sync.Mutex
		ch := make(chan string)

		buildGraph := func(absolutePathToEntryPoint string, depsGraphs *[]BuildDepsGraphResult, wg *sync.WaitGroup, mu *sync.Mutex) {
			depsGraph := buildDepsGraph(minimalTree, absolutePathToEntryPoint, &absolutePathToFilePath, resolveAll)
			mu.Lock()
			*depsGraphs = append(*depsGraphs, depsGraph)
			mu.Unlock()

			// Print this warning only if user provided entry points list
			if depsGraph.FileOrNodeModuleNode == nil && len(resolveEntryPoints) > 0 {
				fmt.Printf("Error: Could not find target file '%s' in dependency graph.\n", filePath)
			}
			wg.Done()
		}

		go func() {
			for absolutePathToEntryPoint := range ch {
				go buildGraph(absolutePathToEntryPoint, &depsGraphs, &wg, &mu)
			}
		}()

		for _, absolutePathToEntryPoint := range absolutePathToEntryPoints {
			wg.Add(1)
			ch <- absolutePathToEntryPoint
		}

		wg.Wait()
		close(ch)

		slices.SortFunc(depsGraphs, func(a BuildDepsGraphResult, b BuildDepsGraphResult) int {
			rootA := a.Root.Path
			rootB := b.Root.Path
			if rootA < rootB {
				return -1
			} else if rootA > rootB {
				return 1
			} else {
				return 0
			}
		})

		totalCount := 0

		fmt.Printf("\nDependency paths from entry points to '%s':\n\n", filePath)

		maxPathLen := 0

		for _, depsGraph := range depsGraphs {
			if len(depsGraph.ResolutionPaths) > 0 {
				length := len(strings.TrimPrefix(depsGraph.Root.Path, cwd))
				if length > maxPathLen {
					maxPathLen = length
				}
			}
		}

		for _, depsGraph := range depsGraphs {
			if len(depsGraph.ResolutionPaths) > 0 {
				resolvePathsCount := len(depsGraph.ResolutionPaths)
				totalCount += resolvePathsCount
				if resolveCompactSummary {
					p := PadRight(strings.TrimPrefix(depsGraph.Root.Path, cwd), ' ', maxPathLen)
					fmt.Println(p, ":", resolvePathsCount)
				} else {
					FormatPaths(depsGraph.ResolutionPaths, cwd)
				}
			}
		}
		if resolveCompactSummary {
			fmt.Println()
		}

		fmt.Printf("Total: %d\n", totalCount)

		return nil
	},
}

// ---------------- entry-points ----------------
var (
	entryPointsCwd               string
	entryPointsIgnoreType        bool
	entryPointsCount             bool
	entryPointsDependenciesCount bool
	entryPointsGraphExclude      []string
	entryPointsResultExclude     []string
	entryPointsResultInclude     []string
)

var entryPointsCmd = &cobra.Command{
	Use:   "entry-points",
	Short: "Discover and list all entry points in the project",
	Long: `Analyzes the project structure to identify all potential entry points.
Useful for understanding your application's architecture and dependencies.`,
	Example: "rev-dep entry-points --print-deps-count",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := ResolveAbsoluteCwd(entryPointsCwd)
		minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, entryPointsIgnoreType, entryPointsGraphExclude, []string{}, packageJsonPath, tsconfigJsonPath)

		notReferencedFiles := GetEntryPoints(minimalTree, entryPointsResultExclude, entryPointsResultInclude, cwd)

		if entryPointsCount {
			fmt.Println(len(notReferencedFiles))
			return nil
		}

		if !entryPointsDependenciesCount {
			for _, filePath := range notReferencedFiles {
				printPath := strings.TrimPrefix(filePath, cwd)
				fmt.Println(printPath)
			}
			return nil
		}

		depsCountMeta := make(map[string]int, len(notReferencedFiles))
		maxFilePathLen := 0
		var mu sync.Mutex
		var wg sync.WaitGroup

		for _, filePath := range notReferencedFiles {
			wg.Add(1)
			go func() {
				graph := buildDepsGraph(minimalTree, filePath, nil, false)
				mu.Lock()
				depsCountMeta[filePath] = len(graph.Vertices)
				if len(filePath) > maxFilePathLen {
					maxFilePathLen = len(filePath)
				}
				mu.Unlock()
				wg.Done()
			}()
		}

		wg.Wait()

		for _, filePath := range notReferencedFiles {
			printPath := strings.TrimPrefix(filePath, cwd)
			fmt.Println(PadRight(printPath, ' ', maxFilePathLen), depsCountMeta[filePath])
		}

		return nil
	},
}

// ---------------- circular ----------------
var (
	circularCwd        string
	circularIgnoreType bool
)

var circularCmd = &cobra.Command{
	Use:   "circular",
	Short: "Detect circular dependencies in your project",
	Long: `Analyzes the project to find circular dependencies between modules.
Circular dependencies can cause hard-to-debug issues and should generally be avoided.`,
	Example: "rev-dep circular --ignore-types-imports",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := ResolveAbsoluteCwd(circularCwd)
		excludeFiles := []string{}

		minimalTree, files, _ := GetMinimalDepsTreeForCwd(cwd, circularIgnoreType, excludeFiles, []string{}, packageJsonPath, tsconfigJsonPath)
		cycles := FindCircularDependencies(minimalTree, files)

		fmt.Fprint(os.Stderr, FormatCircularDependencies(cycles, cwd, minimalTree))

		if len(cycles) > 0 {
			os.Exit(len(cycles))
		}
		return nil
	},
}

// ---------------- node-modules ----------------
var (
	nodeModulesCwd                       string
	nodeModulesIgnoreType                bool
	nodeModulesEntryPoints               []string
	nodeModulesCountFlag                 bool
	nodeModulesZeroExitCode              bool
	nodeModulesGroupByModule             bool
	nodeModulesGroupByFile               bool
	nodeModulesPkgJsonFieldsWithBinaries []string
	nodeModulesFilesWithBinaries         []string
	nodeModulesFilesWithModules          []string
	nodeModulesIncludeModules            []string
	nodeModulesExcludeModules            []string
	nodeModulesShouldOptimize            bool
	nodeModulesVerbose                   bool
	nodeModulesSizeStats                 bool
	nodeModulesOptimizeIsolate           bool
)

var nodeModulesCmd = &cobra.Command{
	Use:   "node-modules",
	Short: "Analyze and manage Node.js dependencies",
	Long: `Tools for analyzing and managing Node.js module dependencies.
Helps identify unused, missing, or duplicate dependencies in your project.`,
	Example: `  rev-dep node-modules used -p src/index.ts
  rev-dep node-modules unused --exclude-modules=@types/*
  rev-dep node-modules missing --entry-points=src/main.ts`,
}

var nodeModulesUsedCmd = &cobra.Command{
	Use:   "used",
	Short: "List all npm packages imported in your code",
	Long: `Analyzes your code to identify which npm packages are actually being used.
Helps keep track of your project's runtime dependencies.`,
	Example: "rev-dep node-modules used -p src/index.ts --group-by-module",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, _ := NodeModulesCmd(
			ResolveAbsoluteCwd(nodeModulesCwd),
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			false,
			false,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
		)

		fmt.Print(result)

		return nil
	},
}

var nodeModulesUnusedCmd = &cobra.Command{
	Use:   "unused",
	Short: "Find installed packages that aren't imported in your code",
	Long: `Compares package.json dependencies with actual imports in your codebase
to identify potentially unused packages.`,
	Example: "rev-dep node-modules unused --exclude-modules=@types/*",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, count := NodeModulesCmd(
			ResolveAbsoluteCwd(nodeModulesCwd),
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			true,
			false,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
		)

		fmt.Print(result)

		if !nodeModulesZeroExitCode {
			os.Exit(count)
		}

		return nil
	},
}

var nodeModulesMissingCmd = &cobra.Command{
	Use:   "missing",
	Short: "Find imported packages not listed in package.json",
	Long: `Identifies packages that are imported in your code but not declared
in your package.json dependencies.`,
	Example: "rev-dep node-modules missing --entry-points=src/main.ts",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, count := NodeModulesCmd(
			ResolveAbsoluteCwd(nodeModulesCwd),
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			false,
			true,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
		)

		fmt.Print(result)

		if !nodeModulesZeroExitCode {
			os.Exit(count)
		}

		return nil
	},
}

var nodeModulesInstalledCmd = &cobra.Command{
	Use:   "installed",
	Short: "List all installed npm packages in the project",
	Long: `Recursively scans node_modules directories to list all installed packages.
Helpful for auditing dependencies across monorepos.`,
	Example: "rev-dep node-modules installed --include-modules=@myorg/*",
	RunE: func(cmd *cobra.Command, args []string) error {
		result := GetInstalledModulesCmd(
			ResolveAbsoluteCwd(nodeModulesCwd),
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
		)

		fmt.Print(result)

		return nil
	},
}

var nodeModulesInstalledDuplicatesCmd = &cobra.Command{
	Use:   "installed-duplicates",
	Short: "Find and optimize duplicate package installations",
	Long: `Identifies packages that are installed multiple times in node_modules.
Can optimize storage by creating symlinks between duplicate packages.`,
	Example: "rev-dep node-modules installed-duplicates --optimize --size-stats",
	RunE: func(cmd *cobra.Command, args []string) error {
		result := GetDuplicatedModulesCmd(
			ResolveAbsoluteCwd(nodeModulesCwd),
			nodeModulesShouldOptimize,
			nodeModulesVerbose,
			nodeModulesSizeStats,
			nodeModulesOptimizeIsolate,
		)

		fmt.Print(result)

		return nil
	},
}

var nodeModulesAnalyzeSize = &cobra.Command{
	Use:   "analyze-size",
	Short: "Analyze disk usage of node_modules",
	Long: `Provides detailed size analysis of node_modules directory.
Helps identify space-hogging dependencies.`,
	Example: "rev-dep node-modules analyze-size",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := ResolveAbsoluteCwd(nodeModulesCwd)
		modules, _ := GetInstalledModules(cwd, []string{}, []string{})
		results, err := AnalyzeNodeModules(cwd, modules)
		if err != nil {
			log.Fatalf("analysis failed: %v", err)
		}

		PrintAnalysis(results)
		return nil
	},
}

var nodeModuleDirsSize = &cobra.Command{
	Use:   "dirs-size",
	Short: "Calculates cumulative files size in node_modules directories",
	Long: `Calculates and displays the size of node_modules folders
in the current directory and subdirectories. Sizes will be smaller than actual file size taken on disk. Tool is calculating actual file size rather than file size on disk (related to disk blocks usage)`,
	Example: "rev-dep node-modules dirs-size",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := ResolveAbsoluteCwd(nodeModulesCwd)
		result := ModulesDiskSizeCmd(cwd)

		fmt.Println(result)
		return nil
	},
}

// ---------------- list-files ----------------
var (
	listFilesCwd     string
	listFilesInclude []string
	listFilesExclude []string
	listFilesCount   bool
)

var listCwdFilesCmd = &cobra.Command{
	Use:   "list-cwd-files",
	Short: "List all files in the current working directory",
	Long: `Recursively lists all files in the specified directory,
with options to filter results.`,
	Example: "rev-dep list-cwd-files --include='*.ts' --exclude='*.test.ts'",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := ResolveAbsoluteCwd(listFilesCwd)
		files := GetFiles(cwd, []string{}, FindAndProcessGitIgnoreFilesUpToRepoRoot(cwd))

		includeGlobs := CreateGlobMatchers(listFilesInclude, cwd)
		excludeGlobs := CreateGlobMatchers(listFilesExclude, cwd)
		count := 0

		for _, filePath := range files {
			if len(includeGlobs) == 0 || MatchesAnyGlobMatcher(filePath, includeGlobs, false) {
				isExcluded := MatchesAnyGlobMatcher(filePath, excludeGlobs, false)

				if !isExcluded {
					if listFilesCount {
						count++
					} else {
						// filePath is internal-normalized; convert to OS-native and compute relative path for printing
						filePathOs := DenormalizePathForOS(filePath)
						rel, _ := filepath.Rel(cwd, filePathOs)
						fmt.Println(rel)
					}
				}
			}
		}

		if listFilesCount {
			fmt.Println(count)
		}

		return nil
	},
}

// ---------------- files ----------------
var (
	filesCwd        string
	filesEntryPoint string
	filesIgnoreType bool
	filesCount      bool
)

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "List all files in the dependency tree of an entry point",
	Long: `Recursively finds and lists all files that are required
by the specified entry point.`,
	Example: "rev-dep files --entry-point src/index.ts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := ResolveAbsoluteCwd(filesCwd)
		absolutePathToEntryPoint := filepath.Join(cwd, filesEntryPoint)
		excludeFiles := []string{}

		minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, filesIgnoreType, excludeFiles, []string{absolutePathToEntryPoint}, packageJsonPath, tsconfigJsonPath)

		depsGraph := buildDepsGraph(minimalTree, absolutePathToEntryPoint, nil, false)

		if filesCount {
			fmt.Println(len(depsGraph.Vertices))
		} else {
			filePaths := make([]string, 0, len(depsGraph.Vertices))
			for _, node := range depsGraph.Vertices {
				// node.Path is internal-normalized; convert to OS path before computing relative path
				nodePathOs := DenormalizePathForOS(node.Path)
				relative, _ := filepath.Rel(cwd, nodePathOs)
				filePaths = append(filePaths, relative)
			}
			slices.Sort(filePaths)
			for _, filePath := range filePaths {
				fmt.Println(filePath)
			}
		}
		return nil
	},
}

var (
	locCwd string
)

var linesOfCodeCmd = &cobra.Command{
	Use:     "lines-of-code",
	Short:   "Count actual lines of code in the project excluding comments and blank lines",
	Example: "rev-dep lines-of-code",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := ResolveAbsoluteCwd(locCwd)

		files := GetFiles(cwd, []string{}, FindAndProcessGitIgnoreFilesUpToRepoRoot(cwd))
		ch := make(chan [3]int) // [lines, linesWithoutComments, linesWithoutTemplates]
		var wg sync.WaitGroup

		for _, filePath := range files {
			wg.Add(1)
			go func(filePath string, ch chan [3]int, wg *sync.WaitGroup) {
				// filePath is internal-normalized; convert back to OS-native for IO
				fileContent, err := os.ReadFile(DenormalizePathForOS(filePath))
				if err == nil {

					lines := bytes.Count(
						bytes.ReplaceAll(fileContent, []byte{'\n', '\n'}, []byte{'\n'}),
						[]byte{'\n'},
					)

					// Count lines without comments
					linesWithoutComments := bytes.Count(
						bytes.ReplaceAll(RemoveCommentsFromCode(fileContent), []byte{'\n', '\n'}, []byte{'\n'}),
						[]byte{'\n'},
					)

					// Count lines without template literals (and comments)
					linesWithoutTemplates := bytes.Count(
						bytes.ReplaceAll(RemoveTaggedTemplateLiterals(fileContent), []byte{'\n', '\n'}, []byte{'\n'}),
						[]byte{'\n'},
					)

					ch <- [3]int{lines, linesWithoutComments, linesWithoutTemplates}
				} else {
					ch <- [3]int{0, 0, 0}
				}
				wg.Done()
			}(filePath, ch, &wg)
		}

		go func() {
			wg.Wait()
			close(ch)
		}()

		totalLines := 0
		totalLinesWithoutComments := 0
		totalLinesWithoutTemplates := 0

		for counts := range ch {
			totalLines += counts[0]
			totalLinesWithoutComments += counts[1]
			totalLinesWithoutTemplates += counts[2]
		}

		// formatNumber formats an integer with underscores as thousand separators
		formatNumber := func(n int) string {
			s := fmt.Sprintf("%d", n)
			var b strings.Builder
			for i, r := range s {
				if i > 0 && (len(s)-i)%3 == 0 {
					b.WriteRune('_')
				}
				b.WriteRune(r)
			}
			return b.String()
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "Metric\tLines\tPercentage")
		fmt.Fprintln(w, "------\t-----\t----------")
		fmt.Fprintf(w, "Total lines\t%v\t100.00%%\n", formatNumber(totalLines))
		fmt.Fprintf(w, "Without comments\t%v\t%.2f%%\n",
			formatNumber(totalLinesWithoutComments),
			float64(totalLinesWithoutComments)/float64(totalLines)*100)
		fmt.Fprintf(w, "Without comments and template strings\t%v\t%.2f%%\n",
			formatNumber(totalLinesWithoutTemplates),
			float64(totalLinesWithoutTemplates)/float64(totalLines)*100)
		w.Flush()
		return nil
	},
}

func addNodeModulesIncludeExcludeFlags(command *cobra.Command) {
	command.Flags().StringSliceVarP(&nodeModulesIncludeModules, "include-modules", "i", []string{}, "list of modules to include in the output")
	command.Flags().StringSliceVarP(&nodeModulesExcludeModules, "exclude-modules", "e", []string{}, "list of modules to exclude from the output")
}

func addNodeModulesFlags(command *cobra.Command, skipGroupingFlags bool) {
	addSharedFlags(command)
	command.Flags().StringVarP(&nodeModulesCwd, "cwd", "c", currentDir,
		"Working directory for the command")
	command.Flags().StringSliceVarP(&nodeModulesEntryPoints, "entry-points", "p", []string{},
		"Entry point file(s) to start analysis from (default: auto-detected)")
	command.Flags().BoolVarP(&nodeModulesIgnoreType, "ignore-type-imports", "t", false,
		"Exclude type imports from the analysis")
	command.Flags().BoolVarP(&nodeModulesCountFlag, "count", "n", false,
		"Only display the count of modules")
	if !skipGroupingFlags {
		command.Flags().BoolVar(&nodeModulesGroupByModule, "group-by-module", false,
			"Organize output by npm package name")
		command.Flags().BoolVar(&nodeModulesGroupByFile, "group-by-file", false,
			"Organize output by project file path")
	}
	command.Flags().StringSliceVar(&nodeModulesPkgJsonFieldsWithBinaries, "pkg-fields-with-binaries", []string{},
		"Additional package.json fields to check for binary usages")
	command.Flags().StringSliceVarP(&nodeModulesFilesWithBinaries, "files-with-binaries", "b", []string{},
		"Additional files to search for binary usages. Use paths relative to cwd")
	command.Flags().StringSliceVarP(&nodeModulesFilesWithModules, "files-with-node-modules", "m", []string{},
		"Additional files to search for module imports. Use paths relative to cwd")
	addNodeModulesIncludeExcludeFlags(command)
}

func init() {
	// resolve flags
	addSharedFlags(resolveCmd)
	resolveCmd.Flags().StringVarP(&resolveCwd, "cwd", "c", currentDir,
		"Working directory for the command")
	resolveCmd.Flags().StringVarP(&resolveFile, "file", "f", "",
		"Target file to check for dependencies")
	resolveCmd.Flags().StringSliceVar(&resolveGraphExclude, "graph-exclude", []string{},
		"Glob patterns to exclude files from dependency analysis")
	resolveCmd.Flags().StringSliceVarP(&resolveEntryPoints, "entry-points", "p", []string{},
		"Entry point file(s) to start analysis from (default: auto-detected)")
	resolveCmd.Flags().BoolVarP(&resolveIgnoreType, "ignore-type-imports", "t", false,
		"Exclude type imports from the analysis")
	resolveCmd.Flags().BoolVarP(&resolveAll, "all", "a", false,
		"Show all possible resolution paths, not just the first one")
	resolveCmd.Flags().BoolVar(&resolveCompactSummary, "compact-summary", false,
		"Display a compact summary of found paths")
	resolveCmd.MarkFlagRequired("file")

	// entry-points flags
	addSharedFlags(entryPointsCmd)
	entryPointsCmd.Flags().StringVarP(&entryPointsCwd, "cwd", "c", currentDir,
		"Working directory for the command")
	entryPointsCmd.Flags().BoolVarP(&entryPointsIgnoreType, "ignore-type-imports", "t", false,
		"Exclude type imports from the analysis")
	entryPointsCmd.Flags().BoolVarP(&entryPointsCount, "count", "n", false,
		"Only display the number of entry points found")
	entryPointsCmd.Flags().BoolVar(&entryPointsDependenciesCount, "print-deps-count", false,
		"Show the number of dependencies for each entry point")
	entryPointsCmd.Flags().StringSliceVar(&entryPointsGraphExclude, "graph-exclude", []string{},
		"Exclude files matching these glob patterns from analysis")
	entryPointsCmd.Flags().StringSliceVar(&entryPointsResultExclude, "result-exclude", []string{},
		"Exclude files matching these glob patterns from results")
	entryPointsCmd.Flags().StringSliceVar(&entryPointsResultInclude, "result-include", []string{},
		"Only include files matching these glob patterns in results")

	// circular flags
	addSharedFlags(circularCmd)
	circularCmd.Flags().StringVarP(&circularCwd, "cwd", "c", currentDir,
		"Working directory for the command")
	circularCmd.Flags().BoolVarP(&circularIgnoreType, "ignore-type-imports", "t", false,
		"Exclude type imports from the analysis")

	// node-modules flags
	addNodeModulesFlags(nodeModulesUsedCmd, false)
	addNodeModulesFlags(nodeModulesUnusedCmd, true)
	nodeModulesUnusedCmd.Flags().BoolVar(&nodeModulesZeroExitCode, "zero-exit-code", false, "Use this flag to always return zero exit code")
	addNodeModulesFlags(nodeModulesMissingCmd, false)
	nodeModulesMissingCmd.Flags().BoolVar(&nodeModulesZeroExitCode, "zero-exit-code", false, "Use this flag to always return zero exit code")
	addNodeModulesIncludeExcludeFlags(nodeModulesInstalledCmd)
	nodeModulesInstalledCmd.Flags().StringVarP(&nodeModulesCwd, "cwd", "c", currentDir, "Working directory for the command")

	nodeModulesInstalledDuplicatesCmd.Flags().StringVarP(&nodeModulesCwd, "cwd", "c", currentDir,
		"Working directory for the command")
	nodeModulesInstalledDuplicatesCmd.Flags().BoolVar(&nodeModulesShouldOptimize, "optimize", false,
		"Automatically create symlinks to deduplicate packages")
	nodeModulesInstalledDuplicatesCmd.Flags().BoolVar(&nodeModulesVerbose, "verbose", false,
		"Show detailed information about each optimization")
	nodeModulesInstalledDuplicatesCmd.Flags().BoolVar(&nodeModulesSizeStats, "size-stats", false, "Print node modules dirs size before and after optimization. Might take longer than optimization itself")
	nodeModulesInstalledDuplicatesCmd.Flags().BoolVar(&nodeModulesOptimizeIsolate, "isolate", false, "Create symlinks only within the same top-level node_module directories. By default optimize creates symlinks between top-level node_module directories (eg. when workspaces are used). Needs --optimize flag to take effect")

	nodeModulesAnalyzeSize.Flags().StringVarP(&nodeModulesCwd, "cwd", "c", currentDir, "Working directory for the command")
	nodeModuleDirsSize.Flags().StringVarP(&nodeModulesCwd, "cwd", "c", currentDir, "Working directory for the command")

	// node modules commands
	nodeModulesCmd.AddCommand(nodeModulesUsedCmd, nodeModulesUnusedCmd, nodeModulesMissingCmd, nodeModulesInstalledCmd, nodeModulesInstalledDuplicatesCmd, nodeModulesAnalyzeSize, nodeModuleDirsSize)

	// list-files flags
	listCwdFilesCmd.Flags().StringVar(&listFilesCwd, "cwd", currentDir,
		"Directory to list files from")
	listCwdFilesCmd.Flags().StringSliceVar(&listFilesExclude, "exclude", []string{},
		"Exclude files matching these glob patterns")
	listCwdFilesCmd.Flags().StringSliceVar(&listFilesInclude, "include", []string{},
		"Only include files matching these glob patterns")
	listCwdFilesCmd.Flags().BoolVar(&listFilesCount, "count", false,
		"Only display the count of matching files")

	// files flags
	addSharedFlags(filesCmd)
	filesCmd.Flags().StringVarP(&filesCwd, "cwd", "c", currentDir,
		"Working directory for the command")
	filesCmd.Flags().StringVarP(&filesEntryPoint, "entry-point", "p", "",
		"Entry point file to analyze (required)")
	filesCmd.Flags().BoolVarP(&filesIgnoreType, "ignore-type-imports", "t", false,
		"Exclude type imports from the analysis")
	filesCmd.Flags().BoolVarP(&filesCount, "count", "n", false,
		"Only display the count of files in the dependency tree")
	filesCmd.MarkFlagRequired("entry-point")

	// lines-of-code flags
	linesOfCodeCmd.Flags().StringVarP(&locCwd, "cwd", "c", currentDir,
		"Directory to analyze")

	// add commands
	rootCmd.AddCommand(resolveCmd, entryPointsCmd, circularCmd, nodeModulesCmd, listCwdFilesCmd, filesCmd, linesOfCodeCmd, docsCmd)
}

func main() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func GetMinimalDepsTreeForCwd(cwd string, ignoreTypeImports bool, excludeFiles []string, upfrontFilesList []string, packageJson string, tsconfigJson string) (MinimalDependencyTree, []string, map[string]bool) {
	var files []string

	excludePatterns := CreateGlobMatchers(excludeFiles, cwd)

	gitIgnoreExcludePatterns := FindAndProcessGitIgnoreFilesUpToRepoRoot(cwd)

	allExcludePatterns := append(excludePatterns, gitIgnoreExcludePatterns...)

	// Resolver is capable of starting with just one file and discover other files as it iterates.
	// While it's faster than looking up for all files upfront, if the file list for entry point is small, it's slower if file list for entry point is long, as resolver is not concurrent
	// To leverage that we have to make resolver concurrent using channels as queue
	if len(upfrontFilesList) == 0 {
		files = GetFiles(cwd, []string{}, allExcludePatterns)
	} else {
		files = upfrontFilesList
	}

	fileImportsArr, _ := ParseImportsFromFiles(files, ignoreTypeImports)

	slices.Sort(files)

	skipResolveMissing := false

	fileImportsArr, sortedFiles, nodeModules := ResolveImports(fileImportsArr, files, cwd, ignoreTypeImports, skipResolveMissing, packageJson, tsconfigJson, allExcludePatterns)

	minimalTree := TransformToMinimalDependencyTreeCustomParser(fileImportsArr)

	return minimalTree, sortedFiles, nodeModules
}
