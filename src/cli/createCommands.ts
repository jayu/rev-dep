import commander from 'commander'
import createResolve from './resolve'
import createDocs from './docs'
import createEntryPoints from './entryPoints'
import createFiles from './files'

export function createCommands(program: commander.Command) {
  createResolve(program)
  createEntryPoints(program)
  createFiles(program)
  createDocs(program)
}
