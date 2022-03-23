import { resolve } from '../../lib/find'
import commander from 'commander'
import { InputParams } from './types'
import { formatResults } from './formatResults'
import { sanitizeUserEntryPoints } from '../../lib/utils'

export default function createResolve(program: commander.Command) {
  program
    .command('resolve <filePath> [entryPoints...]')
    .description(
      'Checks if a filePath is required from entryPoint(s) and prints the resolution path'
    )
    .option(
      '-cs, --compactSummary',
      'print a compact summary of reverse resolution with a count of found paths'
    )
    .option(
      '-wc, --webpackConfig <path>',
      'path to webpack config to enable webpack aliases support'
    )
    .option('-pmd, --printMaxDepth', 'print max depth in the tree', false)
    .option(
      '-a, --all',
      'finds all paths combination of a given dependency. Might work very slow and tend to crash for some projects',
      false
    )
    .action(
      async (filePath: string, entryPoints: string[], data: InputParams) => {
        const { compactSummary, webpackConfig, printMaxDepth, all } = data

        const sanitizedEntryPoints = sanitizeUserEntryPoints(entryPoints)

        const results = await resolve({
          entryPoints: sanitizedEntryPoints,
          filePath,
          webpackConfig,
          printMaxDepth,
          all
        })

        const formatted = formatResults({
          results,
          entryPoints,
          compactSummary,
          filePath
        })

        console.log(formatted)
      }
    )
}
