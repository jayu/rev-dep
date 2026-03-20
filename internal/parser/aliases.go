package parser

import "rev-dep-go/internal/model"

type ImportKind = model.ImportKind

type ResolvedImportType = model.ResolvedImportType

type ParseMode = model.ParseMode

type NodeModulesMatchingStrategy = model.NodeModulesMatchingStrategy

type KeywordInfo = model.KeywordInfo

type KeywordMap = model.KeywordMap

type Import = model.Import

type FileImports = model.FileImports

const (
	NotTypeOrMixedImport = model.NotTypeOrMixedImport
	OnlyTypeImport       = model.OnlyTypeImport
)

const (
	UserModule             = model.UserModule
	NodeModule             = model.NodeModule
	BuiltInModule          = model.BuiltInModule
	ExcludedByUser         = model.ExcludedByUser
	NotResolvedModule      = model.NotResolvedModule
	AssetModule            = model.AssetModule
	MonorepoModule         = model.MonorepoModule
	LocalExportDeclaration = model.LocalExportDeclaration
)

const (
	ParseModeBasic    = model.ParseModeBasic
	ParseModeDetailed = model.ParseModeDetailed
)

const (
	NodeModulesMatchingStrategySelfResolver = model.NodeModulesMatchingStrategySelfResolver
	NodeModulesMatchingStrategyRootResolver = model.NodeModulesMatchingStrategyRootResolver
	NodeModulesMatchingStrategyCwdResolver  = model.NodeModulesMatchingStrategyCwdResolver
)
