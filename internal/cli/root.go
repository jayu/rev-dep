package cli

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/config"
	"rev-dep-go/internal/diag"
	"rev-dep-go/internal/fs"
	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/graph"
	"rev-dep-go/internal/model"
	"rev-dep-go/internal/node"
	"rev-dep-go/internal/pathutil"
	"rev-dep-go/internal/resolve"
	"rev-dep-go/internal/source"
	"rev-dep-go/internal/version"
)

var (
	currentDir, _ = os.Getwd()
	rootCmd       = &cobra.Command{
		Use:   "rev-dep",
		Short: "Analyze and visualize JavaScript/TypeScript project dependencies",
		Long: `A powerful tool for analyzing and visualizing dependencies in JavaScript and TypeScript projects. 
Helps identify circular dependencies, unused modules, and optimize project structure.`,
		Version: version.Version,
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
	packageJsonPath        string
	tsconfigJsonPath       string
	verboseFlag            bool
	conditionNames         []string
	followMonorepoPackages []string
)

const followMonorepoPackagesAllSentinel = "__REV_DEP_FOLLOW_ALL__"

func sanitizeFlagSentinelInHelpOutput(helpOutput string) string {
	followAllToken := "[=" + followMonorepoPackagesAllSentinel + "]"
	return strings.ReplaceAll(helpOutput, followAllToken, strings.Repeat(" ", len(followAllToken)))
}

func installHelpOutputSanitizer(command *cobra.Command) {
	defaultHelpFunc := command.HelpFunc()
	defaultUsageFunc := command.UsageFunc()

	command.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		var captured bytes.Buffer
		originalOut := cmd.OutOrStdout()
		originalErr := cmd.ErrOrStderr()
		cmd.SetOut(&captured)
		cmd.SetErr(&captured)
		defer func() {
			cmd.SetOut(originalOut)
			cmd.SetErr(originalErr)
		}()

		defaultHelpFunc(cmd, args)
		fmt.Fprint(originalOut, sanitizeFlagSentinelInHelpOutput(captured.String()))
	})

	command.SetUsageFunc(func(cmd *cobra.Command) error {
		var captured bytes.Buffer
		originalOut := cmd.OutOrStdout()
		originalErr := cmd.ErrOrStderr()
		cmd.SetOut(&captured)
		cmd.SetErr(&captured)
		defer func() {
			cmd.SetOut(originalOut)
			cmd.SetErr(originalErr)
		}()

		usageErr := defaultUsageFunc(cmd)
		_, writeErr := fmt.Fprint(originalErr, sanitizeFlagSentinelInHelpOutput(captured.String()))
		if usageErr != nil {
			return usageErr
		}
		return writeErr
	})
}

func addSharedFlags(command *cobra.Command) {
	command.Flags().StringVar(&packageJsonPath, "package-json", "",
		"Path to package.json (default: ./package.json)")
	command.Flags().StringVar(&tsconfigJsonPath, "tsconfig-json", "",
		"Path to tsconfig.json (default: ./tsconfig.json)")
	command.Flags().BoolVarP(&verboseFlag, "verbose", "v", false,
		"Show warnings and verbose output")
	command.Flags().StringSliceVar(&conditionNames, "condition-names", []string{},
		"List of conditions for package.json imports resolution (e.g. node, imports, default)")
	command.Flags().StringSliceVar(&followMonorepoPackages, "follow-monorepo-packages", []string{},
		"Enable resolution of imports from monorepo workspace packages. Pass without value to follow all, or pass package names")
	command.Flags().Lookup("follow-monorepo-packages").NoOptDefVal = followMonorepoPackagesAllSentinel
}

func getFollowMonorepoPackagesValue(cmd *cobra.Command) (model.FollowMonorepoPackagesValue, error) {
	if !cmd.Flags().Changed("follow-monorepo-packages") {
		return model.FollowMonorepoPackagesValue{}, nil
	}

	if len(followMonorepoPackages) == 0 {
		return model.FollowMonorepoPackagesValue{FollowAll: true}, nil
	}

	trimmedValues := make(map[string]bool, len(followMonorepoPackages))
	for i, value := range followMonorepoPackages {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			return model.FollowMonorepoPackagesValue{}, fmt.Errorf("invalid --follow-monorepo-packages value at index %d: expected non-empty package name", i)
		}
		trimmedValues[trimmedValue] = true
	}

	if len(trimmedValues) == 1 && trimmedValues[followMonorepoPackagesAllSentinel] {
		return model.FollowMonorepoPackagesValue{FollowAll: true}, nil
	}

	return model.FollowMonorepoPackagesValue{Packages: trimmedValues}, nil
}

