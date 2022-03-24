import path from 'path'
import escapeGlob from 'glob-escape'

export const removeInitialDot = (path: string) => path.replace(/^\.\//, '')

export const createResolveAbsolutePath = (cwd: string) => (
  p: string | undefined
) => (typeof p === 'string' ? path.resolve(cwd, p) : p)

export const asyncFilter = async <T>(
  arr: T[],
  predicate: (el: T) => Promise<boolean>
) => {
  const results = await Promise.all(arr.map(predicate))

  return arr.filter((_v, index) => results[index])
}

export const sanitizeUserEntryPoints = (entryPoints: string[]) => {
  const globEscapedEntryPoints = entryPoints.map(escapeGlob)
  return globEscapedEntryPoints
}

export const resolvePath = <P extends string | undefined>(p: P) => {
  if (!p || path.isAbsolute(p)) {
    return p
  }

  return path.resolve(p)
}
