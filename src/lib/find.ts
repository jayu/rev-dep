import { buildGraphDpdm } from './buildDepsGraph'
import { getDepsTree } from './getDepsTree'
import { getEntryPoints } from './getEntryPoints'
import { getMaxDepth } from './getMaxDepthInGrapth'
import { Node } from './types'
import { removeInitialDot } from './utils'

const resolvePathsToRoot = (
  node: Node,
  all = false,
  resolvedPaths: Array<Array<string>> = [[]]
): Array<Array<string>> => {
  const newPaths = resolvedPaths.map((resolvedPath) => [
    node.path,
    ...resolvedPath
  ])
  if (node.parents.length === 0) {
    return newPaths
  }

  if (all) {
    return node.parents
      .map((parentPath) => resolvePathsToRoot(parentPath, all, newPaths))
      .flat(1)
  }

  return resolvePathsToRoot(node.parents[0], false, newPaths)
}

type FindParams = {
  entryPoints: string[]
  filePath: string
  webpackConfig?: string
  cwd?: string
  printMaxDepth?: boolean
  all: boolean
}

export const resolve = async ({
  entryPoints: _entryPoints,
  filePath,
  webpackConfig,
  cwd = process.cwd(),
  printMaxDepth,
  all
}: FindParams) => {
  let deps, entryPoints

  if (_entryPoints.length > 0) {
    entryPoints = _entryPoints
    deps = await getDepsTree(cwd, entryPoints, webpackConfig)
  } else {
    ;[entryPoints, deps] = await getEntryPoints({ cwd })
  }

  const cleanedEntryPoints = entryPoints.map(removeInitialDot)
  const cleanedFilePath = removeInitialDot(filePath)

  const forest = cleanedEntryPoints.map(buildGraphDpdm(deps, cleanedFilePath))

  if (printMaxDepth) {
    forest.forEach(([tree]) => {
      console.log('Max depth', ...getMaxDepth()(tree))
    })
  }

  const resolvedPaths = forest.reduce(
    (allPaths, [_, fileNode]): string[][][] => {
      if (!fileNode) {
        return [...allPaths, []]
      }
      const pathsForTree = resolvePathsToRoot(fileNode, all)

      return [...allPaths, pathsForTree]
    },
    [] as string[][][]
  )
  return resolvedPaths as Array<Array<Array<string>>>
}
