import { Dependency } from 'dpdm'
export type Node = {
  path: string
  children: Node[]
  parents: Node[]
}

export type MinimalDependency = Pick<Dependency, 'id' | 'request'>
export type MinimalDependencyTree = {
  [key: string]: readonly MinimalDependency[] | null
}