// ---------------- resolve ----------------
var (
	resolveCwd            string
	resolveFile           string
	resolveModule         string
	resolveEntryPoints    []string
	resolveGraphExclude   []string
	resolveIgnoreType     bool
	resolveAll            bool
	resolveCompactSummary bool
)

func resolveCmdFn(cwd, filePath, moduleName string, entryPoints, graphExclude []string, ignoreType, resolveAll, resolveCompactSummary bool, packageJsonPath, tsconfigJsonPath string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue) error {
	hasFile := strings.TrimSpace(filePath) != ""
	hasModule := strings.TrimSpace(moduleName) != ""

	if hasFile == hasModule {
		return fmt.Errorf("exactly one of --file or --module must be provided")
	}

	absolutePathToEntryPoints, discoveredFiles := resolve.ResolveEntryPointsFromPatterns(cwd, entryPoints, graphExclude, nil)
	minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, ignoreType, graphExclude, nil, discoveredFiles, packageJsonPath, tsconfigJsonPath, conditionNames, followMonorepoPackages, nil)

	if len(absolutePathToEntryPoints) == 0 {
		absolutePathToEntryPoints = graph.GetEntryPoints(minimalTree, []string{}, []string{}, cwd)
	}

	targetDisplayName := filePath
	targetNodeOrModuleName := ""
	if hasFile {
		targetNodeOrModuleName = pathutil.NormalizePathForInternal(pathutil.JoinWithCwd(cwd, filePath))
	} else {
		targetNodeOrModuleName = strings.TrimSpace(moduleName)
		targetDisplayName = targetNodeOrModuleName
	}

	notFoundCount := 0
	for _, absolutePathToEntryPoint := range absolutePathToEntryPoints {
		if _, found := minimalTree[absolutePathToEntryPoint]; !found {
			notFoundCount++
			//return fmt.Errorf("could not find entry point '%s' in dependency tree", absolutePathToEntryPoints[idx])
		}
	}

	if hasFile {
		if _, found := minimalTree[targetNodeOrModuleName]; !found {
			fmt.Printf("Error: Target file '%s' ('%s') not found in dependency tree.\n", filePath, targetNodeOrModuleName)
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
	}

	type RootAndResolutionPaths struct {
		Root                 *graph.SerializableNode `json:"root"`
		ResolutionPaths      [][]string              `json:"resolutionPaths,omitempty"`
		FileOrNodeModuleNode *graph.SerializableNode `json:"fileOrNodeModuleNode"`
	}

	depsGraphs := make([]RootAndResolutionPaths, 0, len(absolutePathToEntryPoints))

	var wg sync.WaitGroup
	var mu sync.Mutex
	ch := make(chan string)

	buildGraph := func(absolutePathToEntryPoint string, depsGraphs *[]RootAndResolutionPaths, wg *sync.WaitGroup, mu *sync.Mutex) {
		// We cannot use multiple entry points here as it will break the reverse resolution.
		// Reverse resolution which only looks for the first possible path, must have only one entry point
		// Otherwise it may follow wrong path and not find the result
		depsGraphsTemp := graph.BuildDepsGraphForMultiple(minimalTree, []string{absolutePathToEntryPoint}, &targetNodeOrModuleName, resolveAll, false)
		depsGraph := RootAndResolutionPaths{
			Root:                 depsGraphsTemp.Roots[absolutePathToEntryPoint],
			ResolutionPaths:      depsGraphsTemp.ResolutionPaths[absolutePathToEntryPoint],
			FileOrNodeModuleNode: depsGraphsTemp.FileOrNodeModuleNode,
		}
		if hasModule && depsGraph.FileOrNodeModuleNode != nil && depsGraph.FileOrNodeModuleNode.LookedUpNodeModuleImportRequest != "" && len(depsGraph.ResolutionPaths) == 0 {
			// Mirror previous resolve output expectations when entry point itself imports the target module.
			depsGraph.ResolutionPaths = [][]string{{absolutePathToEntryPoint}}
		}

		mu.Lock()
		*depsGraphs = append(*depsGraphs, depsGraph)
		mu.Unlock()

		// Print this warning only if user provided entry points list
		if len(entryPoints) > 0 {
			if hasFile && depsGraph.FileOrNodeModuleNode == nil {
				diag.Warnf("Error: Could not find target file '%s' in dependency graph.\n", filePath)
			}
			if hasModule && (depsGraph.FileOrNodeModuleNode == nil || depsGraph.FileOrNodeModuleNode.LookedUpNodeModuleImportRequest == "") {
				diag.Warnf("Error: Could not find a matching module import in dependency graph.\n")
			}
		}
		wg.Done()
	}

	// Limit concurrency
	maxConcurrency := runtime.GOMAXPROCS(0) * 2
	sem := make(chan struct{}, maxConcurrency)

	go func() {
		for absolutePathToEntryPoint := range ch {
			sem <- struct{}{} // Acquire
			go func(ep string) {
				defer func() { <-sem }() // Release
				buildGraph(ep, &depsGraphs, &wg, &mu)
			}(absolutePathToEntryPoint)
		}
	}()

	for _, absolutePathToEntryPoint := range absolutePathToEntryPoints {
		wg.Add(1)
		ch <- absolutePathToEntryPoint
	}

	wg.Wait()
	close(ch)

	slices.SortFunc(depsGraphs, func(a RootAndResolutionPaths, b RootAndResolutionPaths) int {
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

	if hasFile {
		fmt.Printf("\nDependency paths from entry points to '%s':\n\n", targetDisplayName)
	} else {
		fmt.Printf("\nDependency paths from entry points to matching module imports:\n\n")
	}

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
				p := source.PadRight(strings.TrimPrefix(depsGraph.Root.Path, cwd), ' ', maxPathLen)
				fmt.Println(p, ":", resolvePathsCount)
			} else {
				if hasModule {
					additionalItem := ""
					if depsGraph.FileOrNodeModuleNode != nil {
						additionalItem = depsGraph.FileOrNodeModuleNode.LookedUpNodeModuleImportRequest
					}
					graph.FormatPathsWithAdditionalItem(depsGraph.ResolutionPaths, cwd, additionalItem)
				} else {
					graph.FormatPaths(depsGraph.ResolutionPaths, cwd)
				}
			}
		}
	}
	if resolveCompactSummary {
		fmt.Println()
	}

	fmt.Printf("Total: %d\n", totalCount)

	return nil
}

var resolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Trace and display the dependency path between files in your project",
	Long: `Analyze and display the dependency chain between specified files.
Helps understand how different parts of your codebase are connected.`,
	Example: "rev-dep resolve -p src/index.ts -f src/utils/helpers.ts",
	RunE: func(cmd *cobra.Command, args []string) error {
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		return resolveCmdFn(
			pathutil.ResolveAbsoluteCwd(resolveCwd),
			resolveFile,
			resolveModule,
			resolveEntryPoints,
			resolveGraphExclude,
			resolveIgnoreType,
			resolveAll,
			resolveCompactSummary,
			packageJsonPath,
			tsconfigJsonPath,
			conditionNames,
			followValue,
		)
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

func entryPointsCmdFn(cwd string, ignoreType, entryPointsCount, entryPointsDependenciesCount bool, graphExclude, resultExclude, resultInclude []string, packageJsonPath, tsconfigJsonPath string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue) error {
	minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, ignoreType, graphExclude, nil, []string{}, packageJsonPath, tsconfigJsonPath, conditionNames, followMonorepoPackages, nil)

	notReferencedFiles := graph.GetEntryPoints(minimalTree, resultExclude, resultInclude, cwd)

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

	maxFilePathLen := 0

	multiGraph := graph.BuildDepsGraphForMultiple(minimalTree, notReferencedFiles, nil, false, false)

	depsCountMeta := make(map[string]int, len(notReferencedFiles))

	// Parallel BST processing for each entry point
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Limit concurrency
	maxConcurrency := runtime.GOMAXPROCS(0) * 2
	sem := make(chan struct{}, maxConcurrency)

	for entryPoint, root := range multiGraph.Roots {
		wg.Add(1)
		sem <- struct{}{} // Acquire
		go func(ep string, r *graph.SerializableNode) {
			defer func() { <-sem }() // Release
			vertices := graph.BST(r, multiGraph.Vertices)
			printPath := strings.TrimPrefix(ep, cwd)

			mu.Lock()
			if len(printPath) > maxFilePathLen {
				maxFilePathLen = len(printPath)
			}
			depsCountMeta[ep] = len(vertices)
			mu.Unlock()
			wg.Done()
		}(entryPoint, root)
	}

	wg.Wait()

	for _, filePath := range notReferencedFiles {
		printPath := strings.TrimPrefix(filePath, cwd)

		fmt.Println(source.PadRight(printPath, ' ', maxFilePathLen), depsCountMeta[filePath])
	}

	return nil
}

var entryPointsCmd = &cobra.Command{
	Use:   "entry-points",
	Short: "Discover and list all entry points in the project",
	Long: `Analyzes the project structure to identify all potential entry points.
Useful for understanding your application's architecture and dependencies.`,
	Example: "rev-dep entry-points --print-deps-count",
	RunE: func(cmd *cobra.Command, args []string) error {
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		return entryPointsCmdFn(
			pathutil.ResolveAbsoluteCwd(entryPointsCwd),
			entryPointsIgnoreType,
			entryPointsCount,
			entryPointsDependenciesCount,
			entryPointsGraphExclude,
			entryPointsResultExclude,
			entryPointsResultInclude,
			packageJsonPath,
			tsconfigJsonPath,
			conditionNames,
			followValue,
		)
	},
}

