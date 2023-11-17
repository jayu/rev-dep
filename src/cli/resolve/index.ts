import { resolve } from '../../lib/resolve'
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
  excludeOption,
  ignoreTypesImports,
  IgnoreTypesImportsOptionType
} from '../commonOptions'

export default function createResolve(program: commander.Command) {
  program
    .command('resolve <filePathOrNodeModuleName> [entryPoints...]')
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
    .option(
      '-ntp --notTraversePaths <paths...>',
      'Specify file paths relative to resolution root, that should not be traversed when finding dependency path'
    )
    .option(
      '-inm --includeNodeModules',
      'Whether to include node modules in dependency graph. Has to be provided to resolve node module.',
      true
    )
    .option(...ignoreTypesImports)
    .action(
      async (
        filePathOrNodeModuleName: string,
        entryPoints: string[],
        data: InputParams &
          WebpackConfigOptionType &
          CwdOptionType &
          ReexportRewireOptionType &
          IncludeOptionType &
          ExcludeOptionType &
          IgnoreTypesImportsOptionType
      ) => {
        const {
          compactSummary,
          webpackConfig,
          all,
          cwd,
          exclude,
          include,
          notTraversePaths,
          ignoreTypesImports,
          includeNodeModules
        } = data

        const [results, resolveEntryPoints] = await resolve({
          entryPoints,
          filePathOrNodeModuleName,
          webpackConfig,
          all,
          cwd: resolvePath(cwd),
          exclude,
          include,
          notTraversePaths,
          ignoreTypesImports,
          includeNodeModules
        })

        const formatted = formatResults({
          results,
          entryPoints: resolveEntryPoints,
          compactSummary,
          filePathOrNodeModuleName
        })

        console.log(formatted)
      }
    )
}
