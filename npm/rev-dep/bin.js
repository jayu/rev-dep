#!/usr/bin/env node
const fs = require('fs')
const path = require('path')
const cp = require('child_process')
const binaryArgs = process.argv.slice(2)

const binaryPackageName = `@rev-dep/${process.platform}-${process.arch}`
let packageDir = ''

const nestedNodeModulesPath = path.join(__dirname, 'node_modules', binaryPackageName)
const siblingNodeModulesPath = path.join(__dirname, '../', binaryPackageName)

const checkedPaths = []
const fsRoot = path.parse(__dirname).root

if (fs.existsSync(nestedNodeModulesPath)) {
  packageDir = nestedNodeModulesPath
}
else if (fs.existsSync(siblingNodeModulesPath)) {
  packageDir = siblingNodeModulesPath
} else {
  checkedPaths.push(nestedNodeModulesPath, siblingNodeModulesPath)
  let lookupDir = path.join(__dirname, '../../../')
  while (lookupDir != undefined && packageDir == '') {
    const pathToCheck = path.join(lookupDir, 'node_modules', binaryPackageName)
    if (fs.existsSync(pathToCheck)) {
      packageDir = pathToCheck
    }
    else {
      checkedPaths.push(pathToCheck)
      if (lookupDir === fsRoot) {
        lookupDir = undefined
      }
      else {
        lookupDir = path.join(lookupDir, '../')
      }
    }
  }
}

if (packageDir === '') {
  console.error("Could not locate rev-dep binary for your platform: ", binaryPackageName)
  console.log('Checked paths', checkedPaths)
  console.log('Please open an issue to request platform support')
  console.log('https://github.com/jayu/rev-dep/issues')
  process.exit(1)
}
const isWin = process.platform === "win32"
const binary = path.join(packageDir, 'bin', 'rev-dep' + (isWin ? '.exe' : ''))

if (!fs.existsSync(binary)) {
  console.error("Could not locate binary in package directory.")
  console.log(binary, 'does not exist')
  process.exit(1)
}

// config init needs interactive terminal
const positionalArgs = binaryArgs.filter((arg) => !arg.startsWith('-'))
const isConfigInit = positionalArgs[0] === 'config' && positionalArgs[1] === 'init'
const isInteractiveTerminal = Boolean(process.stdin.isTTY && process.stdout.isTTY)

if (isConfigInit && isInteractiveTerminal) {
  const interactive = cp.spawnSync(binary, binaryArgs, { stdio: 'inherit' })
  if (interactive.error) {
    console.error(interactive.error.message)
    process.exit(1)
  }
  // status is null when the child was killed by a signal; treat that as a failure.
  process.exit(interactive.status === null ? 1 : interactive.status)
}

try {
  const binaryArgsWrapped = binaryArgs.map((arg) => `"${arg}"`)

  const result = cp.execSync(`${binary} ${binaryArgsWrapped.join(' ')}`, { stdio: 'pipe' })

  if (Buffer.isBuffer(result)) {
    process.stdout.write(result.toString())
  }
  else {
    console.error("Unexpected binary result", result)
  }
} catch (e) {
  process.stdout.write(e.stdout)
  process.stderr.write(e.stderr)
  process.exit(e.status)
}

