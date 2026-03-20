package resolve

import (
	"rev-dep-go/internal/model"
	"rev-dep-go/internal/monorepo"
)

type ImportKind = model.ImportKind

type ResolvedImportType = model.ResolvedImportType

type ParseMode = model.ParseMode

type NodeModulesMatchingStrategy = model.NodeModulesMatchingStrategy

type KeywordMap = model.KeywordMap

type FileImports = model.FileImports

type Import = model.Import

type FollowMonorepoPackagesValue = model.FollowMonorepoPackagesValue

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

type NodeType = monorepo.NodeType

const (
	LeafNode = monorepo.LeafNode
	MapNode  = monorepo.MapNode
)

type ImportTargetTreeNode = monorepo.ImportTargetTreeNode

type WildcardPattern = monorepo.WildcardPattern

type PackageJsonExports = monorepo.PackageJsonExports

type MonorepoContext = monorepo.MonorepoContext
