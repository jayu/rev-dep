const depcruise = require('dependency-cruiser').cruise
const resolveWebpackConfig = require('dependency-cruiser/src/config-utl/extract-webpack-resolve-config')
const getDepsSet = (entryPoints, skipRegex, webpackConfigPath) => {
  const skip =
    skipRegex || '(node_modules|/__tests__|/__test__|/__mockContent__|.scss)'
  const webpackResolveOptions = webpackConfigPath ? resolveWebpackConfig(webpackConfigPath) : null
  const result = depcruise(entryPoints, {
    exclude: skip,
    doNotFollow: { path: skip },
  }, webpackResolveOptions)
  return result.output.modules
}

module.exports = getDepsSet
