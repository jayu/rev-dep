package main

// Output structures for minimal dependency tree
type MinimalDependency struct {
	ID           *string            `json:"id"`
	Request      string             `json:"request"`
	ResolvedType ResolvedImportType `json:"resolvedType"`
	ImportKind   *ImportKind        `json:"importKind"`
}

type MinimalDependencyTree map[string][]MinimalDependency

func TransformToMinimalDependencyTreeCustomParser(fileImportsArr []FileImports) MinimalDependencyTree {
	result := make(MinimalDependencyTree)

	processedFiles := 0
	processedImports := 0
	for _, fileImports := range fileImportsArr {
		processedFiles++
		imports := fileImports.Imports
		filePath := fileImports.FilePath
		var dependencies []MinimalDependency

		for _, imp := range imports {
			processedImports++

			dependency := MinimalDependency{
				ID:           &imp.PathOrName, // Set to the requested file path
				Request:      imp.Request,
				ResolvedType: imp.ResolvedType,
				ImportKind:   &imp.Kind,
			}

			dependencies = append(dependencies, dependency)

			// If this import was excluded by the user, ensure it exists as a key
			// in the result map with an empty dependency list.
			if imp.ResolvedType == ExcludedByUser && imp.PathOrName != "" {
				if _, exists := result[imp.PathOrName]; !exists {
					result[imp.PathOrName] = []MinimalDependency{}
				}
			}

		}

		result[filePath] = dependencies

	}

	return result
}
