package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/Masterminds/semver/v3"
)

func CheckDirForInstalledModules(dirName string, cwd string, packageInfoChan chan PackageInfo, nodeModulesDirChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	entries, err := os.ReadDir(dirName)
	dirNamePathParts := strings.Split(dirName, osSeparator)
	isNodeModule := (len(dirNamePathParts) > 1 && dirNamePathParts[len(dirNamePathParts)-2] == "node_modules") || (len(dirNamePathParts) > 2 && dirNamePathParts[len(dirNamePathParts)-3] == "node_modules")

	if err != nil {
		return
	}

	for _, entry := range entries {
		entryName := entry.Name()
		entryFilePath := filepath.Join(dirName, entryName)

		if entry.IsDir() {
			if strings.Count(entryFilePath, "node_modules") == 1 && strings.HasSuffix(entryFilePath, "node_modules") {
				nodeModulesDirChan <- entryFilePath
			}
			wg.Add(1)
			go CheckDirForInstalledModules(entryFilePath, cwd, packageInfoChan, nodeModulesDirChan, wg)
		} else if entryName == "package.json" && isNodeModule {
			wg.Add(1)
			go ParsePackageJson(entryFilePath, cwd, packageInfoChan, wg)
		}
	}
}

func CheckDirForNodeModuleDirs(dirName string, cwd string, nodeModulesDirChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	entries, err := os.ReadDir(dirName)

	if err != nil {
		return
	}

	for _, entry := range entries {
		entryName := entry.Name()
		entryFilePath := filepath.Join(dirName, entryName)

		if entry.IsDir() {
			if strings.Count(entryFilePath, "node_modules") == 1 && strings.HasSuffix(entryFilePath, "node_modules") {
				nodeModulesDirChan <- entryFilePath
			} else {
				wg.Add(1)
				go CheckDirForNodeModuleDirs(entryFilePath, cwd, nodeModulesDirChan, wg)
			}
		}
	}
}

func GetInstalledModules(cwd string, modulesToInclude []string, modulesToExclude []string) (map[string][]PackageInfo, []string) {
	shouldIncludeModule := createShouldModuleByIncluded(modulesToInclude, modulesToExclude)

	packageInfoChan := make(chan PackageInfo)
	nodeModulesDirChan := make(chan string)
	var wg sync.WaitGroup
	var wg2 sync.WaitGroup

	wg.Add(1)

	go CheckDirForInstalledModules(cwd, cwd, packageInfoChan, nodeModulesDirChan, &wg)

	go func() {
		wg.Wait()
		close(packageInfoChan)
		close(nodeModulesDirChan)
	}()

	modules := map[string][]PackageInfo{}
	nodeModuleDirs := []string{}

	wg2.Add(1)
	go func() {
		for nodeModulesDir := range nodeModulesDirChan {
			// This should not require lock as this is only place that writes to this array and loop is sequential
			nodeModuleDirs = append(nodeModuleDirs, nodeModulesDir)
		}
		wg2.Done()
	}()

	for info := range packageInfoChan {
		if shouldIncludeModule(info.Name) {
			_, has := (modules)[info.Name]
			if has {
				modules[info.Name] = append(modules[info.Name], info)
			} else {
				(modules)[info.Name] = []PackageInfo{info}
			}
		}
	}

	wg2.Wait()

	SortPathsToNodeModulesByNestingLevel(nodeModuleDirs)

	return modules, nodeModuleDirs
}

func GetNodeModuleDirs(cwd string) []string {

	nodeModulesDirChan := make(chan string)
	var wg sync.WaitGroup

	wg.Add(1)

	go CheckDirForNodeModuleDirs(cwd, cwd, nodeModulesDirChan, &wg)

	go func() {
		wg.Wait()
		close(nodeModulesDirChan)
	}()

	nodeModuleDirs := []string{}

	for nodeModulesDir := range nodeModulesDirChan {
		nodeModuleDirs = append(nodeModuleDirs, nodeModulesDir)
	}

	SortPathsToNodeModulesByNestingLevel(nodeModuleDirs)

	return nodeModuleDirs
}

