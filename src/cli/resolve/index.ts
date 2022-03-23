import { find } from '../../lib/find';
import commander from 'commander'
import { InputParams } from './types';
import { formatResults } from './formatResults';

export default function createResolve(program: commander.Command) {




  program
    .command('resolve <filePath> [entryPoints...]')
    .option(
      '-cs, --compactSummary',
      'print a compact summary of reverse resolution with a count of found paths'
    )
    .option('--verbose', 'print current action information')
    .option(
      '-wc, --webpackConfig <path>',
      'path to webpack config to enable webpack aliases support'
    )
    .option(
      '-tc, --typescriptConfig <path>',
      'path to TypeScript config to enable TS aliases support'
    )
    .option(
      '-md, --maxDepth <maxDepth>',
      'max depth of the dependency tree',
      '10'
    )
    .option(
      '-pmd, --printMaxDepth',
      'print max depth in the tree',
      false
    )
    .option(
      '-pdc, --printDependentCount',
      'print count of entry point dependencies',
      false
    )
    .option(
      '-co, --checkOnly',
      'finds only one path to entry point instead of all',
      false
    )
    .action(async (filePath: string, entryPoints: string[], data: InputParams) => {
      const { compactSummary, verbose, webpackConfig, typescriptConfig, maxDepth, printMaxDepth, printDependentCount, checkOnly } = data

      const results = await find({
        entryPoints,
        filePath,
        verbose,
        webpackConfig,
        typescriptConfig,
        maxDepth,
        printMaxDepth,
        printDependentCount,
        checkOnly
      })

      const formatted = formatResults({ results, entryPoints, compactSummary, filePath });
      console.log(formatted);

    })
}