const depcruise = require('dependency-cruiser').cruise
// eslint-disable-next-line
const resolveWebpackConfig = require('dependency-cruiser/config-utl/extract-webpack-resolve-config')
// eslint-disable-next-line
const resolveTsConfig = require('dependency-cruiser/config-utl/extract-ts-config')
const getDepsSet = (entryPoints, skipRegex, webpackConfigPath, tsConfigPath) => {
  const skip =
    skipRegex || '(node_modules|/__tests__|/__test__|/__mockContent__|.scss)'
  const webpackResolveOptions = webpackConfigPath ? resolveWebpackConfig(webpackConfigPath) : null
  const tsConfigOptions = tsConfigPath ? resolveTsConfig(tsConfigPath) : null

  const result = depcruise(entryPoints, {
    exclude: skip,
    doNotFollow: { path: skip },
    tsPreCompilationDeps: true,

  }, webpackResolveOptions, tsConfigOptions)
  return result.output.modules
}

module.exports = getDepsSet
