#!/usr/bin/env node
// Create a GitHub release for the current version: pushes the tag, attaches the
// three platform binaries and a sha256 checksums.txt, and auto-generates notes
// from the commits since the previous release.
//
// Run AFTER scripts/buildProdBinaries.sh and the "chore: release X.Y.Z" commit.
// Requires: gh (authenticated), git, node.
const fs = require('fs')
const os = require('os')
const path = require('path')
const crypto = require('crypto')
const { execFileSync } = require('child_process')
const { generateReleaseNotes, dateOf } = require('./releaseNotes')

const root = path.join(__dirname, '..')
process.chdir(root)

// Run a command, streaming its output; throws on non-zero exit (like `set -e`).
const run = (cmd, args) => execFileSync(cmd, args, { stdio: 'inherit' })
// Run a command and capture stdout; returns null instead of throwing.
const tryCapture = (cmd, args) => {
  try {
    return execFileSync(cmd, args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim()
  } catch {
    return null
  }
}

const version = require(path.join(root, 'npm/rev-dep/package.json')).version
if (!version) {
  console.error('Could not read version from npm/rev-dep/package.json')
  process.exit(1)
}
console.log(`Preparing GitHub release ${version}`)

// Built binary -> stable, platform-identifiable asset name.
const binaries = [
  { src: 'npm/@rev-dep/darwin-arm64/bin/rev-dep', asset: 'rev-dep-darwin-arm64' },
  { src: 'npm/@rev-dep/linux-x64/bin/rev-dep', asset: 'rev-dep-linux-x64' },
  { src: 'npm/@rev-dep/win32-x64/bin/rev-dep.exe', asset: 'rev-dep-win32-x64.exe' },
]

const dist = fs.mkdtempSync(path.join(os.tmpdir(), 'rev-dep-release-'))
try {
  // Copy binaries and build a `shasum -a 256 -c`-compatible checksums.txt
  // ("<hex>  <name>"), so users can verify a downloaded binary.
  const checksumLines = []
  for (const { src, asset } of binaries) {
    if (!fs.existsSync(src)) {
      console.error(`Missing binary ${src} - run scripts/buildProdBinaries.sh first`)
      process.exit(1)
    }
    const destPath = path.join(dist, asset)
    fs.copyFileSync(src, destPath)
    const hash = crypto.createHash('sha256').update(fs.readFileSync(destPath)).digest('hex')
    checksumLines.push(`${hash}  ${asset}`)
  }
  const checksums = checksumLines.join('\n') + '\n'
  fs.writeFileSync(path.join(dist, 'checksums.txt'), checksums)
  console.log('checksums.txt:')
  process.stdout.write(checksums)

  // Categorized notes from commits since the previous release tag. Compute the
  // previous tag before creating this one.
  const prevTag = tryCapture('git', ['describe', '--tags', '--abbrev=0', 'HEAD^'])
  const range = prevTag ? `${prevTag}..HEAD` : 'HEAD'
  const notes = generateReleaseNotes({ range, version, date: dateOf('HEAD'), previousVersion: prevTag || undefined })
  const notesFile = path.join(dist, 'release-notes.md')
  fs.writeFileSync(notesFile, notes)
  console.log('\nrelease notes:\n')
  process.stdout.write(notes)

  // Tag the release commit and push the tag.
  if (tryCapture('git', ['rev-parse', version]) === null) {
    run('git', ['tag', version])
  }
  run('git', ['push', 'origin', `refs/tags/${version}`])

  // Mark pre-release for versions with a suffix (e.g. 3.0.0-beta.1).
  const isPrerelease = version.includes('-')

  const assetPaths = binaries.map(({ asset }) => path.join(dist, asset))
  assetPaths.push(path.join(dist, 'checksums.txt'))

  const ghArgs = ['release', 'create', version, '--title', version, '--notes-file', notesFile, '--verify-tag']
  if (isPrerelease) ghArgs.push('--prerelease')
  ghArgs.push(...assetPaths)
  run('gh', ghArgs)

  console.log(`Released https://github.com/jayu/rev-dep/releases/tag/${version}`)
} finally {
  fs.rmSync(dist, { recursive: true, force: true })
}
