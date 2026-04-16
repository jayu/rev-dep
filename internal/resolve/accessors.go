package resolve

import "rev-dep-go/internal/monorepo"

func (rm *ResolverManager) MonorepoContext() *monorepo.MonorepoContext {
	return rm.monorepoContext
}

func (rm *ResolverManager) FollowMonorepoPackages() FollowMonorepoPackagesValue {
	return rm.followMonorepoPackages
}

func (rm *ResolverManager) ConditionNames() []string {
	return rm.conditionNames
}

func (rm *ResolverManager) RootParams() RootParams {
	return rm.rootParams
}

func (rm *ResolverManager) FilesAndExtensions() *map[string]string {
	return rm.filesAndExtensions
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

func (mr *ModuleResolver) AliasesCache() map[string]ResolvedModuleInfo {
	return mr.aliasesCache
}

func (mr *ModuleResolver) ResolverRoot() string {
	return mr.resolverRoot
}
