package debugutil

import (
	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/model"
	"rev-dep-go/internal/resolve"
)

type MinimalDependency = model.MinimalDependency

type MinimalDependencyTree = model.MinimalDependencyTree

type FileImports = model.FileImports

type Import = model.Import

type ImportKind = model.ImportKind

type ResolvedImportType = model.ResolvedImportType

const (
	NotTypeOrMixedImport = model.NotTypeOrMixedImport
	OnlyTypeImport       = model.OnlyTypeImport
)

type ResolutionError = resolve.ResolutionError

const (
	AliasNotResolved = resolve.AliasNotResolved
	FileNotFound     = resolve.FileNotFound
)

type ResolverManager = resolve.ResolverManager

type ModuleResolver = resolve.ModuleResolver

type TsConfigParsed = resolve.TsConfigParsed

type PackageJsonImports = resolve.PackageJsonImports

type ImportTargetTreeNode = resolve.ImportTargetTreeNode

type NodeType = resolve.NodeType

const (
	LeafNode = resolve.LeafNode
	MapNode  = resolve.MapNode
)

type ImportConventionViolation = checks.ImportConventionViolation

func ImportKindToString(kind ImportKind) string {
	return model.ImportKindToString(kind)
}

func ResolvedImportTypeToString(resolvedType ResolvedImportType) string {
	return model.ResolvedImportTypeToString(resolvedType)
}
