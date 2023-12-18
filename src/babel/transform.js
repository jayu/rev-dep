/*eslint-disable @typescript-eslint/no-var-requires */
const { getFilesList } = require('@codeque/core')
const babelCore = require('@babel/core')
const parser = require('@babel/parser')
const fs = require('fs')
const path = require('path')

import { babelParsingOptions } from './babelParsingOptions'

import { processTextCodeModificationsArray } from './processCodeTextModificationsArray'

export const transform = async ({
  rootPath,
  inputFilePath,
  includeBarrelExportFiles,
  excludeBarrelExportFiles
}) => {
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
  let cache = new Map()

  for (const filePath of filesList) {
    try {
      const fileName = path.parse(filePath).name
      const fileContent = fs.readFileSync(filePath).toString()

      const result = babelCore.transformFileSync(filePath, {
        plugins: [
          [
            __dirname + '/index.js',
            {
              tsConfigPath: path.join(root, 'tsconfig.json'),
              cache,
              includeBarrelExportFiles,
              excludeBarrelExportFiles
            }
          ]
        ],
        parserOpts: babelParsingOptions,
        filename: fileName
      })

      const changes = result.metadata[filePath]

      if (changes?.length > 0) {
        const resultCode = processTextCodeModificationsArray({
          code: fileContent,
          changes
        })

        fs.writeFileSync(filePath, resultCode)
      }

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
}
