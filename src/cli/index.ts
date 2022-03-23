import { Command } from 'commander'
//eslint-disable-next-line
const pkg = require('../../package.json')

import { createCommands } from './createCommands'

const program = new Command('rev-dep')

program.version(pkg.version, '-v, --version')

createCommands(program)

program.parse(process.argv)
