import { buildDepsGraph } from './buildDepsGraph'
import { getDepsTree } from './getDepsTree'
import { getEntryPoints } from './getEntryPoints'
import { Node } from './types'
import { removeInitialDot, sanitizeUserEntryPoints } from './utils'

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
    /*
     * If there is only one path, and has length of 1, it's file self reference
     * It's invalid result, so we return empty paths in that case
     */
    if (newPaths.length === 1 && newPaths[0].length === 1) {
      return []
    }
    return newPaths
  }

  if (all) {
    return node.parents
      .map((parentPath) => resolvePathsToRoot(parentPath, all, newPaths))
      .flat(1)
  }

  return resolvePathsToRoot(node.parents[0], false, newPaths)
}

type ResolveParams = {
  entryPoints?: string[]
  filePath: string
  webpackConfig?: string
  cwd?: string
  all: boolean
  exclude?: string[]
  include?: string[]
  notTraversePaths?: string[]
  ignoreTypesImports?: boolean
}

export const resolve = async ({
  entryPoints: _entryPoints,
  filePath,
  webpackConfig,
  cwd = process.cwd(),
  all,
  include,
  exclude,
  notTraversePaths,
  ignoreTypesImports
}: ResolveParams) => {
  let deps, entryPoints

  if (_entryPoints && _entryPoints?.length > 0) {
    entryPoints = _entryPoints
    const sanitizedEntryPoints = sanitizeUserEntryPoints(entryPoints)

    deps = await getDepsTree(
      cwd,
      sanitizedEntryPoints,
      webpackConfig,
      ignoreTypesImports
    )
  } else {
    ;[entryPoints, deps] = await getEntryPoints({
      cwd,
      exclude,
      include,
      webpackConfigPath: webpackConfig,
      ignoreTypesImports
    })
  }

  const cleanedEntryPoints = entryPoints.map(removeInitialDot)
  const cleanedFilePath = removeInitialDot(filePath)

  const forest = cleanedEntryPoints.map(
    buildDepsGraph(deps, cleanedFilePath, notTraversePaths)
  )

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

  return [resolvedPaths, entryPoints] as [string[][][], string[]]
}
