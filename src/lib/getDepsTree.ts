import { getDepsSetWebpack } from './getDepsSetWebpack'
import { parseDependencyTree } from 'dpdm'
import { cleanupDpdmDeps } from './cleanupDpdmDeps'

export async function getDepsTree(
  cwd: string,
  entryPoints: string[],
  webpackConfigPath?: string,
  ignoreTypesImports = false
) {
  let deps

  if (webpackConfigPath) {
    deps = getDepsSetWebpack(entryPoints, webpackConfigPath, cwd)
  } else {
    // dpdm does not support custom search directory :/
    const oldProcessCwd = process.cwd
    process.cwd = () => cwd

    deps = cleanupDpdmDeps(
      await parseDependencyTree(entryPoints, {
        context: cwd,
        transform: ignoreTypesImports
      })
    )

    process.cwd = oldProcessCwd
  }

  return deps
}
