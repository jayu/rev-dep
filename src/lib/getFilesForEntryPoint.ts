import { sanitizeUserEntryPoints, resolvePath } from './utils'
import { getDepsTree } from './getDepsTree'
import { MinimalDependencyTree } from './types'

export async function getFilesForEntryPoint({
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
      ignoreTypesImports
    ))

  const filePaths = Object.keys(depsTree)

  return filePaths.sort()
}
