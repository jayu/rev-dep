import { DependencyTree } from 'dpdm'
import { MinimalDependencyTree } from './types'
import isBuiltinModule from 'is-builtin-module'

export const cleanupDpdmDeps = (
  deps: MinimalDependencyTree | DependencyTree,
  includeNodeModules = false
) => {
  const newDeps = {} as MinimalDependencyTree

  Object.entries(deps).forEach(([id, dependencies]) => {
    const nodeModules: string[] = []

    if (
      !isBuiltinModule(id) &&
      !id.includes('node_modules') &&
      dependencies !== null
    ) {
      newDeps[id] = dependencies
        .filter(
          ({ id }) =>
            id &&
            (includeNodeModules || !id.includes('node_modules')) &&
            !isBuiltinModule(id)
        )
        .map(({ id, request }) => {
          const shouldAddNodeModule =
            includeNodeModules && id?.includes('node_modules')

          const idToAdd = shouldAddNodeModule ? request : id

          if (shouldAddNodeModule && idToAdd) {
            nodeModules.push(idToAdd)
          }
          return {
            id: idToAdd,
            request
          }
        })
    }

    if (includeNodeModules) {
      nodeModules.forEach((nodeModuleName) => {
        newDeps[nodeModuleName] = []
      })
    }
  })

  return newDeps
}
