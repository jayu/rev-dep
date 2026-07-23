package resolve

import (
	"maps"

	"rev-dep-go/internal/monorepo"
)

func (rm *ResolverManager) MonorepoContext() *monorepo.MonorepoContext {
	return rm.monorepoContext
}

func (rm *ResolverManager) FollowMonorepoPackages() FollowMonorepoPackagesValue {
	return rm.followMonorepoPackages
}

func (rm *ResolverManager) ConditionNames() []string {
	return rm.conditionNames
}

func (rm *ResolverManager) Input() ResolverManagerInput {
	return rm.input
}

// FilesAndExtensions returns a snapshot of the discovered-file index, copied under the
// manager's lock. It deliberately does not expose the live map: that map is written by the
// concurrent resolution goroutines, so a caller ranging over the real one would be reading
// it unsynchronised. See ResolverManager.filesAndExtensionsMu.
func (rm *ResolverManager) FilesAndExtensions() map[string]string {
	rm.filesAndExtensionsMu.RLock()
	defer rm.filesAndExtensionsMu.RUnlock()
	if rm.filesAndExtensions == nil {
		return nil
	}
	return maps.Clone(*rm.filesAndExtensions)
}

func (rm *ResolverManager) SubpackageResolvers() []SubpackageResolver {
	return rm.subpackageResolvers
}

func (rm *ResolverManager) RootResolver() *ModuleResolver {
	return rm.rootResolver
}

func (rm *ResolverManager) CwdResolver() *ModuleResolver {
	return rm.cwdResolver
}

func NewResolverManagerWithMonorepoContext(ctx *monorepo.MonorepoContext) *ResolverManager {
	return &ResolverManager{monorepoContext: ctx}
}

func NewResolverManagerForTests(ctx *monorepo.MonorepoContext, subpackageResolvers []SubpackageResolver, rootResolver *ModuleResolver) *ResolverManager {
	return &ResolverManager{
		monorepoContext:     ctx,
		subpackageResolvers: subpackageResolvers,
		rootResolver:        rootResolver,
	}
}

func (rm *ResolverManager) SetSubpackageResolvers(resolvers []SubpackageResolver) {
	rm.subpackageResolvers = resolvers
}

func (rm *ResolverManager) SetRootResolver(resolver *ModuleResolver) {
	rm.rootResolver = resolver
}

func (rm *ResolverManager) SetFilesAndExtensions(filesAndExtensions *map[string]string) {
	rm.filesAndExtensions = filesAndExtensions
}

func AddFilePathToFilesAndExtensions(filePath string, filesAndExtensions *map[string]string) {
	addFilePathToFilesAndExtensions(filePath, filesAndExtensions)
}

func NewModuleResolverForTests(tsConfigParsed *TsConfigParsed, resolverRoot string) *ModuleResolver {
	return &ModuleResolver{
		tsConfigParsed: tsConfigParsed,
		resolverRoot:   resolverRoot,
	}
}

func (mr *ModuleResolver) NodeModules() map[string]bool {
	return mr.nodeModules
}

func (mr *ModuleResolver) DevNodeModules() map[string]bool {
	return mr.devNodeModules
}

func (mr *ModuleResolver) PackageJSONPath() string {
	return mr.packageJsonPath
}

func (mr *ModuleResolver) TsConfigParsed() *TsConfigParsed {
	return mr.tsConfigParsed
}

func (mr *ModuleResolver) PackageJsonImports() *PackageJsonImports {
	return mr.packageJsonImports
}

// AliasesCache returns a snapshot of the alias memo, copied under the resolver's lock. Like
// FilesAndExtensions it must not expose the live map, which the resolution goroutines write
// through cacheAlias.
func (mr *ModuleResolver) AliasesCache() map[string]ResolvedModuleInfo {
	mr.aliasesCacheMu.RLock()
	defer mr.aliasesCacheMu.RUnlock()
	return maps.Clone(mr.aliasesCache)
}

func (mr *ModuleResolver) ResolverRoot() string {
	return mr.resolverRoot
}
