import { getDepsSetWebpack } from './getDepsSetWebpack'
import { parseDependencyTree } from 'dpdm'
import { cleanupDpdmDeps } from './cleanupDpdmDeps'

export async function getDepsTree(
  cwd: string,
  entryPoints: string[],
  webpackConfigPath?: string
) {
  const deps = webpackConfigPath
    ? getDepsSetWebpack(entryPoints, webpackConfigPath, cwd)
    : cleanupDpdmDeps(
        await parseDependencyTree(entryPoints, {
          context: cwd
        })
      )
  return deps
}
