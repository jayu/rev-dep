package model

type ImportKind uint8

const (
	NotTypeOrMixedImport ImportKind = iota
	OnlyTypeImport
)

type ResolvedImportType uint8

const (
	UserModule ResolvedImportType = iota
	NodeModule
	BuiltInModule
	ExcludedByUser
	NotResolvedModule
	AssetModule
	MonorepoModule
	LocalExportDeclaration
)

type ParseMode uint8

const (
	ParseModeBasic    ParseMode = iota // Current behavior
	ParseModeDetailed                  // Keyword tracking + local exports
)

type NodeModulesMatchingStrategy uint8

const (
	NodeModulesMatchingStrategySelfResolver NodeModulesMatchingStrategy = iota
	NodeModulesMatchingStrategyRootResolver
	NodeModulesMatchingStrategyCwdResolver
)

type KeywordInfo struct {
	Name       string // Original name ("default" for default imports, "*" for namespace)
	Alias      string // Local alias if "as" used, empty otherwise
	Start      uint32 // Byte offset of identifier start in source
	End        uint32 // Byte offset of identifier end in source
	Position   uint32 // 0-based position in the import/export list
	CommaAfter uint32 // Byte offset of `,` after this entry (0 if no trailing comma)
	IsType     bool   // true if inline "type" keyword precedes this identifier
}

type KeywordMap struct {
	Keywords []KeywordInfo  // Insertion-ordered slice; primary storage
	index    map[string]int // Name -> index; lazily built on first Get() call
}

func (km *KeywordMap) Get(name string) (KeywordInfo, bool) {
	if km.index == nil {
		km.index = make(map[string]int, len(km.Keywords))
		for i, kw := range km.Keywords {
			km.index[kw.Name] = i
		}
	}
	idx, ok := km.index[name]
	if !ok {
		return KeywordInfo{}, false
	}
	return km.Keywords[idx], true
}

func (km *KeywordMap) Add(kw KeywordInfo) {
	km.Keywords = append(km.Keywords, kw)
	km.index = nil // invalidate index
}

func (km *KeywordMap) Len() int {
	return len(km.Keywords)
}

type Import struct {
	Request      string             `json:"request"`
	PathOrName   string             `json:"path"`
	Keywords     *KeywordMap        `json:"-"` // nil in basic mode
	RequestStart uint32             `json:"requestStart"`
	RequestEnd   uint32             `json:"requestEnd"`
	Kind         ImportKind         `json:"kind"`
	ResolvedType ResolvedImportType `json:"resolvedType"`

	IsDynamicImport bool `json:"-"` // true for `import('...')`
	IsLocalExport   bool `json:"-"` // true for `export const/default/function/...` without `from`

	// New fields — populated only in ParseModeDetailed
	ExportKeyStart     uint32 `json:"-"` // Byte offset where `export` keyword starts
	ExportKeyEnd       uint32 `json:"-"` // Byte offset right after `export `
	ExportDeclStart    uint32 `json:"-"` // After `export [default] ` — where the declaration starts
	ExportBraceStart   uint32 `json:"-"` // Position of `{` in brace-list exports (0 if not brace-list)
	ExportBraceEnd     uint32 `json:"-"` // Position after `}` in brace-list exports
	ExportStatementEnd uint32 `json:"-"` // Position after full statement including optional `;`
}

type FileImports struct {
	FilePath string   `json:"filePath"`
	Imports  []Import `json:"imports"`
}

type FollowMonorepoPackagesValue struct {
	FollowAll bool
	Packages  map[string]bool
}

func (f FollowMonorepoPackagesValue) IsEnabled() bool {
	return f.FollowAll || len(f.Packages) > 0
}

func (f FollowMonorepoPackagesValue) ShouldFollowAll() bool {
	return f.FollowAll
}

func (f FollowMonorepoPackagesValue) ShouldFollowPackage(name string) bool {
	if f.FollowAll {
		return true
	}

	return f.Packages[name]
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
