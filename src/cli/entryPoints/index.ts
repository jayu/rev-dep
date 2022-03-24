import commander from 'commander'
import { InputParams } from './types'
import {
  webpackConfigOption,
  WebpackConfigOptionType,
  cwdOption,
  CwdOptionType,
  reexportRewireOption,
  ReexportRewireOptionType
} from '../commonOptions'
import { getEntryPoints } from '../../lib/getEntryPoints'
import { buildGraphDpdm } from '../../lib/buildDepsGraph'
import { resolvePath } from '../../lib/utils'

export default function createEntryPoints(program: commander.Command) {
  program
    .command('entry-points')
    .description('Print list of entry points in current directory')
    .option(...webpackConfigOption)
    .option(...cwdOption)
    .option(...reexportRewireOption)
    .option(
      '-pdc, --printDependenciesCount',
      'print count of entry point dependencies',
      false
    )
    .action(
      async (
        data: InputParams &
          WebpackConfigOptionType &
          CwdOptionType &
          ReexportRewireOptionType
      ) => {
        const {
          webpackConfig: webpackConfigPath,
          cwd,
          printDependenciesCount
        } = data

        const [entryPoints, depsTree] = await getEntryPoints({
          cwd: resolvePath(cwd),
          webpackConfigPath
        })

        let depsCount: number[] | null = null

        if (printDependenciesCount) {
          depsCount = entryPoints
            .map(buildGraphDpdm(depsTree))
            .map(([_, __, vertices]) => vertices.size)
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
