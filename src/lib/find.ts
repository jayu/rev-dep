import { getDepsSetWebpack } from './getDepsSetWebpack'
import { parseDependencyTree } from 'dpdm'
import { Node } from './types'

import escapeGlob from 'glob-escape'
import { getTreeForEntryPointsSearch } from './findEntryPoints'
import { getMaxDepth } from './getMaxDepthInGrapth'
import { removeInitialDot, _resolveAbsolutePath } from './utils'
import { cleanupDpdmDeps } from './cleanupDpdmDeps'
import { buildGraphDpdm } from './buildDepsGraph'

const resolvePathsToRoot = (
  node: Node,
  onlyFirst = false,
  resolvedPaths: Array<Array<string>> = [[]]
): Array<Array<string>> => {
  const newPaths = resolvedPaths.map((resolvedPath) => [
    node.path,
    ...resolvedPath
  ])
  if (node.parents.length === 0) {
    return newPaths
  }

  if (onlyFirst) {
    return resolvePathsToRoot(node.parents[0], onlyFirst, newPaths)
  }
  return node.parents
    .map((parentPath) => resolvePathsToRoot(parentPath, false, newPaths))
    .flat(1)
}

type FindParams = {
  entryPoints: string[]
  filePath: string
  skipRegex?: RegExp
  cwd?: string
  compactSummary?: boolean
  webpackConfig?: string
  typescriptConfig?: string
  printMaxDepth?: boolean
  printDependentCount?: boolean
  checkOnly?: boolean
}

export const resolve = async ({
  entryPoints: _entryPoints,
  filePath,
  skipRegex,
  webpackConfig,
  typescriptConfig,
  cwd = process.cwd(),
  printMaxDepth,
  printDependentCount,
  checkOnly
}: FindParams) => {
  const resolveAbsolutePath = _resolveAbsolutePath(cwd)
  const entryPoints =
    _entryPoints.length > 0
      ? _entryPoints
      : await getTreeForEntryPointsSearch(cwd)
  const absoluteEntryPoints = entryPoints.map(resolveAbsolutePath) as string[]
  const globEscapedEntryPoints = entryPoints.map(escapeGlob)

  const deps = webpackConfig
    ? getDepsSetWebpack(
        absoluteEntryPoints,
        skipRegex,
        resolveAbsolutePath(webpackConfig) as string
      )
    : cleanupDpdmDeps(
        await parseDependencyTree(globEscapedEntryPoints, {
          context: cwd
        })
      )

  const cleanedEntryPoints = entryPoints.map(removeInitialDot)
  const cleanedFilePath = removeInitialDot(filePath)

  const forest = cleanedEntryPoints.map(buildGraphDpdm(deps, cleanedFilePath))

  if (printMaxDepth) {
    forest.forEach((maybeTree) => {
      const tree = typescriptConfig ? maybeTree[0] : maybeTree
      console.log('Max depth', ...getMaxDepth()(tree))
    })
  }

  //todo it does not work properly for multiple entry points
  // Need to count vertices from graph
  if (printDependentCount) {
    console.log('Deps count ', deps.length || Object.keys(deps).length)
  }

  const resolvedPaths = forest.reduce((allPaths, [tree, fileNode], idx) => {
    if (!fileNode) {
      return [...allPaths, []]
    }
    const pathsForTree = resolvePathsToRoot(fileNode, checkOnly)

    return [...allPaths, pathsForTree]
  }, [])
  return resolvedPaths as Array<Array<Array<string>>>
}