// ---------------- circular ----------------
var (
	circularCwd        string
	circularIgnoreType bool
	circularAlgorithm  string
)

func circularCmdFn(cwd string, ignoreType bool, packageJsonPath, tsconfigJsonPath string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue) (int, error) {
	excludeFiles := []string{}

	minimalTree, files, _ := resolve.GetMinimalDepsTreeForCwd(cwd, ignoreType, excludeFiles, nil, []string{}, packageJsonPath, tsconfigJsonPath, conditionNames, followMonorepoPackages, nil)
	algo := strings.ToLower(strings.TrimSpace(circularAlgorithm))
	if algo == "" {
		algo = "dfs"
	}

	var cycles [][]string
	switch algo {
	case "dfs":
		cycles = checks.FindCircularDependencies(minimalTree, files, ignoreType)
	case "scc":
		cycles = checks.FindCircularDependenciesSCC(minimalTree, files, ignoreType)
	default:
		return 0, fmt.Errorf("invalid value for --algorithm: %q (allowed: DFS, SCC)", circularAlgorithm)
	}

	formatted := checks.FormatCircularDependencies(cycles, cwd, minimalTree)
	if len(cycles) > 0 {
		fmt.Fprint(os.Stderr, formatted)
	} else {
		fmt.Fprint(os.Stdout, formatted)
	}

	return len(cycles), nil
}

