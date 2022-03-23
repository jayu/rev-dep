import path from 'path'

export const removeInitialDot = (path: string) => path.replace(/^\.\//, '')

export const _resolveAbsolutePath = (cwd: string) => (p: string | undefined) =>
  typeof p === 'string' ? path.resolve(cwd, p) : p

export const asyncFilter = async <T>(
  arr: T[],
  predicate: (el: T) => Promise<boolean>
) => {
  const results = await Promise.all(arr.map(predicate))

  return arr.filter((_v, index) => results[index])
}
