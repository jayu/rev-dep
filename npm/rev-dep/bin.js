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

// The child inherits our stdio: output streams straight to the terminal, pipe or file, so there is
// no size limit (execSync capped it at maxBuffer, 1 MiB, then killed the child) and nothing is held
// in this process to flush. Interactive commands (config init) get the TTY for the same reason.
// No shell either: arguments reach the binary exactly as given.
const result = cp.spawnSync(binary, binaryArgs, { stdio: 'inherit' })

// result.errr is only set when binary was not started successfully (ENOENT, EACCES, etc). 
if (result.error) {
  console.error(result.error.message)
  process.exitCode = 1
} else {
  // status is null when the child was killed by a signal; treat that as a failure.
  // exitCode, not process.exit(): exit() drops stdout/stderr writes still queued for a pipe.
  process.exitCode = result.status === null ? 1 : result.status
}