func SortPathsToNodeModulesByNestingLevel(paths []string) {
	slices.SortStableFunc(paths, func(pathA string, pathB string) int {
		pathACount := strings.Count(pathA, "node_modules")
		pathBCount := strings.Count(pathB, "node_modules")

		// Higher level packages first
		if pathACount < pathBCount {
			return -1
		}
		if pathACount > pathBCount {
			return 1
		}

		// Shorter name package first
		if len(pathA) < len(pathB) {
			return -1
		}
		if len(pathA) > len(pathB) {
			return 1
		}

		// Alphabetical order asc
		if pathA < pathB {
			return -1
		}
		if pathA > pathB {
			return 1
		}

		return 0
	})
}

func GetDuplicatedModulesCmd(cwd string, shouldOptimize bool, verbose bool, sizeStats bool, isolate bool) string {
	modules, nodeModuleDirs := GetInstalledModules(cwd, []string{}, []string{})

	duplicatedModulesByVersion := make(map[string]map[string][]string)

	for name, installations := range modules {
		moduleInfo := map[string][]string{}
		hasDuplicates := false
		for _, ins := range installations {
			_, hasVersionArr := moduleInfo[ins.Version]
			if hasVersionArr {
				moduleInfo[ins.Version] = append(moduleInfo[ins.Version], ins.FilePath)
				hasDuplicates = true
			} else {
				moduleInfo[ins.Version] = []string{ins.FilePath}
			}
		}
		if hasDuplicates {
			moduleInfoWithDuplicates := map[string][]string{}
			for version, filePaths := range moduleInfo {
				if len(filePaths) > 1 {
					moduleInfoWithDuplicates[version] = filePaths
				}
			}
			(duplicatedModulesByVersion)[name] = moduleInfoWithDuplicates
		}
	}

	nodeModuleDirsWithoutCwd := []string{}

	for _, nodeModuleDir := range nodeModuleDirs {
		nodeModuleDirsWithoutCwd = append(nodeModuleDirsWithoutCwd, strings.Replace(nodeModuleDir, cwd, "", 1))
	}

	count := 0
	errorC := 0
	skipped := []string{}
	installedSizeBefore := map[string]int64{}

	if shouldOptimize {
		if sizeStats {
			for _, modulePath := range nodeModuleDirs {
				size, _ := dirSizeWithoutSymlinkSize(modulePath)
				installedSizeBefore[modulePath] = size
			}
		}
		// It's only safe to create symlinks to leaf packages
		for _, data := range duplicatedModulesByVersion {
			for version, paths := range data {
				SortPathsToNodeModulesByNestingLevel(paths)
				pathsGroups := groupNodeModulePathsByNodeModuleDirs(paths, nodeModuleDirsWithoutCwd, isolate)

				for _, paths := range pathsGroups {
					stored := paths[0]
					rest := paths[1:]

					storedDir := strings.Replace(stored, osSeparator+"package.json", "", 1)
					storedDirAbsPath := filepath.Join(cwd, storedDir)

					nestedNodeModules, err := os.Stat(filepath.Join(storedDirAbsPath, "node_modules"))

					if err == nil && nestedNodeModules.IsDir() {
						skipped = append(skipped, storedDirAbsPath)
						continue
					}

					for _, pathToSymlink := range rest {
						pathToSymlinkDir := strings.Replace(pathToSymlink, osSeparator+"package.json", "", 1)
						symlinkDirAbsPath := filepath.Join(cwd, pathToSymlinkDir)

						nestedNodeModules, err = os.Stat(filepath.Join(symlinkDirAbsPath, "node_modules"))
						if err == nil && nestedNodeModules.IsDir() {
							skipped = append(skipped, symlinkDirAbsPath)
							continue
						}

						os.RemoveAll(symlinkDirAbsPath)
						symlinkErr := os.Symlink(storedDirAbsPath, symlinkDirAbsPath)

						if verbose {
							fmt.Println("Symlink", version, storedDirAbsPath, "in", symlinkDirAbsPath)
						}

						if symlinkErr != nil {
							if verbose {
								fmt.Println(symlinkErr)
							}
							errorC++
						}
						count++
					}
				}
			}
		}
	}

	sortedDuplicatedModuleNames := make([]string, len(duplicatedModulesByVersion))

	for name := range duplicatedModulesByVersion {
		sortedDuplicatedModuleNames = append(sortedDuplicatedModuleNames, name)
	}

	slices.Sort(sortedDuplicatedModuleNames)

	result := ""

	for _, name := range sortedDuplicatedModuleNames {
		data := duplicatedModulesByVersion[name]
		result += fmt.Sprintln(name)

		// Get and sort versions
		versions := make([]string, 0, len(data))
		for version := range data {
			versions = append(versions, version)
		}
		slices.Sort(versions)

		for _, version := range versions {
			paths := data[version]
			// Sort paths alphabetically
			slices.Sort(paths)
			result += fmt.Sprintf("   %s:\n", version)
			result += fmt.Sprintf("      %s\n", strings.Join(paths, "\n      "))
		}
	}

	if shouldOptimize {
		result += fmt.Sprintln("\nSymlinks", "Created:", (count), "Skipped:", len(skipped), "Errored:", errorC, "\n", "")
	}

	if shouldOptimize && sizeStats {

		var builder strings.Builder
		var sumBefore int64 = 0
		var sumAfter int64 = 0

		writer := tabwriter.NewWriter(&builder, 0, 0, 3, ' ', 0)

		fmt.Fprintln(writer, "DIR NAME\tBEFORE(MB)\tAFTER(MB)\tREDUCTION(MB)")
		fmt.Fprintln(writer, "\t\t\t\t")

		for _, modulePath := range nodeModuleDirs {
			size, _ := dirSizeWithoutSymlinkSize(modulePath)
			sumBefore += installedSizeBefore[modulePath]
			sumAfter += size
			// fmt.Fprintln(writer, modulePath, "\t", installedSizeBefore[modulePath], "After:", size, "Reduced:", installedSizeBefore[modulePath]-size)
			fmt.Fprintf(writer, "%s\t%.2f\t%.2f\t%.2f\n",
				strings.Replace(modulePath, cwd, "", 1), bytesToMB(installedSizeBefore[modulePath]), bytesToMB(size), bytesToMB(installedSizeBefore[modulePath]-size),
			)
		}

		fmt.Fprintln(writer, "\t\t\t\t")
		fmt.Fprintf(writer, "%s\t%.2f\t%.2f\t%.2f\n", "TOTAL", bytesToMB(sumBefore), bytesToMB(sumAfter), bytesToMB((sumBefore)-(sumAfter)))

		writer.Flush()

		result += builder.String()

	}

	return result
}

