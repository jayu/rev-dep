const cp = require('child_process')
const fs = require('fs')
const prettier = require('prettier')

const fileName = 'temp.md'
const readme = 'README.md'

const startPhrase = 'cli-docs-start'
const endPhrase = 'cli-docs-end'

cp.spawnSync('yarn', ['dev', 'docs', fileName])

const docsLines = fs.readFileSync(fileName).toString().split('\n')

const readmeLines = fs.readFileSync(readme).toString().split('\n')

const startIndex = readmeLines.findIndex((line) => line.includes(startPhrase))
const endIndex = readmeLines.findIndex((line) => line.includes(endPhrase))

const preDocs = readmeLines.slice(0, startIndex + 1)
const postDocs = readmeLines.slice(endIndex)

const newReadme = prettier.format(
  [...preDocs, ...docsLines, ...postDocs].join('\n'),
  {
    parser: 'markdown'
  }
)

fs.writeFileSync(readme, newReadme)

fs.unlinkSync(fileName)
