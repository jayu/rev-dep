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

export default function createEntryPoints(program: commander.Command) {
  program
    .command('entry-points')
    .description('Print list of entry points in current directory')
    .option(...webpackConfigOption)
    .option(...tsConfigOption)
    .option(...cwdOption)
    .option(...reexportRewireOption)
    .option(
      '-pdc, --printDependentCount',
      'print count of entry point dependencies',
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

        console.log('entry points command')
      }
    )
}
