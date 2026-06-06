// Shared release-notes generator. Produces categorized Markdown from the commits
// in a git range, grouped into Features / Bug Fixes / Documentation / Other
// Changes (by conventional-commit prefix), ordered by commit date within each
// group, with a date header and per-commit links. Used by release.js.
const { execSync } = require('child_process')

const REPO_URL = 'https://github.com/jayu/rev-dep'
const UNIT = '\x1f' // unlikely-in-text field separator

const sh = (cmd) => execSync(cmd, { encoding: 'utf8' }).trim()

// Map a conventional-commit type to a section bucket.
const bucketFor = (type) => {
  switch (type) {
    case 'feat':
      return 'feat'
    case 'fix':
      return 'fix'
    case 'docs':
      return 'docs'
    default:
      return 'other'
  }
}

// Returns the YYYY-MM-DD committer date of a ref.
function dateOf(ref) {
  return sh(`git show -s --format=%cs ${ref}`)
}

// range: e.g. "2.1.0..<hash>" or a single ref for the very first release.
// version, date: head the notes ("## <version> (<date>)").
// previousVersion: when set, the version in the header links to the GitHub
// compare view (previousVersion...version), like "Full Changelog: x...y".
function generateReleaseNotes({ range, version, date, previousVersion }) {
  const raw = sh(`git log ${range} --no-merges --format=%H${UNIT}%cI${UNIT}%s`)
  const commits = raw
    ? raw.split('\n').map((line) => {
        const [hash, iso, subject] = line.split(UNIT)
        return { hash, iso, subject }
      })
    : []

  const groups = { feat: [], fix: [], docs: [], other: [] }
  for (const c of commits) {
    // Skip the release / version-bump commits themselves.
    if (/^chore[:(]?.*\b(release|publish)\b/i.test(c.subject)) continue

    const m = c.subject.match(/^(\w+)(?:\(([^)]*)\))?!?:\s*(.*)$/)
    let bucket = 'other'
    let text = c.subject
    if (m) {
      bucket = bucketFor(m[1].toLowerCase())
      const scope = m[2]
      const desc = m[3]
      text = scope ? `${scope}: ${desc}` : desc
    }
    groups[bucket].push({ ...c, text })
  }

  // Order by commit date (ascending) within each group.
  for (const key of Object.keys(groups)) {
    groups[key].sort((a, b) => a.iso.localeCompare(b.iso))
  }

  const heading = previousVersion
    ? `## [${version}](${REPO_URL}/compare/${previousVersion}...${version}) (${date})`
    : `## ${version} (${date})`
  const lines = [heading, '']
  const section = (title, items) => {
    if (!items.length) return
    lines.push(`### ${title}`, '')
    for (const it of items) {
      const short = it.hash.slice(0, 7)
      lines.push(`- ${it.text} ([${short}](${REPO_URL}/commit/${it.hash}))`)
    }
    lines.push('')
  }
  section('Features', groups.feat)
  section('Bug Fixes', groups.fix)
  section('Documentation', groups.docs)
  section('Other Changes', groups.other)

  if (!groups.feat.length && !groups.fix.length && !groups.docs.length && !groups.other.length) {
    lines.push('_No notable changes._', '')
  }

  return lines.join('\n').trim() + '\n'
}

module.exports = { generateReleaseNotes, dateOf, REPO_URL }
