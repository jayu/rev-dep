import commander from 'commander'
import { InputParams } from './types'
import {
  webpackConfigOption,
  WebpackConfigOptionType,
  cwdOption,
  CwdOptionType,
  reexportRewireOption,
  ReexportRewireOptionType,
  includeOption,
  IncludeOptionType,
  excludeOption,
  ExcludeOptionType,
  IgnoreTypesImportsOptionType,
  ignoreTypesImports
} from '../commonOptions'
import { getEntryPoints } from '../../lib/getEntryPoints'
import { buildDepsGraph } from '../../lib/buildDepsGraph'
import { resolvePath } from '../../lib/utils'

export default function createEntryPoints(program: commander.Command) {
  program
    .command('entry-points')
    .description('Print list of entry points in current directory')
    .option(...webpackConfigOption)
    .option(...cwdOption)
    // .option(...reexportRewireOption)
    .option(...includeOption)
    .option(...excludeOption)
    .option(
      '-pdc, --printDependenciesCount',
      'print count of entry point dependencies',
      false
    )
    .option('-c, --count', 'print just count of found entry points', false)
    .option(...ignoreTypesImports)
    .action(
      async (
        data: InputParams &
          WebpackConfigOptionType &
          CwdOptionType &
          ReexportRewireOptionType &
          IncludeOptionType &
          ExcludeOptionType &
          IgnoreTypesImportsOptionType
      ) => {
        const {
          webpackConfig: webpackConfigPath,
          cwd,
          printDependenciesCount,
          include,
          exclude,
          count,
          ignoreTypesImports
        } = data

        const [entryPoints, depsTree] = await getEntryPoints({
          cwd: resolvePath(cwd),
          webpackConfigPath,
          exclude,
          include,
          ignoreTypesImports
        })

        let depsCount: number[] | null = null

        if (printDependenciesCount) {
          depsCount = entryPoints
            .map(buildDepsGraph(depsTree))
            .map(([_, __, vertices]) => vertices.size)
        }

        if (count) {
          console.log('Found', entryPoints.length, 'entry points.')
          return
        }
        if (entryPoints.length === 0) {
          console.log('No results found')
          return
        }

        entryPoints.forEach((pathName, idx) => {
          if (depsCount !== null) {
            console.log(pathName, depsCount[idx])
          } else {
            console.log(pathName)
          }
        })
      }
    )
}
