import { cruise as depcruise, ICruiseResult } from 'dependency-cruiser'
import { createResolveAbsolutePath } from './utils'

// eslint-disable-next-line
const resolveWebpackConfig = require('dependency-cruiser/config-utl/extract-webpack-resolve-config')

const normalizeDepsTree = (modules: ICruiseResult['modules']) => {
  const normalized = {} as {
    [key: string]: Array<{ id: string; request: string }>
  }
  const nonResolvableDeps = [] as string[]

  modules.forEach((mod) => {
    const { source, dependencies } = mod
    if (!nonResolvableDeps.includes(source)) {
      normalized[source] = dependencies
        .filter(({ couldNotResolve, resolved: id }) => {
          if (couldNotResolve) {
            nonResolvableDeps.push(id)
          }
          return !couldNotResolve
        })
        .map(({ resolved, module }) => ({
          id: resolved,
          request: module
        }))
    }
  })

  return normalized
}

export const getDepsSetWebpack = (
  entryPoints: string[],
  webpackConfigPath: string,
  cwd: string,
  skipRegex?: RegExp
) => {
  const skip =
    skipRegex || '(node_modules|/__tests__|/__test__|/__mockContent__|.scss)'
  const webpackResolveOptions = webpackConfigPath
    ? resolveWebpackConfig(createResolveAbsolutePath(cwd)(webpackConfigPath))
    : null

  const result = depcruise(
    entryPoints,
    {
      //@ts-ignore
      exclude: skip,
      //@ts-ignore
      doNotFollow: { path: skip },
      tsPreCompilationDeps: true,
      baseDir: cwd
    },
    webpackResolveOptions
  )

  return normalizeDepsTree((result.output as ICruiseResult).modules)
}
