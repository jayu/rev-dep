import { resolve } from '../../lib/find'
import commander from 'commander'
import { InputParams } from './types'
import { formatResults } from './formatResults'
import { sanitizeUserEntryPoints, resolvePath } from '../../lib/utils'
import {
  webpackConfigOption,
  cwdOption,
  reexportRewireOption,
  WebpackConfigOptionType,
  CwdOptionType,
  ReexportRewireOptionType
} from '../commonOptions'

export default function createResolve(program: commander.Command) {
  program
    .command('resolve <filePath> [entryPoints...]')
    .description(
      'Checks if a filePath is required from entryPoint(s) and prints the resolution path'
    )
    .option(...webpackConfigOption)
    .option(...cwdOption)
    .option(...reexportRewireOption)
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
          ReexportRewireOptionType
      ) => {
        const { compactSummary, webpackConfig, all, cwd } = data

        const sanitizedEntryPoints = sanitizeUserEntryPoints(entryPoints)

        const [results, resolveEntryPoints] = await resolve({
          entryPoints: sanitizedEntryPoints,
          filePath,
          webpackConfig,
          all,
          cwd: resolvePath(cwd)
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
