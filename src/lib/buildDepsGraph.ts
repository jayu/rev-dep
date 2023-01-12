import { Node, MinimalDependencyTree } from './types'
import minimatch from 'minimatch'

export const buildDepsGraph = (
  deps: MinimalDependencyTree,
  filePath?: string,
  notTraversePath?: Array<string>
) => (entryPoint: string) => {
  const vertices = new Map()
  let fileNode: Node | null = null

  const inner = (
    path: string,
    visited = new Set(),
    depth = 1,
    parent: Node | null = null
  ): Node => {
    const vertex = vertices.get(path)

    if (vertex) {
      vertex.parents.push(parent)

      return vertex
    }

    const localVisited = new Set(visited)

    if (localVisited.has(path)) {
      // console.error('CIRCULAR DEP', ...localVisited.values(), path)

      return {
        path: 'CIRCULAR',
        parents: parent ? [parent] : [],
        children: []
      }
    }

    localVisited.add(path)

    const dep = deps[path]
    if (dep === undefined) {
      throw new Error(`Dependency '${path}' not found!`)
    }

    const node = {
      parents: parent ? [parent] : [],
      path
    } as Node

    node.children = (dep || [])
      .map((d) => d.id)
      .filter(
        (path) =>
          path !== null &&
          !path.includes('node_modules') &&
          !notTraversePath?.some((pathToNotTraverse) =>
            minimatch(path, pathToNotTraverse)
          )
      )
      .map((path) => inner(path as string, localVisited, depth + 1, node))

    vertices.set(path, node)

    if (path === filePath) {
      fileNode = node
    }

    return node
  }
  return [inner(entryPoint), fileNode, vertices] as [
    Node,
    Node | null,
    Map<string, Node>
  ]
}
