export const babelParsingOptions = {
  errorRecovery: true,
  sourceType: 'module',
  plugins: [
    'jsx',
    'typescript',
    'objectRestSpread',
    'classProperties',
    'asyncGenerators',
    'decorators-legacy'
  ]
}
