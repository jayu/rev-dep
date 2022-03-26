import { resolve } from '../../lib/find'
import commander from 'commander'
import { InputParams } from './types'
import { formatResults } from './formatResults'
import { resolvePath } from '../../lib/utils'
import {
  webpackConfigOption,
  cwdOption,
  reexportRewireOption,
  WebpackConfigOptionType,
  CwdOptionType,
  ReexportRewireOptionType,
  IncludeOptionType,
  ExcludeOptionType,
  includeOption,
  excludeOption
} from '../commonOptions'

export default function createResolve(program: commander.Command) {
  program
    .command('resolve <filePath> [entryPoints...]')
    .description(
      'Checks if a filePath is required from entryPoint(s) and prints the resolution path',
      {
        filePath: 'Path to a file that should be resolved in entry points',
        'entryPoints...': 'List of entry points to look for file'
      }
    )
    .option(...webpackConfigOption)
    .option(...cwdOption)
    // .option(...reexportRewireOption)
    .option(...includeOption)
    .option(...excludeOption)
    .option(
      '-cs, --compactSummary',
      'print a compact summary of reverse resolution with a count of found paths'
    )
    .option(
      '-a, --all',
      'finds all paths combination of a given dependency. Might work very slow or crash for some projects due to heavy usage of RAM',
      false
    )
    .action(
      async (
        filePath: string,
        entryPoints: string[],
        data: InputParams &
          WebpackConfigOptionType &
          CwdOptionType &
          ReexportRewireOptionType &
          IncludeOptionType &
          ExcludeOptionType
      ) => {
        const {
          compactSummary,
          webpackConfig,
          all,
          cwd,
          exclude,
          include
        } = data

        const [results, resolveEntryPoints] = await resolve({
          entryPoints,
          filePath,
          webpackConfig,
          all,
          cwd: resolvePath(cwd),
          exclude,
          include
        })

        const formatted = formatResults({
          results,
          entryPoints: resolveEntryPoints,
          compactSummary,
          filePath
        })

        console.log(formatted)
      }
    )
}
