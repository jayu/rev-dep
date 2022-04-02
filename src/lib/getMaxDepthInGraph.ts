import { Node } from './types'

type MaxDepthMeta = [number, string[]]

export const getMaxDepth = (
  depth = 1,
  path: string[] = [],
  vertices = new Map()
) => {
  return (tree: Node): MaxDepthMeta => {
    const depthFromCache = vertices.get(tree.path)

    if (depthFromCache) {
      return depthFromCache
    }

    const newPath = [...path, tree.path]

    if (tree.children.length === 0) {
      return [depth, newPath]
    }

    const results = tree.children.map(getMaxDepth(depth + 1, newPath, vertices))

    const maxChildDepth = Math.max(...results.map(([depth]) => depth))

    const itemWithMaxDepth = results.find(
      ([depth]) => depth === maxChildDepth
    ) as MaxDepthMeta

    vertices.set(tree.path, itemWithMaxDepth)

    return itemWithMaxDepth
  }
}
