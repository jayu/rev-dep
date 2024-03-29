import commander from 'commander'
import { InputParams } from './types'
import {
  webpackConfigOption,
  WebpackConfigOptionType,
  cwdOption,
  CwdOptionType,
  reexportRewireOption,
  ReexportRewireOptionType,
  ignoreTypesImports,
  IgnoreTypesImportsOptionType
} from '../commonOptions'

import { getFilesForEntryPoint } from '../../lib/getFilesForEntryPoint'

export default function createFiles(program: commander.Command) {
  program
    .command('files <entryPoint>')
    .description('Get list of files required by entry point', {
      entryPoint: 'Path to entry point'
    })
    .option(...webpackConfigOption)
    .option(...cwdOption)
    // .option(...reexportRewireOption)
    .option(
      '-c, --count',
      'print only count of entry point dependencies',
      false
    )
    .option(...ignoreTypesImports)
    .action(
      async (
        entryPoint: string,
        data: InputParams &
          WebpackConfigOptionType &
          CwdOptionType &
          ReexportRewireOptionType &
          IgnoreTypesImportsOptionType
      ) => {
        const {
          webpackConfig: webpackConfigPath,
          cwd,
          count,
          ignoreTypesImports
        } = data

        const filePaths = await getFilesForEntryPoint({
          cwd,
          entryPoint,
          webpackConfigPath,
          ignoreTypesImports
        })

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
