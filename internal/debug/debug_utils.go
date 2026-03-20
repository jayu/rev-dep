package debugutil

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func getSortedTreeForPrint(tree MinimalDependencyTree) string {
	type kv struct {
		FilePath string
		Imports  [][]string
	}

	var sortedTree []kv
	for k, v := range tree {
		imports := [][]string{}

		for _, imp := range v {
			imports = append(imports, []string{imp.ID, imp.Request})
		}

		sort.Slice(imports, func(i, j int) bool {
			return imports[i][0] > imports[j][0]
		})

		sortedTree = append(sortedTree, kv{k, imports})
	}

	sort.Slice(sortedTree, func(i, j int) bool {
		return sortedTree[i].FilePath > sortedTree[j].FilePath
	})

	result := ""

	for _, entry := range sortedTree {

		result += entry.FilePath + "\n(\n"

		for _, imports := range entry.Imports {
			result += "  " + imports[0] + "=" + imports[1] + "\n"
		}

		result += ")\n"
	}
	return result
}

func StringifyMinimalDependencyTree(tree MinimalDependencyTree) string {
	var b strings.Builder

	// Collect and sort keys deterministically
	keys := make([]string, 0, len(tree))
	for k := range tree {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(":\n")

		deps := make([]MinimalDependency, len(tree[k]))
		copy(deps, tree[k])

		// Sort dependencies by Request, then ID, then ResolvedType (stringified)
		sort.SliceStable(deps, func(i, j int) bool {
			if deps[i].Request != deps[j].Request {
				return deps[i].Request < deps[j].Request
			}
			idI := deps[i].ID
			idJ := deps[j].ID

			if idI != idJ {
				return idI < idJ
			}
			return ResolvedImportTypeToString(deps[i].ResolvedType) < ResolvedImportTypeToString(deps[j].ResolvedType)
		})

		if len(deps) == 0 {
			b.WriteString("  (no dependencies)\n")
			continue
		}

		for _, d := range deps {
			id := "<nil>"
			if d.ID != "" {
				id = d.ID
			}
			importKind := ImportKindToString(d.ImportKind)

			b.WriteString("  - request: ")
			b.WriteString(d.Request)
			b.WriteString("\n")
			b.WriteString("    id: ")
			b.WriteString(id)
			b.WriteString("\n")
			b.WriteString("    resolvedType: ")
			b.WriteString(ResolvedImportTypeToString(d.ResolvedType))
			b.WriteString("\n")
			b.WriteString("    importKind: ")
			b.WriteString(importKind)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func StringifyFileImportsArr(fileImportsArr []FileImports) []byte {
	type importForJson struct {
		Request      string `json:"request"`
		Kind         string `json:"kind"`
		Path         string `json:"path"`
		ResolvedType string `json:"resolvedType"`
	}

	type fileImportsForJson struct {
		FilePath string          `json:"filePath"`
		Imports  []importForJson `json:"imports"`
	}

	fileImportsArrSortedForJson := make([]fileImportsForJson, 0, len(fileImportsArr))

	for _, fi := range fileImportsArr {
		imports := make([]importForJson, 0, len(fi.Imports))
		for _, imp := range fi.Imports {
			imports = append(imports, importForJson{
				Request:      imp.Request,
				Kind:         ImportKindToString(imp.Kind),
				Path:         imp.PathOrName,
				ResolvedType: ResolvedImportTypeToString(imp.ResolvedType),
			})
		}

		fileImportsArrSortedForJson = append(fileImportsArrSortedForJson, fileImportsForJson{
			FilePath: fi.FilePath,
			Imports:  imports,
		})
	}

	// Sort for deterministic output
	sort.SliceStable(fileImportsArrSortedForJson, func(i, j int) bool {
		return fileImportsArrSortedForJson[i].FilePath < fileImportsArrSortedForJson[j].FilePath
	})

	jsonData, _ := json.MarshalIndent(fileImportsArrSortedForJson, "", "  ")
	return jsonData
}

// StringifyResolverManager returns a deterministic string representation of ResolverManager
// Maps are iterated in sorted key order to ensure deterministic output.
func StringifyResolverManager(rm *ResolverManager) []byte {
	var b strings.Builder
	if rm == nil {
		return []byte("<nil>")
	}

	b.WriteString("ResolverManager:\n")
	followMode := "disabled"
	follow := rm.FollowMonorepoPackages()
	if follow.ShouldFollowAll() {
		followMode = "all"
	} else if follow.IsEnabled() {
		packages := make([]string, 0, len(follow.Packages))
		for packageName := range follow.Packages {
			packages = append(packages, packageName)
		}
		sort.Strings(packages)
		followMode = fmt.Sprintf("selective:[%s]", strings.Join(packages, ","))
	}
	b.WriteString(fmt.Sprintf("  followMonorepoPackages: %s\n", followMode))

	// conditionNames (sorted)
	cond := make([]string, len(rm.ConditionNames()))
	copy(cond, rm.ConditionNames())
	sort.Strings(cond)
	b.WriteString("  conditionNames:\n")
	for _, c := range cond {
		b.WriteString("    - ")
		b.WriteString(c)
		b.WriteString("\n")
	}

	// rootParams
	b.WriteString("  rootParams:\n")
	b.WriteString("    Cwd: ")
	b.WriteString(rm.RootParams().Cwd)
	b.WriteString("\n")
	// SortedFiles (sort for determinism)
	files := make([]string, len(rm.RootParams().SortedFiles))
	copy(files, rm.RootParams().SortedFiles)
	sort.Strings(files)
	b.WriteString("    SortedFiles:\n")
	for _, f := range files {
		b.WriteString("      - ")
		b.WriteString(f)
		b.WriteString("\n")
	}

	// filesAndExtensions
	b.WriteString("  filesAndExtensions:\n")
	if rm.FilesAndExtensions() != nil {
		// dereference map
		m := *rm.FilesAndExtensions()
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString("    ")
			b.WriteString(k)
			b.WriteString(" = ")
			b.WriteString(m[k])
			b.WriteString("\n")
		}
	}

	// monorepoContext
	b.WriteString("  monorepoContext:\n")
	if rm.MonorepoContext() != nil {
		mc := rm.MonorepoContext()
		b.WriteString("    WorkspaceRoot: ")
		b.WriteString(mc.WorkspaceRoot)
		b.WriteString("\n")

		// PackageToPath sorted
		keys := make([]string, 0, len(mc.PackageToPath))
		for k := range mc.PackageToPath {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("    PackageToPath:\n")
		for _, k := range keys {
			b.WriteString("      ")
			b.WriteString(k)
			b.WriteString(" = ")
			b.WriteString(mc.PackageToPath[k])
			b.WriteString("\n")
		}

		// PackageConfigCache keys sorted - print basic info
		cfgKeys := make([]string, 0, len(mc.PackageConfigCache))
		for k := range mc.PackageConfigCache {
			cfgKeys = append(cfgKeys, k)
		}
		sort.Strings(cfgKeys)
		b.WriteString("    PackageConfigCache:\n")
		for _, k := range cfgKeys {
			cfg := mc.PackageConfigCache[k]
			if cfg == nil {
				continue
			}
			b.WriteString("      ")
			b.WriteString(k)
			b.WriteString(": Name=")
			b.WriteString(cfg.Name)
			b.WriteString(", Version=")
			b.WriteString(cfg.Version)
			b.WriteString(", Main=")
			b.WriteString(cfg.Main)
			b.WriteString(", Module=")
			b.WriteString(cfg.Module)
			b.WriteString("\n")
		}
	} else {
		b.WriteString("    <nil>\n")
	}

	// subpackageResolvers (already sorted by path length)
	b.WriteString("  subpackageResolvers:\n")
	for _, subPkg := range rm.SubpackageResolvers() {
		b.WriteString("    ")
		b.WriteString(subPkg.PkgPath)
		b.WriteString(":\n")
		b.WriteString(stringifyModuleResolver(subPkg.Resolver, "      "))
	}

	// rootResolver
	b.WriteString("  rootResolver:\n")
	if rm.RootResolver() != nil {
		b.WriteString(stringifyModuleResolver(rm.RootResolver(), "    "))
	} else {
		b.WriteString("    <nil>\n")
	}

	// cwdResolver
	b.WriteString("  cwdResolver:\n")
	if rm.CwdResolver() != nil {
		if rm.RootResolver() != nil && rm.CwdResolver() == rm.RootResolver() {
			b.WriteString("    (same as rootResolver)\n")
		} else {
			b.WriteString(stringifyModuleResolver(rm.CwdResolver(), "    "))
		}
	} else {
		b.WriteString("    <nil>\n")
	}

	return []byte(b.String())
}

func stringifyModuleResolver(mr *ModuleResolver, indent string) string {
	var b strings.Builder
	if mr == nil {
		b.WriteString(indent)
		b.WriteString("<nil>\n")
		return b.String()
	}

	b.WriteString(indent)
	b.WriteString("resolverRoot: ")
	b.WriteString(mr.ResolverRoot())
	b.WriteString("\n")

	// aliasesCache
	keys := make([]string, 0, len(mr.AliasesCache()))
	for k := range mr.AliasesCache() {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteString(indent)
	b.WriteString("aliasesCache:\n")
	for _, k := range keys {
		val := mr.AliasesCache()[k]
		b.WriteString(indent)
		b.WriteString("  ")
		b.WriteString(k)
		b.WriteString(" = ")
		b.WriteString(val.Path)
		b.WriteString(" (")
		b.WriteString(ResolvedImportTypeToString(val.Type))
		b.WriteString(")\n")
	}

	// nodeModules (sorted)
	nodeKeys := make([]string, 0, len(mr.NodeModules()))
	for k := range mr.NodeModules() {
		nodeKeys = append(nodeKeys, k)
	}
	sort.Strings(nodeKeys)
	b.WriteString(indent)
	b.WriteString("nodeModules:\n")
	for _, k := range nodeKeys {
		b.WriteString(indent)
		b.WriteString("  - ")
		b.WriteString(k)
		b.WriteString("\n")
	}

	// tsConfigParsed aliases
	if mr.TsConfigParsed() != nil {
		aliasKeys := make([]string, 0, len(mr.TsConfigParsed().Aliases))
		for k := range mr.TsConfigParsed().Aliases {
			aliasKeys = append(aliasKeys, k)
		}
		sort.Strings(aliasKeys)
		b.WriteString(indent)
		b.WriteString("tsConfigParsed.aliases:\n")
		for _, k := range aliasKeys {
			b.WriteString(indent)
			b.WriteString("  ")
			b.WriteString(k)
			b.WriteString(" = ")
			b.WriteString(mr.TsConfigParsed().Aliases[k])
			b.WriteString("\n")
		}
	}

	// packageJsonImports keys
	if mr.PackageJsonImports() != nil {
		impKeys := make([]string, 0, len(mr.PackageJsonImports().Imports))
		for k := range mr.PackageJsonImports().Imports {
			impKeys = append(impKeys, k)
		}
		sort.Strings(impKeys)
		b.WriteString(indent)
		b.WriteString("packageJsonImports.keys:\n")
		for _, k := range impKeys {
			b.WriteString(indent)
			b.WriteString("  - ")
			b.WriteString(k)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func ResolutionErrorToString(err ResolutionError) string {
	switch err {
	case AliasNotResolved:
		return "AliasNotResolved"
	case FileNotFound:
		return "FileNotFound"
	default:
		return "Unknown"
	}
}

func stringifyParsedTsConfig(tsConfigParsed *TsConfigParsed) string {
	result := ""

	for key, val := range tsConfigParsed.Aliases {
		result += key + ":" + val + "\n"
	}

	result += "\n___________\n"

	result += "\n___________\n"

	for _, val := range tsConfigParsed.AliasesRegexps {
		result += fmt.Sprintf("%v", val) + "\n"
	}

	return result
}

func stringifyPackageJsonImports(pji *PackageJsonImports) string {
	if pji == nil {
		return "PackageJsonImports: nil"
	}

	var builder strings.Builder
	builder.WriteString("PackageJsonImports:\n")
	builder.WriteString("  imports:\n")

	if len(pji.Imports) == 0 {
		builder.WriteString("    (empty)\n")
	} else {
		for key, value := range pji.Imports {
			builder.WriteString(fmt.Sprintf("    %s: %v\n", key, value))
		}
	}

	builder.WriteString("\n  simpleImportTargetsByKey:\n")
	if len(pji.SimpleImportTargetsByKey) == 0 {
		builder.WriteString("    (empty)\n")
	} else {
		for key, value := range pji.SimpleImportTargetsByKey {
			builder.WriteString(fmt.Sprintf("    %s: %s\n", key, value))
		}
	}

	builder.WriteString("\n  conditionalImportTargetsByKey:\n")
	if len(pji.ConditionalImportTargetsByKey) == 0 {
		builder.WriteString("    (empty)\n")
	} else {
		for key, conditions := range pji.ConditionalImportTargetsByKey {
			builder.WriteString(fmt.Sprintf("    %s:\n", key))
			if len(conditions) == 0 {
				builder.WriteString("      (no conditions)\n")
			} else {
				for condition, regex := range conditions {
					builder.WriteString(fmt.Sprintf("      %s: %s\n", condition, regex.String()))
				}
			}
		}
	}

	builder.WriteString("\n  importsRegexps:\n")
	if len(pji.ImportsRegexps) == 0 {
		builder.WriteString("    (empty)\n")
	} else {
		for _, item := range pji.ImportsRegexps {
			builder.WriteString(fmt.Sprintf("    %s: %s\n", item.AliasKey, item.RegExp.String()))
		}
	}

	builder.WriteString("\n  wildcardPatterns:\n")
	if len(pji.WildcardPatterns) == 0 {
		builder.WriteString("    (empty)\n")
	} else {
		for _, pattern := range pji.WildcardPatterns {
			builder.WriteString(fmt.Sprintf("    key: %s, prefix: %s, suffix: %s\n", pattern.Key, pattern.Prefix, pattern.Suffix))
		}
	}

	builder.WriteString("\n  conditionNames:\n")
	if len(pji.ConditionNames) == 0 {
		builder.WriteString("    (none)\n")
	} else {
		for i, name := range pji.ConditionNames {
			builder.WriteString(fmt.Sprintf("    [%d]: %s\n", i, name))
		}
	}

	builder.WriteString("\n  parsedImportTargets:\n")
	if len(pji.ParsedImportTargets) == 0 {
		builder.WriteString("    (empty)\n")
	} else {
		for key, node := range pji.ParsedImportTargets {
			builder.WriteString(fmt.Sprintf("    %s:\n", key))
			builder.WriteString(stringifyImportTargetTreeNode(node, "      "))
		}
	}

	return builder.String()
}

func stringifyImportTargetTreeNode(node *ImportTargetTreeNode, indent string) string {
	if node == nil {
		return indent + "(nil)\n"
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%snodeType: %s\n", indent, node.NodeType))

	if node.NodeType == LeafNode {
		builder.WriteString(fmt.Sprintf("%svalue: %s\n", indent, node.Value))
	} else if node.NodeType == MapNode {
		if len(node.ConditionsMap) == 0 {
			builder.WriteString(indent + "conditionsMap: (empty)\n")
		} else {
			builder.WriteString(indent + "conditionsMap:\n")
			for condition, childNode := range node.ConditionsMap {
				builder.WriteString(fmt.Sprintf("%s  %s:\n", indent, condition))
				builder.WriteString(stringifyImportTargetTreeNode(childNode, indent+"    "))
			}
		}
	}

	return builder.String()
}

// StringifyImportConventionViolation returns a string representation of ImportConventionViolation
// including the nested Change struct if present
func StringifyImportConventionViolation(violation ImportConventionViolation, cwd string) string {
	var builder strings.Builder

	builder.WriteString("ImportConventionViolation:\n")
	builder.WriteString(fmt.Sprintf("  FilePath: %s\n", strings.Replace(violation.FilePath, cwd, "", 1)))
	builder.WriteString(fmt.Sprintf("  ImportRequest: %s\n", violation.ImportRequest))
	builder.WriteString(fmt.Sprintf("  ImportIndex: %d\n", violation.ImportIndex))
	builder.WriteString(fmt.Sprintf("  ViolationType: %s\n", violation.ViolationType))

	if violation.Fix != nil {
		builder.WriteString("  Fix:\n")
		builder.WriteString(fmt.Sprintf("    Start: %d\n", violation.Fix.Start))
		builder.WriteString(fmt.Sprintf("    End: %d\n", violation.Fix.End))
		builder.WriteString(fmt.Sprintf("    Text: %s\n", violation.Fix.Text))
	} else {
		builder.WriteString("  Fix: <nil>\n")
	}

	return builder.String()
}
