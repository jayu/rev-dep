import { MinimalDependencyTree } from './types'
import minimatch from 'minimatch'
import path from 'path'
import { parseDependencyTree } from 'dpdm'
import fs from 'fs/promises'
import { asyncFilter } from './utils'

export const getDirectoriesForEntryPoints = async (
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
      await Promise.all(joinedWithDir.map(getDirectoriesForEntryPoints))
    ).flat(1)
  ]
}

export const findEntryPoints = (
  deps: MinimalDependencyTree,
  exclude: string[] = []
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
}

/**
 * TODO
 * - support cruiser conditionally
 * - reuse already scanned deps
 */
export const getTreeForEntryPointsSearch = async (cwd: string) => {
  const dirs = await getDirectoriesForEntryPoints(cwd)

  const globs = dirs
    .map((dirName) => path.relative(cwd, dirName))
    .map((dirName) => `${dirName}/*`)

  const possibleEntryPoints = findEntryPoints(
    await parseDependencyTree(['*', ...globs], {
      context: cwd
    }),
    [
      '**/*stories*',
      '**stories**',
      '**/*test*',
      '**/pages/**',
      '**/api/**',
      'cypress/**',
      '**/*config.*'
    ]
  )
  console.log('possibleEntryPoints', possibleEntryPoints)
  console.log('possibleEntryPoints.length', possibleEntryPoints.length)
  return possibleEntryPoints
}