func groupNodeModulePathsByNodeModuleDirs(paths []string, nodeModuleDirs []string, shouldGroup bool) [][]string {
	if !shouldGroup {
		return [][]string{paths}
	} else {
		groups := map[string][]string{}

		for _, nodeModuleDir := range nodeModuleDirs {
			groups[nodeModuleDir] = []string{}
		}

		for _, path := range paths {
			for _, nodeModuleDir := range nodeModuleDirs {
				if strings.HasPrefix(path, nodeModuleDir) {
					groups[nodeModuleDir] = append(groups[nodeModuleDir], path)
					break
				}
			}
		}

		result := [][]string{}
		for _, groupPaths := range groups {
			if len(groupPaths) > 0 {
				result = append(result, groupPaths)
			}
		}

		return result
	}
}

func GetInstalledModulesCmd(cwd string, modulesToInclude []string, modulesToExclude []string) string {
	modules, _ := GetInstalledModules(cwd, modulesToInclude, modulesToExclude)

	sortedModules := GetSortedMap(modules)
	result := ""
	count := 0

	for _, kv := range sortedModules {
		modulesInfo := kv.v
		slices.SortFunc(modulesInfo, func(a PackageInfo, b PackageInfo) int {
			if a.Version < b.Version {
				return 1
			}

			if a.Version > b.Version {
				return -1
			}

			if a.FilePath < b.FilePath {
				return -1
			}
			if a.FilePath > b.FilePath {
				return 1
			}

			return 0
		})

		result += "\n"
		for _, moduleInfo := range modulesInfo {
			count++
			result += fmt.Sprintf("%s@%s %s\n", moduleInfo.Name, moduleInfo.Version, moduleInfo.FilePath)
		}
		result += "\n"
	}

	result += fmt.Sprintln("Total count: ", count)

	return result
}

