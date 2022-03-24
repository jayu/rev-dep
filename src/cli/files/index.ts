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
import { sanitizeUserEntryPoints, resolvePath } from '../../lib/utils'
import { getDepsTree } from '../../lib/getDepsTree'

export default function createFiles(program: commander.Command) {
  program
    .command('files <entryPoint>')
    .description('Get list of files required by entry point')
    .option(...webpackConfigOption)
    .option(...cwdOption)
    .option(...reexportRewireOption)
    .option(
      '-c, --count',
      'print only count of entry point dependencies',
      false
    )
    .action(
      async (
        entryPoint: string,
        data: InputParams &
          WebpackConfigOptionType &
          CwdOptionType &
          ReexportRewireOptionType
      ) => {
        const { webpackConfig: webpackConfigPath, cwd, count } = data

        const sanitizedEntryPoints = sanitizeUserEntryPoints([entryPoint])

        const depsTree = await getDepsTree(
          resolvePath(cwd),
          sanitizedEntryPoints,
          webpackConfigPath
        )

        const filePaths = Object.keys(depsTree)

        if (filePaths.length === 0) {
          console.log('No results found')
          return
        }

        if (count) {
          console.log(filePaths.length)
        } else {
          filePaths.forEach((filePath) => console.log(filePath))
        }
      }
    )
}
