import { sanitizeUserEntryPoints, resolvePath } from './utils'
import { getDepsTree } from './getDepsTree'
import { MinimalDependencyTree } from './types'

export async function getNodeModulesForEntryPoint({
  cwd,
  entryPoint,
  webpackConfigPath,
  ignoreTypesImports,
  depsTree: initDepsTree
}: {
  cwd: string
  entryPoint: string
  ignoreTypesImports: boolean
  webpackConfigPath?: string
  depsTree?: MinimalDependencyTree
}) {
  const sanitizedEntryPoints = sanitizeUserEntryPoints([entryPoint])

  const depsTree =
    initDepsTree ??
    (await getDepsTree(
      resolvePath(cwd),
      sanitizedEntryPoints,
      webpackConfigPath,
      ignoreTypesImports,
      true
    ))

  const nodeModuleImports = Object.values(depsTree)
    .filter((depsTree) => depsTree !== null)
    .flat(2)
    .filter((dep) => dep?.id && dep.id.includes('node_modules'))
    .map((dep) => dep?.request)
    .filter(Boolean)

  const uniqueNodeModuleImports = [...new Set(nodeModuleImports)].sort()

  return uniqueNodeModuleImports
}
