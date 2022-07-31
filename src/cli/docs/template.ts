import type { Command } from './generate'
import dedent from 'dedent'
const programName = 'rev-dep'

const header = (level: number, ...text: string[]) =>
  '#'.repeat(level) + ' ' + text.join(' ')

const code = (...text: string[]) => '`' + text.join(' ') + '`'
const codeBlock = (...text: string[]) => '```sh\n' + text.join(' ') + '\n```'
const requiredStr = '**required**'
const optionalStr = '_optional_'

const filterFalsy = (array: any[]) => array.filter((val) => val)

function template(commands: Command[], headerLevel: number) {
  return dedent(
    commands
      .map((cmd) => {
        return `
      ${header(headerLevel, 'Command', code(cmd.name))}

      ${cmd.description || 'Description not available'}

      ${header(headerLevel + 1, 'Usage')}

      ${codeBlock(
        ...filterFalsy([
          programName,
          cmd.name,
          ...cmd.arguments.map((arg) => arg.nameRaw),
          cmd.options.length > 0 ? '[options]' : undefined
        ])
      )}
      ${cmd.arguments.length > 0 ? header(headerLevel + 1, 'Arguments') : ''}

      ${cmd.arguments
        .map(
          ({ name, required, description }) => dedent`
        * ${code(name)} - ${description} (${
            required ? requiredStr : optionalStr
          })
      `
        )
        .join('\n')}
      
      ${cmd.options.length > 0 ? header(headerLevel + 1, 'Options') : ''}

      ${cmd.options
        .map(
          ({ shortName, longName, argument, required, description }) => dedent`
        * ${code(
          filterFalsy([shortName, longName]).join(', ') +
            (argument ? ` ${argument}` : '')
        )} - ${description} (${required ? requiredStr : optionalStr})
      `
        )
        .join('\n')}

    `
      })
      .join('\n')
  )
}

export default template
