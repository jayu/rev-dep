import commander from 'commander'
import generate from './generate'

type Options = {
  headerLevel: string
}

function create(program: commander.Command) {
  program
    .command('docs <outputPath>')
    .description('Generate documentation of available commands into md file.', {
      outputPath: 'path to output *.md file'
    })
    .option('-hl, --headerLevel <value>', 'Initial header level', '3')
    .action((outputPath: string, options: Options) => {
      generate(outputPath, parseInt(options.headerLevel))
    })
}

export default create
