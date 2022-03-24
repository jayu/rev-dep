import { MinimalDependencyTree } from './types'
import minimatch from 'minimatch'
import path from 'path'
import fs from 'fs/promises'
import { asyncFilter } from './utils'
import { getDepsTree } from './getDepsTree'
import ignore from 'ignore'

export const getDirectoriesForEntryPointsSearch = async (
  dir: string
): Promise<string[]> => {
  const entries = await fs.readdir(dir)
  const directories = await asyncFilter(entries, async (pathName) => {
    if (pathName === 'node_modules' || pathName.startsWith('.')) {
      return false
    }
    const stat = await fs.lstat(path.resolve(dir, pathName))
    return stat.isDirectory()
  })

  const joinedWithDir = directories.map((pathName) => path.join(dir, pathName))

  return [
    ...joinedWithDir,
    ...(
      await Promise.all(joinedWithDir.map(getDirectoriesForEntryPointsSearch))
    ).flat(1)
  ]
}

export const findEntryPointsInDepsTree = (
  deps: MinimalDependencyTree,
  exclude: string[] = [],
  include: string[] | undefined = undefined
) => {
  const referencedIds = new Set()

  Object.values(deps).forEach((entry) => {
    if (entry !== null) {
      entry.forEach(({ id }) => referencedIds.add(id))
    }
  })

  return Object.keys(deps)
    .filter(
      (id) =>
        /\.(ts|tsx|mjs|js|jsx)$/.test(id) &&
        !/node_modules/.test(id) &&
        !referencedIds.has(id)
    )
    .filter((id) =>
      exclude.reduce(
        (result, pattern) => result && !minimatch(id, pattern),
        true as boolean
      )
    )
    .filter((id) =>
      include
        ? include.reduce(
            (result, pattern) => result || minimatch(id, pattern),
            false as boolean
          )
        : true
    )
    .sort()
}

export const getEntryPoints = async ({
  cwd,
  exclude,
  include,
  webpackConfigPath
}: {
  cwd: string
  exclude?: string[]
  include?: string[]
  webpackConfigPath?: string
}) => {
  const dirs = await getDirectoriesForEntryPointsSearch(cwd)

  const globs = dirs
    .map((dirName) => path.relative(cwd, dirName))
    .map((dirName) => `${dirName}/*`)

  const globsWithRoot = ['*', ...globs]
  const depsTree = await getDepsTree(cwd, globsWithRoot, webpackConfigPath)

  const possibleEntryPoints = findEntryPointsInDepsTree(
    depsTree,
    exclude,
    include
  )
  const ignoreInstance = ignore()

  let gitignore = ''

  try {
    gitignore = (await fs.readFile(path.join(cwd, '.gitignore'))).toString()
    const lines = gitignore.split('\n')
    const nonCommentedNonEmptyLines = lines
      .filter((line) => !/^(\s*)#/.test(line))
      .filter((line) => !/^(\s*)$/.test(line))

    gitignore = nonCommentedNonEmptyLines.join('\n')
  } catch (e) {
    e
  }

  ignoreInstance.add(gitignore)

  return [ignoreInstance.filter(possibleEntryPoints), depsTree] as [
    string[],
    MinimalDependencyTree
  ]
}
