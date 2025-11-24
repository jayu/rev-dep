package main

import (
	"sort"
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
