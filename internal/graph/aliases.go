package graph

import "rev-dep-go/internal/model"

type ImportKind = model.ImportKind

type ResolvedImportType = model.ResolvedImportType

type MinimalDependency = model.MinimalDependency

type MinimalDependencyTree = model.MinimalDependencyTree

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
