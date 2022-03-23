import { cruise as depcruise, ICruiseResult } from 'dependency-cruiser'

// eslint-disable-next-line
const resolveWebpackConfig = require('dependency-cruiser/config-utl/extract-webpack-resolve-config')

export const getDepsSetWebpack = (
  entryPoints: string[],
  skipRegex: RegExp | undefined,
  webpackConfigPath: string
) => {
  const skip =
    skipRegex || '(node_modules|/__tests__|/__test__|/__mockContent__|.scss)'
  const webpackResolveOptions = webpackConfigPath
    ? resolveWebpackConfig(webpackConfigPath)
    : null

  const result = depcruise(
    entryPoints,
    {
      //@ts-ignore
      exclude: skip,
      //@ts-ignore
      doNotFollow: { path: skip },
      tsPreCompilationDeps: true
    },
    webpackResolveOptions
  )

  const normalized = {} as {
    [key: string]: Array<{ id: string; request: string }>
  }
  const nonResolvableDeps = [] as string[]

  ;(result.output as ICruiseResult).modules.forEach((mod) => {
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
