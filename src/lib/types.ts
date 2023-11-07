export type Node = {
  path: string
  children: Node[]
  parents: Node[]
}

export type MinimalDependency = { id: string | null; request: string }
export type MinimalDependencyTree = {
  [key: string]: readonly MinimalDependency[] | null
}
