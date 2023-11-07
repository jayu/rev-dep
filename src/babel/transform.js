/*eslint-disable @typescript-eslint/no-var-requires */
const { getFilesList } = require('@codeque/core')
const babelCore = require('@babel/core')
const fs = require('fs')
const path = require('path')
const rootPath = process.argv[2]
const inputFilePath = process.argv[3]

if (!rootPath) {
  console.error('Please provide correct transformation root')
  process.exit(1)
}

;(async () => {
  const root = path.resolve(rootPath)
  const resolvedInputFilePath = inputFilePath
    ? path.join(root, inputFilePath)
    : undefined
  console.log('root', root)
  const filesList = resolvedInputFilePath
    ? [path.resolve(resolvedInputFilePath)]
    : await getFilesList({
        searchRoot: root,
        extensionTester: /\.(ts|tsx)$/
      })
  const errors = []
  let progressCount = 0

  for (const filePath of filesList) {
    try {
      const fileName = path.parse(filePath).name

      const result = babelCore.transformFileSync(filePath, {
        plugins: [
          ['./babel.js', { tsConfigPath: path.join(root, 'tsconfig.json') }]
        ],
        parserOpts: {
          plugins: ['typescript', 'jsx']
        },
        filename: fileName
      })

      fs.writeFileSync(filePath, result.code)
      progressCount++
      if (progressCount % 100 === 0) {
        console.log(`${progressCount}+${errors.length}/${filesList.length}`)
      }
    } catch (e) {
      errors.push(e)
    }
  }

  console.log(errors)
  console.log(
    `Done: ${progressCount}/${filesList.length}; Failed: ${errors.length}`
  )
})()
