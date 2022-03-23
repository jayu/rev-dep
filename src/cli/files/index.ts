import commander from 'commander'
import { InputParams } from './types'
import {
  tsConfigOption,
  TsConfigOptionType,
  webpackConfigOption,
  WebpackConfigOptionType,
  cwdOption,
  CwdOptionType,
  reexportRewireOption,
  ReexportRewireOptionType
} from '../commonOptions'

export default function createFiles(program: commander.Command) {
  program
    .command('files <entryPoint>')
    .description('Get list of files required by entry point')
    .option(...webpackConfigOption)
    .option(...tsConfigOption)
    .option(...cwdOption)
    .option(...reexportRewireOption)
    .option(
      '-c, --count',
      'print only count of entry point dependencies',
      false
    )
    .action(
      async (
        data: InputParams &
          TsConfigOptionType &
          WebpackConfigOptionType &
          CwdOptionType &
          ReexportRewireOptionType
      ) => {
        const { webpackConfig, tsConfig, cwd } = data

        console.log('files command')
      }
    )
}
