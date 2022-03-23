import commander from 'commander'
import createResolve from './resolve'
import createDocs from './docs'
import createEntryPoints from './entryPoints/index'

export function createCommands(program: commander.Command) {
  createResolve(program)
  createDocs(program)
  createEntryPoints(program)
}
