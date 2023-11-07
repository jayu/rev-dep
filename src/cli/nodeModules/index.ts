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
import { sanitizeUserEntryPoints, resolvePath } from '../../lib/utils'
import { getDepsTree } from '../../lib/getDepsTree'
import { getNodeModulesForEntryPoint } from '../../lib/getNodeModulesForEntryPoint'

export default function createNodeModules(program: commander.Command) {
  program
    .command('node-modules <entryPoint>')
    .description('Get list of node modules required by entry point', {
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

        const uniqueNodeModuleImports = await getNodeModulesForEntryPoint({
          cwd,
          entryPoint,
          webpackConfigPath,
          ignoreTypesImports
        })

        if (uniqueNodeModuleImports.length === 0) {
          console.log('No results found')
          return
        }

        if (count) {
          console.log(uniqueNodeModuleImports.length)
        } else {
          uniqueNodeModuleImports.forEach((nodeModuleName) =>
            console.log(nodeModuleName)
          )
        }
      }
    )
}