func ModulesDiskSizeCmd(cwd string) string {
	nodeModuleDirs := GetNodeModuleDirs(cwd)

	var builder strings.Builder
	var sum int64 = 0

	writer := tabwriter.NewWriter(&builder, 0, 0, 3, ' ', 0)

	fmt.Fprintln(writer, "DIR NAME\tSIZE(MB)")
	fmt.Fprintln(writer, "\t\t\t\t")

	for _, modulePath := range nodeModuleDirs {
		size, _ := dirSizeWithoutSymlinkSize(modulePath)
		sum += size
		fmt.Fprintf(writer, "%s\t%.2f\n",
			strings.Replace(modulePath, cwd, "", 1), bytesToMB(size),
		)
	}

	fmt.Fprintln(writer, "\t\t\t\t")
	fmt.Fprintf(writer, "%s\t%.2f\n", "TOTAL", bytesToMB(sum))

	writer.Flush()

	return builder.String()
}

type ModuleReport struct {
	Name    string
	Version string
	Path    string

	OwnSize           int64
	ExclusiveDepsSize int64
	SharedDepsSize    int64
	OwnPlusExclusive  int64
	TotalSize         int64

	RemovedPaths []string
}

// AnalyzeNodeModules performs dependency-aware size breakdown with semver-aware exclusivity.
func AnalyzeNodeModules(cwd string, modules map[string][]PackageInfo) ([]ModuleReport, error) {
	realPath := func(p string) string {
		if rp, err := filepath.EvalSymlinks(p); err == nil {
			return rp
		}
		return p
	}

	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		cwd = wd
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	absCwd = realPath(absCwd)

	// ---------- STEP 1: Build installed module index ----------
	type node struct {
		Key     string
		Name    string
		Version string
		Dir     string
		Size    int64
		Deps    []DeclaredDep
	}

	installedByPkgJSON := make(map[string]*node)
	installedPkgJSONSet := make(map[string]bool)

	for _, arr := range modules {
		for _, pi := range arr {
			if pi.FilePath == "" {
				continue
			}
			absPath, err := filepath.Abs(pi.FilePath)
			if err != nil {
				continue
			}
			absPath = realPath(absPath)
			dir := realPath(filepath.Dir(absPath))

			installedByPkgJSON[absPath] = &node{
				Key:     absPath,
				Name:    pi.Name,
				Version: pi.Version,
				Dir:     dir,
			}
			installedPkgJSONSet[absPath] = true
		}
	}

	// ---------- STEP 1.5: Size calculation (follow symlinks) ----------
	dirSizeWithSymlinksSize := func(root string) (int64, error) {
		var size int64
		visited := make(map[string]bool)

		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			real := realPath(path)
			if visited[real] {
				return nil
			}
			visited[real] = true

			info, err := os.Stat(real)
			if err != nil {
				return nil
			}
			if info.Mode().IsRegular() {
				size += info.Size()
			}
			return nil
		})
		return size, err
	}

	// Fill in sizes and declared deps
	for pkgJSONPath, n := range installedByPkgJSON {
		n.Size, _ = dirSizeWithSymlinksSize(n.Dir)
		deps, _ := readDeclaredDeps(pkgJSONPath)
		n.Deps = deps
	}

	// ---------- STEP 2: Dependency resolution with semver ----------
	semverMatches := func(rangeStr, installed string) bool {
		if rangeStr == "" || rangeStr == "*" {
			return true
		}
		r, err := semver.NewConstraint(rangeStr)
		if err != nil {
			return false
		}
		v, err := semver.NewVersion(installed)
		if err != nil {
			return false
		}
		return r.Check(v)
	}

	resolveDependency := func(consumerDir, depName, depRange string) (string, bool) {
		cur := consumerDir
		for {
			candidate := filepath.Join(cur, "node_modules", depName, "package.json")
			absCand, _ := filepath.Abs(candidate)
			absCand = realPath(absCand)

			if node, ok := installedByPkgJSON[absCand]; ok {
				if semverMatches(depRange, node.Version) {
					return absCand, true
				}
			}

			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}

		rootCandidate := filepath.Join(absCwd, "node_modules", depName, "package.json")
		rootCandidate = realPath(rootCandidate)
		if node, ok := installedByPkgJSON[rootCandidate]; ok {
			if semverMatches(depRange, node.Version) {
				return rootCandidate, true
			}
		}
		return "", false
	}

	// ---------- STEP 3: Build dependency graph ----------
	graph := make(map[string][]string)
	incoming := make(map[string]int)
	for k := range installedByPkgJSON {
		incoming[k] = 0
	}

	for pkgPath, n := range installedByPkgJSON {
		for _, dep := range n.Deps {
			if dep.Name == "" {
				continue
			}
			if resolved, ok := resolveDependency(n.Dir, dep.Name, dep.Version); ok {
				graph[pkgPath] = append(graph[pkgPath], resolved)
				incoming[resolved]++
			}
		}
	}

	// ---------- STEP 4: Identify monorepo package roots ----------
	rootPkgFiles, _ := findMonorepoPackageJSONs(absCwd)
	rootToInstalled := make(map[string][]string)
	rootsReferencingCount := make(map[string]int)

	for _, rootPJ := range rootPkgFiles {
		deps, _ := readDeclaredDeps(rootPJ)
		rootDir := filepath.Dir(rootPJ)
		for _, dep := range deps {
			if resolved, ok := resolveDependency(rootDir, dep.Name, dep.Version); ok {
				rootToInstalled[rootPJ] = append(rootToInstalled[rootPJ], resolved)
				rootsReferencingCount[resolved]++
				incoming[resolved]++
			}
		}
	}

	// ---------- STEP 5: Compute exclusive/shared deps ----------
	var reachableFrom = func(start string) (map[string]bool, int64) {
		visited := make(map[string]bool)
		var total int64
		stack := []string{start}

		for len(stack) > 0 {
			nk := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[nk] {
				continue
			}
			visited[nk] = true
			if inst, ok := installedByPkgJSON[nk]; ok {
				total += inst.Size
			}
			for _, child := range graph[nk] {
				if !visited[child] {
					stack = append(stack, child)
				}
			}
		}
		return visited, total
	}

	results := []ModuleReport{}

	for candidate := range rootsReferencingCount {
		n := installedByPkgJSON[candidate]
		if n == nil {
			continue
		}

		own := n.Size
		visitedAll, totalSize := reachableFrom(candidate)

		tmpIncoming := make(map[string]int)
		for k, v := range incoming {
			tmpIncoming[k] = v
		}

		for _, insts := range rootToInstalled {
			for _, inst := range insts {
				if inst == candidate {
					tmpIncoming[candidate]--
				}
			}
		}

		queue := []string{}
		removedSet := make(map[string]bool)

		for k := range installedByPkgJSON {
			if tmpIncoming[k] == 0 && visitedAll[k] {
				queue = append(queue, k)
				removedSet[k] = true
			}
		}

		for len(queue) > 0 {
			nk := queue[0]
			queue = queue[1:]
			for _, child := range graph[nk] {
				tmpIncoming[child]--
				if tmpIncoming[child] == 0 && visitedAll[child] && !removedSet[child] {
					removedSet[child] = true
					queue = append(queue, child)
				}
			}
		}

		var exclusiveSize int64
		var removedPaths []string

		for p := range removedSet {
			if p == candidate {
				continue
			}
			if inst := installedByPkgJSON[p]; inst != nil {
				exclusiveSize += inst.Size
				removedPaths = append(removedPaths, inst.Dir)
			}
		}

		shared := totalSize - own - exclusiveSize
		ownPlusExclusive := own + exclusiveSize

		sort.Strings(removedPaths)
		results = append(results, ModuleReport{
			Name:              n.Name,
			Version:           n.Version,
			Path:              n.Dir,
			OwnSize:           own,
			ExclusiveDepsSize: exclusiveSize,
			SharedDepsSize:    shared,
			OwnPlusExclusive:  ownPlusExclusive,
			TotalSize:         totalSize,
			RemovedPaths:      removedPaths,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].OwnPlusExclusive != results[j].OwnPlusExclusive {
			return results[i].OwnPlusExclusive > results[j].OwnPlusExclusive
		}
		// If OwnPlusExclusive values are equal, sort by package name
		if results[i].Name != results[j].Name {
			return results[i].Name < results[j].Name
		}
		// If package names are equal, sort by version
		if results[i].Version != results[j].Version {
			return results[i].Version > results[j].Version
		}
		// If all previous criteria are equal, sort by total size
		return results[i].TotalSize > results[j].TotalSize
	})

	return results, nil
}

