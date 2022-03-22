
import { Command } from 'commander';
const pkg = require('../package.json')

import { find } from './find';
const program = new Command('rev-dep')
program.version(pkg.version, '-v, --version')

const pathToString = (str: string, filePath: string, indentation: number) => {
  return `${str ? `${str}\n` : ''}${' '.repeat(indentation)} ➞ ${filePath}`
}

type InputParams = {
  compactSummary?: boolean, verbose?: boolean, webpackConfig?: string, typescriptConfig?: string, maxDepth?: number, printMaxDepth?: boolean, printDependentCount?: boolean, checkOnly?: boolean
}

program
  .command('resolve <filePath> [entryPoints...]')
  .option(
    '-cs, --compactSummary',
    'print a compact summary of reverse resolution with a count of found paths'
  )
  .option('--verbose', 'print current action information')
  .option(
    '-wc, --webpackConfig <path>',
    'path to webpack config to enable webpack aliases support'
  )
  .option(
    '-tc, --typescriptConfig <path>',
    'path to TypeScript config to enable TS aliases support'
  )
  .option(
    '-md, --maxDepth <maxDepth>',
    'max depth of the dependency tree',
    '10'
  )
  .option(
    '-pmd, --printMaxDepth',
    'print max depth in the tree',
    false
  )
  .option(
    '-pdc, --printDependentCount',
    'print count of entry point dependencies',
    false
  )
  .option(
    '-co, --checkOnly',
    'finds only one path to entry point instead of all',
    false
  )
  .action(async (filePath: string, entryPoints: string[], data: InputParams) => {
    const { compactSummary, verbose, webpackConfig, typescriptConfig, maxDepth, printMaxDepth, printDependentCount, checkOnly } = data

    const results = await find({
      entryPoints,
      filePath,
      verbose,
      webpackConfig,
      typescriptConfig,
      maxDepth,
      printMaxDepth,
      printDependentCount,
      checkOnly
    })
    const hasAnyResults = results.some((paths) => paths.length > 0)
    if (!hasAnyResults) {
      console.log('No results found for', filePath, 'in', entryPoints)
      return
    }
    console.log('Results:\n')
    if (compactSummary) {
      const maxEntryLength = entryPoints.reduce((maxLength, entryPoint) => {
        return entryPoint.length > maxLength ? entryPoint.length : maxLength
      }, 0)
      let total = 0
      entryPoints.forEach((entry, index) => {
        console.log(`${entry.padEnd(maxEntryLength)} :`, results[index].length)
        total += results[index].length
      })
      console.log('\nTotal:', total)
    } else {
      results.forEach((entryPointResults, index) => {
        entryPointResults.forEach((path) => {
          console.log(path.reduce(pathToString, ''), '\n')
        })
        if (index < results.length - 1) {
          console.log('_'.repeat(process.stdout.columns))
        }
      })
    }


  })

program.parse(process.argv)
