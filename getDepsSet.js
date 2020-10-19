const depcruise = require('dependency-cruiser').cruise

const getDepsSet = (entryPoints, skipRegex) => {
  const skip =
    skipRegex || '(node_modules|/__tests__|/__test__|/__mockContent__|.scss)'

  const dependencies = depcruise(entryPoints, {
    exclude: skip,
    doNotFollow: { path: skip },
    tsConfig: {
      fileName: 'tsconfig.json'
    }
  }).output
  return dependencies.modules
}

module.exports = getDepsSet
