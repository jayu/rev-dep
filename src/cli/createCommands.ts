import commander from 'commander';
import createResolve from './resolve';
import createDocs from './docs';

export function createCommands(program: commander.Command) {
  createResolve(program)
  createDocs(program)
}