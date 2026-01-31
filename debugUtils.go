package main

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
			imports = append(imports, []string{*imp.ID, imp.Request})
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
			var idI, idJ string
			if deps[i].ID != nil {
				idI = *deps[i].ID
			}
			if deps[j].ID != nil {
				idJ = *deps[j].ID
			}
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
			if d.ID != nil && *d.ID != "" {
				id = *d.ID
			}
			importKind := "<nil>"
			if d.ImportKind != nil {
				importKind = ImportKindToString(*d.ImportKind)
			}
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
	b.WriteString(fmt.Sprintf("  followMonorepoPackages: %v\n", rm.followMonorepoPackages))

	// conditionNames (sorted)
	cond := make([]string, len(rm.conditionNames))
	copy(cond, rm.conditionNames)
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
	b.WriteString(rm.rootParams.Cwd)
	b.WriteString("\n")
	// SortedFiles (sort for determinism)
	files := make([]string, len(rm.rootParams.SortedFiles))
	copy(files, rm.rootParams.SortedFiles)
	sort.Strings(files)
	b.WriteString("    SortedFiles:\n")
	for _, f := range files {
		b.WriteString("      - ")
		b.WriteString(f)
		b.WriteString("\n")
	}

	// filesAndExtensions
	b.WriteString("  filesAndExtensions:\n")
	if rm.filesAndExtensions != nil {
		// dereference map
		m := *rm.filesAndExtensions
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
	if rm.monorepoContext != nil {
		mc := rm.monorepoContext
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
	for _, subPkg := range rm.subpackageResolvers {
		b.WriteString("    ")
		b.WriteString(subPkg.PkgPath)
		b.WriteString(":\n")
		b.WriteString(stringifyModuleResolver(subPkg.Resolver, "      "))
	}

	// rootResolver
	b.WriteString("  rootResolver:\n")
	if rm.rootResolver != nil {
		b.WriteString(stringifyModuleResolver(rm.rootResolver, "    "))
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
	b.WriteString(mr.resolverRoot)
	b.WriteString("\n")

	// aliasesCache
	keys := make([]string, 0, len(mr.aliasesCache))
	for k := range mr.aliasesCache {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteString(indent)
	b.WriteString("aliasesCache:\n")
	for _, k := range keys {
		val := mr.aliasesCache[k]
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
	nodeKeys := make([]string, 0, len(mr.nodeModules))
	for k := range mr.nodeModules {
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
	if mr.tsConfigParsed != nil {
		aliasKeys := make([]string, 0, len(mr.tsConfigParsed.aliases))
		for k := range mr.tsConfigParsed.aliases {
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
			b.WriteString(mr.tsConfigParsed.aliases[k])
			b.WriteString("\n")
		}
	}

	// packageJsonImports keys
	if mr.packageJsonImports != nil {
		impKeys := make([]string, 0, len(mr.packageJsonImports.imports))
		for k := range mr.packageJsonImports.imports {
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

func ImportKindToString(kind ImportKind) string {
	switch kind {
	case NotTypeOrMixedImport:
		return "NotTypeOrMixedImport"
	case OnlyTypeImport:
		return "OnlyTypeImport"
	default:
		return "Unknown"
	}
}

func ResolvedImportTypeToString(resolvedType ResolvedImportType) string {
	switch resolvedType {
	case UserModule:
		return "UserModule"
	case NodeModule:
		return "NodeModule"
	case BuiltInModule:
		return "BuiltInModule"
	case ExcludedByUser:
		return "ExcludedByUser"
	case NotResolvedModule:
		return "NotResolvedModule"
	case AssetModule:
		return "AssetModule"
	case MonorepoModule:
		return "MonorepoModule"
	default:
		return "Unknown"
	}
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

	for key, val := range tsConfigParsed.aliases {
		result += key + ":" + val + "\n"
	}

	result += "\n___________\n"

	result += "\n___________\n"

	for _, val := range tsConfigParsed.aliasesRegexps {
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

	if len(pji.imports) == 0 {
		builder.WriteString("    (empty)\n")
	} else {
		for key, value := range pji.imports {
			builder.WriteString(fmt.Sprintf("    %s: %v\n", key, value))
		}
	}

	builder.WriteString("\n  simpleImportTargetsByKey:\n")
	if len(pji.simpleImportTargetsByKey) == 0 {
		builder.WriteString("    (empty)\n")
	} else {
		for key, value := range pji.simpleImportTargetsByKey {
			builder.WriteString(fmt.Sprintf("    %s: %s\n", key, value))
		}
	}

	builder.WriteString("\n  conditionalImportTargetsByKey:\n")
	if len(pji.conditionalImportTargetsByKey) == 0 {
		builder.WriteString("    (empty)\n")
	} else {
		for key, conditions := range pji.conditionalImportTargetsByKey {
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
	if len(pji.importsRegexps) == 0 {
		builder.WriteString("    (empty)\n")
	} else {
		for _, item := range pji.importsRegexps {
			builder.WriteString(fmt.Sprintf("    %s: %s\n", item.aliasKey, item.regExp.String()))
		}
	}

	builder.WriteString("\n  wildcardPatterns:\n")
	if len(pji.wildcardPatterns) == 0 {
		builder.WriteString("    (empty)\n")
	} else {
		for _, pattern := range pji.wildcardPatterns {
			builder.WriteString(fmt.Sprintf("    key: %s, prefix: %s, suffix: %s\n", pattern.key, pattern.prefix, pattern.suffix))
		}
	}

	builder.WriteString("\n  conditionNames:\n")
	if len(pji.conditionNames) == 0 {
		builder.WriteString("    (none)\n")
	} else {
		for i, name := range pji.conditionNames {
			builder.WriteString(fmt.Sprintf("    [%d]: %s\n", i, name))
		}
	}

	builder.WriteString("\n  parsedImportTargets:\n")
	if len(pji.parsedImportTargets) == 0 {
		builder.WriteString("    (empty)\n")
	} else {
		for key, node := range pji.parsedImportTargets {
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
	builder.WriteString(fmt.Sprintf("%snodeType: %s\n", indent, node.nodeType))

	if node.nodeType == LeafNode {
		builder.WriteString(fmt.Sprintf("%svalue: %s\n", indent, node.value))
	} else if node.nodeType == MapNode {
		if len(node.conditionsMap) == 0 {
			builder.WriteString(indent + "conditionsMap: (empty)\n")
		} else {
			builder.WriteString(indent + "conditionsMap:\n")
			for condition, childNode := range node.conditionsMap {
				builder.WriteString(fmt.Sprintf("%s  %s:\n", indent, condition))
				builder.WriteString(stringifyImportTargetTreeNode(childNode, indent+"    "))
			}
		}
	}

	return builder.String()
}