var circularCmd = &cobra.Command{
	Use:   "circular",
	Short: "Detect circular dependencies in your project",
	Long: `Analyzes the project to find circular dependencies between modules.
Circular dependencies can cause hard-to-debug issues and should generally be avoided.`,
	Example: "rev-dep circular --ignore-types-imports",
	RunE: func(cmd *cobra.Command, args []string) error {
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		count, err := circularCmdFn(
			pathutil.ResolveAbsoluteCwd(circularCwd),
			circularIgnoreType,
			packageJsonPath,
			tsconfigJsonPath,
			conditionNames,
			followValue,
		)
		if err != nil {
			return err
		}
		if count > 0 {
			os.Exit(count)
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
	nodeModulesGroupByModuleFilesCount   bool
	nodeModulesGroupByEntryPoint         bool
	nodeModulesGroupByEntryPointModCount bool
	nodeModulesGroupByModuleShowEntries  bool
	nodeModulesGroupByModuleEntriesCount bool
	nodeModulesPkgJsonFieldsWithBinaries []string
	nodeModulesFilesWithBinaries         []string
	nodeModulesFilesWithModules          []string
	nodeModulesIncludeModules            []string
	nodeModulesExcludeModules            []string
	nodeModulesShouldOptimize            bool
	nodeModulesVerbose                   bool
	nodeModulesSizeStats                 bool
	nodeModulesOptimizeIsolate           bool
	nodeModulesPrunePatterns             []string
	nodeModulesPruneDefaults             bool
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
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		result, _ := node.NodeModulesCmd(
			pathutil.ResolveAbsoluteCwd(nodeModulesCwd),
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			false,
			false,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesGroupByModuleFilesCount,
			nodeModulesGroupByEntryPoint,
			nodeModulesGroupByEntryPointModCount,
			nodeModulesGroupByModuleShowEntries,
			nodeModulesGroupByModuleEntriesCount,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			packageJsonPath,
			tsconfigJsonPath,
			conditionNames,
			followValue,
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
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		result, count := node.NodeModulesCmd(
			pathutil.ResolveAbsoluteCwd(nodeModulesCwd),
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			true,
			false,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesGroupByModuleFilesCount,
			false,
			false,
			false,
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			packageJsonPath,
			tsconfigJsonPath,
			conditionNames,
			followValue,
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
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		result, count := node.NodeModulesCmd(
			pathutil.ResolveAbsoluteCwd(nodeModulesCwd),
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			false,
			true,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesGroupByModuleFilesCount,
			false,
			false,
			false,
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			packageJsonPath,
			tsconfigJsonPath,
			conditionNames,
			followValue,
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
		result := node.GetInstalledModulesCmd(
			pathutil.ResolveAbsoluteCwd(nodeModulesCwd),
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
		result := node.GetDuplicatedModulesCmd(
			pathutil.ResolveAbsoluteCwd(nodeModulesCwd),
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
		cwd := pathutil.ResolveAbsoluteCwd(nodeModulesCwd)
		modules, _ := node.GetInstalledModules(cwd, []string{}, []string{})
		results, err := node.AnalyzeNodeModules(cwd, modules)
		if err != nil {
			log.Fatalf("analysis failed: %v", err)
		}

		node.PrintAnalysis(results)
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
		cwd := pathutil.ResolveAbsoluteCwd(nodeModulesCwd)
		result := node.ModulesDiskSizeCmd(cwd)

		fmt.Println(result)
		return nil
	},
}

var nodeModulesPruneDocsCmd = &cobra.Command{
	Use:     "prune-docs",
	Aliases: []string{"remove-docs"},
	Short:   "Remove markdown/docs-like files from installed node_modules packages",
	Long: `Removes files from installed node_modules packages based on glob patterns.
Useful for pruning README/LICENSE/docs files to reduce dependency size.`,
	Example: `rev-dep node-modules prune-docs --defaults
rev-dep node-modules prune-docs --patterns "*.md,README.md,docs/**"
rev-dep node-modules prune-docs --defaults --patterns "*.txt"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := node.NodeModulesPruneDocsCmd(
			pathutil.ResolveAbsoluteCwd(nodeModulesCwd),
			nodeModulesPrunePatterns,
			nodeModulesPruneDefaults,
		)
		if err != nil {
			return err
		}

		fmt.Print(result)
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

func listCwdFilesCmdFn(cwd string, include, exclude []string, listFilesCount bool) error {
	files := fs.GetFiles(cwd, []string{}, fs.FindAndProcessGitIgnoreFilesUpToRepoRoot(cwd), nil)

	includeGlobs := globutil.CreateGlobMatchers(include, cwd)
	excludeGlobs := globutil.CreateGlobMatchers(exclude, cwd)
	count := 0

	for _, filePath := range files {
		if len(includeGlobs) == 0 || globutil.MatchesAnyGlobMatcher(filePath, includeGlobs, false) {
			isExcluded := globutil.MatchesAnyGlobMatcher(filePath, excludeGlobs, false)

			if !isExcluded {
				if listFilesCount {
					count++
				} else {
					// filePath is internal-normalized; convert to OS-native and compute relative path for printing
					filePathOs := pathutil.DenormalizePathForOS(filePath)
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
}

var listCwdFilesCmd = &cobra.Command{
	Use:   "list-cwd-files",
	Short: "List all files in the current working directory",
	Long: `Recursively lists all files in the specified directory,
with options to filter results.`,
	Example: "rev-dep list-cwd-files --include='*.ts' --exclude='*.test.ts'",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listCwdFilesCmdFn(
			pathutil.ResolveAbsoluteCwd(listFilesCwd),
			listFilesInclude,
			listFilesExclude,
			listFilesCount,
		)
	},
}

// ---------------- files ----------------
var (
	filesCwd        string
	filesEntryPoint string
	filesIgnoreType bool
	filesCount      bool
)

func filesCmdFn(cwd, entryPoint string, ignoreType, filesCount bool, packageJsonPath, tsconfigJsonPath string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue) error {
	absolutePathToEntryPoint := pathutil.JoinWithCwd(cwd, entryPoint)
	excludeFiles := []string{}

	minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, ignoreType, excludeFiles, nil, []string{absolutePathToEntryPoint}, packageJsonPath, tsconfigJsonPath, conditionNames, followMonorepoPackages, nil)

	depsGraph := graph.BuildDepsGraphForMultiple(minimalTree, []string{absolutePathToEntryPoint}, nil, false, false)

	if filesCount {
		fmt.Println(len(depsGraph.Vertices))
	} else {
		filePaths := make([]string, 0, len(depsGraph.Vertices))
		for _, node := range depsGraph.Vertices {
			// node.Path is internal-normalized; convert to OS path before computing relative path
			nodePathOs := pathutil.DenormalizePathForOS(node.Path)
			relative, _ := filepath.Rel(cwd, nodePathOs)
			filePaths = append(filePaths, relative)
		}
		slices.Sort(filePaths)
		for _, filePath := range filePaths {
			fmt.Println(filePath)
		}
	}
	return nil
}

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "List all files in the dependency tree of an entry point",
	Long: `Recursively finds and lists all files that are required
by the specified entry point.`,
	Example: "rev-dep files --entry-point src/index.ts",
	RunE: func(cmd *cobra.Command, args []string) error {
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		return filesCmdFn(
			pathutil.ResolveAbsoluteCwd(filesCwd),
			filesEntryPoint,
			filesIgnoreType,
			filesCount,
			packageJsonPath,
			tsconfigJsonPath,
			conditionNames,
			followValue,
		)
	},
}

var (
	locCwd string
)

var (
	unresolvedCwd                   string
	unresolvedIgnore                map[string]string
	unresolvedIgnoreFiles           []string
	unresolvedIgnoreImports         []string
	unresolvedCustomAssetExtensions []string
)

// ---------------- imported-by ----------------
var (
	importedByCwd         string
	importedByFile        string
	importedByCount       bool
	importedByListImports bool
)

func linesOfCodeCmdFn(cwd string) error {
	files := fs.GetFiles(cwd, []string{}, fs.FindAndProcessGitIgnoreFilesUpToRepoRoot(cwd), nil)
	ch := make(chan [3]int) // [lines, linesWithoutComments, linesWithoutTemplates]
	var wg sync.WaitGroup

	for _, filePath := range files {
		wg.Add(1)
		go func(filePath string, ch chan [3]int, wg *sync.WaitGroup) {
			// filePath is internal-normalized; convert back to OS-native for IO
			fileContent, err := os.ReadFile(pathutil.DenormalizePathForOS(filePath))
			if err == nil {

				lines := bytes.Count(
					bytes.ReplaceAll(fileContent, []byte{'\n', '\n'}, []byte{'\n'}),
					[]byte{'\n'},
				)

				// Count lines without comments
				linesWithoutComments := bytes.Count(
					bytes.ReplaceAll(source.RemoveCommentsFromCode(fileContent), []byte{'\n', '\n'}, []byte{'\n'}),
					[]byte{'\n'},
				)

				// Count lines without template literals (and comments)
				linesWithoutTemplates := bytes.Count(
					bytes.ReplaceAll(source.RemoveTaggedTemplateLiterals(fileContent), []byte{'\n', '\n'}, []byte{'\n'}),
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
}

func importedByCmdFn(cwd, filePath string, count, listImports bool, packageJsonPath, tsconfigJsonPath string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue) error {
	excludeFiles := []string{}

	minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, false, excludeFiles, nil, []string{}, packageJsonPath, tsconfigJsonPath, conditionNames, followMonorepoPackages, nil)

	absolutePathToFilePath := pathutil.NormalizePathForInternal(pathutil.JoinWithCwd(cwd, filePath))

	// Check if the target file exists in the dependency tree
	if _, found := minimalTree[absolutePathToFilePath]; !found {
		fmt.Printf("Error: Target file '%s' ('%s') not found in dependency tree.\n", filePath, absolutePathToFilePath)
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

	// Find all files that import the target file
	type ImportInfo struct {
		FilePath string
		Request  string
	}

	var importingFiles []string
	var importDetails []ImportInfo

	for filePath, dependencies := range minimalTree {
		for _, dependency := range dependencies {
			if dependency.ID != "" && dependency.ID == absolutePathToFilePath {
				// Convert to relative path for output
				relativePath := strings.TrimPrefix(filePath, cwd)
				if relativePath != filePath { // Only trim if cwd was actually found
					relativePath = strings.TrimPrefix(relativePath, "/")
				}
				importingFiles = append(importingFiles, relativePath)

				if listImports {
					importDetails = append(importDetails, ImportInfo{
						FilePath: relativePath,
						Request:  dependency.Request,
					})
				}
			}
		}
	}

	// Sort the results
	slices.Sort(importingFiles)
	slices.SortFunc(importDetails, func(a, b ImportInfo) int {
		if a.FilePath < b.FilePath {
			return -1
		} else if a.FilePath > b.FilePath {
			return 1
		}
		return 0
	})

	if count {
		fmt.Println(len(importingFiles))
		return nil
	}

	if listImports {
		// Group by file and show import details
		currentFile := ""
		for _, detail := range importDetails {
			if detail.FilePath != currentFile {
				if currentFile != "" {
					fmt.Println()
				}
				fmt.Printf("%s:\n", detail.FilePath)
				currentFile = detail.FilePath
			}
			fmt.Printf("  %s\n", detail.Request)
		}
	} else {
		// Just list the files
		for _, filePath := range importingFiles {
			fmt.Println(filePath)
		}
	}

	return nil
}

var linesOfCodeCmd = &cobra.Command{
	Use:     "lines-of-code",
	Short:   "Count actual lines of code in the project excluding comments and blank lines",
	Example: "rev-dep lines-of-code",
	RunE: func(cmd *cobra.Command, args []string) error {
		return linesOfCodeCmdFn(pathutil.ResolveAbsoluteCwd(locCwd))
	},
}

var importedByCmd = &cobra.Command{
	Use:   "imported-by",
	Short: "List all files that directly import the specified file",
	Long: `Finds and lists all files in the project that directly import the specified file.
This is useful for understanding the impact of changes to a particular file.`,
	Example: "rev-dep imported-by --file src/utils/helpers.ts",
	RunE: func(cmd *cobra.Command, args []string) error {
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}
		return importedByCmdFn(
			pathutil.ResolveAbsoluteCwd(importedByCwd),
			importedByFile,
			importedByCount,
			importedByListImports,
			packageJsonPath,
			tsconfigJsonPath,
			conditionNames,
			followValue,
		)
	},
}

// ---------------- unresolved ----------------
var unresolvedCmd = &cobra.Command{
	Use:   "unresolved",
	Short: "List unresolved imports in the project",
	Long:  `Detect and list imports that could not be resolved during imports resolution. Groups imports by file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		followValue, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			return err
		}

		opts := &config.UnresolvedImportsOptions{
			Enabled:       true,
			Ignore:        stringMapToFileValueIgnoreMap(unresolvedIgnore),
			IgnoreFiles:   unresolvedIgnoreFiles,
			IgnoreImports: unresolvedIgnoreImports,
		}
		if err := config.ValidateUnresolvedImportsOptions(opts, "unresolved"); err != nil {
			return err
		}
		if err := resolve.ValidateCustomAssetExtensions(unresolvedCustomAssetExtensions, "unresolved.customAssetExtensions"); err != nil {
			return err
		}

		return unresolvedCmdRun(pathutil.ResolveAbsoluteCwd(unresolvedCwd), packageJsonPath, tsconfigJsonPath, conditionNames, followValue, opts, unresolvedCustomAssetExtensions)
	},
}

func stringMapToFileValueIgnoreMap(input map[string]string) globutil.FileValueIgnoreMap {
	if len(input) == 0 {
		return nil
	}
	out := make(globutil.FileValueIgnoreMap, len(input))
	for filePath, value := range input {
		out[filePath] = []string{value}
	}
	return out
}

// unresolvedCmdRun is the functional core for the `unresolved` command. It returns an error on failure.
func unresolvedCmdRun(cwd, packageJson, tsconfigJson string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue, options *config.UnresolvedImportsOptions, customAssetExtensions []string) error {
	out, err := getUnresolvedOutput(cwd, packageJson, tsconfigJson, conditionNames, followMonorepoPackages, options, customAssetExtensions)
	if err != nil {
		return err
	}
	if out != "" {
		fmt.Print(out)
	}
	return nil
}

// getUnresolvedOutput returns formatted unresolved imports grouped by file as a string.
func getUnresolvedOutput(cwd, packageJson, tsconfigJson string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue, options *config.UnresolvedImportsOptions, customAssetExtensions []string) (string, error) {
	if options == nil {
		options = &config.UnresolvedImportsOptions{}
	}
	minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, false, []string{}, nil, []string{}, packageJson, tsconfigJson, conditionNames, followMonorepoPackages, customAssetExtensions)

	unresolved := checks.DetectUnresolvedImports(minimalTree, map[string]bool{})
	filterOpts := &checks.UnresolvedFilterOptions{
		Ignore:        options.Ignore,
		IgnoreFiles:   options.IgnoreFiles,
		IgnoreImports: options.IgnoreImports,
	}
	unresolved = checks.FilterUnresolvedImports(unresolved, filterOpts, cwd)

	unresolvedByFile := make(map[string][]string)
	for _, u := range unresolved {
		unresolvedByFile[u.FilePath] = append(unresolvedByFile[u.FilePath], u.Request)
	}

	// Build output
	var filePaths []string
	for fp := range unresolvedByFile {
		filePaths = append(filePaths, fp)
	}
	slices.Sort(filePaths)

	var b strings.Builder
	for _, fp := range filePaths {
		// Convert to relative path
		rel, err := filepath.Rel(cwd, pathutil.DenormalizePathForOS(fp))
		if err != nil {
			b.WriteString(fp)
		} else {
			b.WriteString(filepath.ToSlash(rel))
		}
		b.WriteString("\n")
		for _, req := range unresolvedByFile[fp] {
			b.WriteString("  - ")
			b.WriteString(req)
			b.WriteString("\n")
		}
	}

	return b.String(), nil
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
		command.Flags().BoolVar(&nodeModulesGroupByModuleFilesCount, "group-by-module-files-count", false,
			"Organize output by npm package name and show count of files using it")
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
	resolveCmd.Flags().StringVar(&resolveModule, "module", "",
		"Target node module name to check for dependencies")
	resolveCmd.Flags().StringSliceVar(&resolveGraphExclude, "graph-exclude", []string{},
		"Glob patterns to exclude files from dependency analysis")
	resolveCmd.Flags().StringSliceVarP(&resolveEntryPoints, "entry-points", "p", []string{},
		"Entry point file(s) or glob pattern(s) to start analysis from (default: auto-detected)")
	resolveCmd.Flags().BoolVarP(&resolveIgnoreType, "ignore-type-imports", "t", false,
		"Exclude type imports from the analysis")
	resolveCmd.Flags().BoolVarP(&resolveAll, "all", "a", false,
		"Show all possible resolution paths, not just the first one")
	resolveCmd.Flags().BoolVar(&resolveCompactSummary, "compact-summary", false,
		"Display a compact summary of found paths")
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
	circularCmd.Flags().StringVar(&circularAlgorithm, "algorithm", "DFS",
		"Cycle detection algorithm: DFS (default) or SCC")

	// node-modules flags
	addNodeModulesFlags(nodeModulesUsedCmd, false)
	nodeModulesUsedCmd.Flags().BoolVar(&nodeModulesGroupByEntryPoint, "group-by-entry-point", false,
		"Organize output by entry point file path")
	nodeModulesUsedCmd.Flags().BoolVar(&nodeModulesGroupByEntryPointModCount, "group-by-entry-point-modules-count", false,
		"Organize output by entry point and show count of unique modules")
	nodeModulesUsedCmd.Flags().BoolVar(&nodeModulesGroupByModuleShowEntries, "group-by-module-show-entry-points", false,
		"Organize output by npm package name and list entry points using it")
	nodeModulesUsedCmd.Flags().BoolVar(&nodeModulesGroupByModuleEntriesCount, "group-by-module-entry-points-count", false,
		"Organize output by npm package name and show count of entry points using it")
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
	nodeModulesPruneDocsCmd.Flags().StringVarP(&nodeModulesCwd, "cwd", "c", currentDir, "Working directory for the command")
	nodeModulesPruneDocsCmd.Flags().StringSliceVarP(&nodeModulesPrunePatterns, "patterns", "p", []string{},
		"Glob patterns (relative to each package root) of files to remove, e.g. \"*.md,README.md,docs/**\"")
	nodeModulesPruneDocsCmd.Flags().StringSliceVar(&nodeModulesPrunePatterns, "pattern", []string{},
		"Alias for --patterns")
	nodeModulesPruneDocsCmd.Flags().BoolVar(&nodeModulesPruneDefaults, "defaults", false,
		"Use default prune patterns: LICENSE, README.md, docs/**")

	// node modules commands
	nodeModulesCmd.AddCommand(nodeModulesUsedCmd, nodeModulesUnusedCmd, nodeModulesMissingCmd, nodeModulesInstalledCmd, nodeModulesInstalledDuplicatesCmd, nodeModulesAnalyzeSize, nodeModuleDirsSize, nodeModulesPruneDocsCmd)

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

	// imported-by flags
	addSharedFlags(importedByCmd)
	importedByCmd.Flags().StringVarP(&importedByCwd, "cwd", "c", currentDir,
		"Working directory for the command")
	importedByCmd.Flags().StringVarP(&importedByFile, "file", "f", "",
		"Target file to find importers for (required)")
	importedByCmd.Flags().BoolVarP(&importedByCount, "count", "n", false,
		"Only display the count of importing files")
	importedByCmd.Flags().BoolVar(&importedByListImports, "list-imports", false,
		"List the import identifiers used by each file")
	importedByCmd.MarkFlagRequired("file")

	// unresolved flags
	addSharedFlags(unresolvedCmd)
	unresolvedCmd.Flags().StringVarP(&unresolvedCwd, "cwd", "c", currentDir,
		"Working directory for the command")
	unresolvedCmd.Flags().StringToStringVar(&unresolvedIgnore, "ignore", map[string]string{},
		"Map of file path (relative to cwd) to exact import request to ignore (e.g. --ignore src/index.ts=some-module)")
	unresolvedCmd.Flags().StringSliceVar(&unresolvedIgnoreFiles, "ignore-files", []string{},
		"File path glob patterns to ignore in unresolved output")
	unresolvedCmd.Flags().StringSliceVar(&unresolvedIgnoreImports, "ignore-imports", []string{},
		"Import requests to ignore globally in unresolved output")
	unresolvedCmd.Flags().StringSliceVar(&unresolvedCustomAssetExtensions, "custom-asset-extensions", []string{},
		"Additional asset extensions treated as resolvable (e.g. glb,mp3)")

	// add commands
	rootCmd.AddCommand(resolveCmd, entryPointsCmd, circularCmd, nodeModulesCmd, listCwdFilesCmd, filesCmd, linesOfCodeCmd, importedByCmd, unresolvedCmd, docsCmd, configCmd)
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		diag.SetVerbose(verboseFlag)
	}
	installHelpOutputSanitizer(rootCmd)
}

func Execute() error {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		return err
	}
	return nil
}
