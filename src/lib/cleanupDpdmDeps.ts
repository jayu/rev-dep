import { DependencyTree } from 'dpdm'
import { MinimalDependencyTree } from './types'
import isBuiltinModule from 'is-builtin-module'
export const cleanupDpdmDeps = (
  deps: MinimalDependencyTree | DependencyTree
) => {
  const newDeps = {} as MinimalDependencyTree

  Object.entries(deps).forEach(([id, dependencies]) => {
    if (
      !isBuiltinModule(id) &&
      !id.includes('node_modules') &&
      dependencies !== null
    ) {
      newDeps[id] = dependencies
        .filter(({ id }) => id && !id.includes('node_modules'))
        .map(({ id, request }) => ({
          id,
          request
        }))
    }
  })

  return newDeps
}