// ---------- Helper functions ----------

type DeclaredDep struct {
	Name    string
	Version string
}

func readDeclaredDeps(pkgJSONPath string) ([]DeclaredDep, error) {
	f, err := os.Open(pkgJSONPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var raw map[string]interface{}
	dec := json.NewDecoder(f)
	_ = dec.Decode(&raw)
	deps := []DeclaredDep{}
	sections := []string{
		"dependencies",
		"devDependencies",
		"peerDependencies",
		"optionalDependencies",
		"bundledDependencies",
	}
	for _, s := range sections {
		if v, ok := raw[s]; ok {
			switch t := v.(type) {
			case map[string]interface{}:
				for name, ver := range t {
					deps = append(deps, DeclaredDep{Name: name, Version: fmt.Sprint(ver)})
				}
			case []interface{}:
				for _, vv := range t {
					if s, ok := vv.(string); ok {
						deps = append(deps, DeclaredDep{Name: s, Version: "*"})
					}
				}
			}
		}
	}
	return deps, nil
}

func findMonorepoPackageJSONs(root string) ([]string, error) {
	var res []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == "node_modules" {
			return filepath.SkipDir
		}
		if !d.IsDir() && d.Name() == "package.json" {
			res = append(res, p)
		}
		return nil
	})
	return res, err
}

func dirSizeWithoutSymlinkSize(root string) (int64, error) {
	var total int64
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total, nil
}

func PrintAnalysis(reports []ModuleReport) {
	if len(reports) == 0 {
		fmt.Println("No modules found.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "MODULE\tVERSION\tOWN(MB)\tEXCL(MB)\tSHARED(MB)\tOWN+EXCL(MB)\tTOTAL(MB)")
	for _, r := range reports {
		fmt.Fprintf(w, "%s\t%s\t%.2f\t%.2f\t%.2f\t%.2f\t%.2f\n",
			r.Name, r.Version,
			bytesToMB(r.OwnSize),
			bytesToMB(r.ExclusiveDepsSize),
			bytesToMB(r.SharedDepsSize),
			bytesToMB(r.OwnPlusExclusive),
			bytesToMB(r.TotalSize),
		)
	}
	w.Flush()
}

func bytesToMB(b int64) float64 { return float64(b) / (1024 * 1024) }
