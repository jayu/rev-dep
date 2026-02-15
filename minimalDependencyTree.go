package main

// Output structures for minimal dependency tree
type MinimalDependency struct {
	ID              string             `json:"id"`
	Request         string             `json:"request"`
	ResolvedType    ResolvedImportType `json:"resolvedType"`
	ImportKind      ImportKind         `json:"importKind"`
	RequestStart    uint32             `json:"requestStart"`
	RequestEnd      uint32             `json:"requestEnd"`
	IsDynamicImport bool               `json:"-"`
	// Detailed mode fields (nil/zero in basic mode)
	Keywords           *KeywordMap `json:"-"`
	IsLocalExport      bool        `json:"-"`
	ExportKeyStart     uint32      `json:"-"`
	ExportKeyEnd       uint32      `json:"-"`
	ExportDeclStart    uint32      `json:"-"`
	ExportBraceStart   uint32      `json:"-"`
	ExportBraceEnd     uint32      `json:"-"`
	ExportStatementEnd uint32      `json:"-"`
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
				ID:              imp.PathOrName,
				Request:         imp.Request,
				ResolvedType:    imp.ResolvedType,
				ImportKind:      imp.Kind,
				RequestStart:    imp.RequestStart,
				RequestEnd:      imp.RequestEnd,
				IsDynamicImport: imp.IsDynamicImport,
				// Copy detailed fields (nil/zero when ParseModeBasic)
				Keywords:           imp.Keywords,
				IsLocalExport:      imp.IsLocalExport,
				ExportKeyStart:     imp.ExportKeyStart,
				ExportKeyEnd:       imp.ExportKeyEnd,
				ExportDeclStart:    imp.ExportDeclStart,
				ExportBraceStart:   imp.ExportBraceStart,
				ExportBraceEnd:     imp.ExportBraceEnd,
				ExportStatementEnd: imp.ExportStatementEnd,
			}

			dependencies = append(dependencies, dependency)

		}

		result[filePath] = dependencies

	}

	return result
}
