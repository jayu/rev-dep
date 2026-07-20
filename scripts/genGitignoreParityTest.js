// Generates internal/glob/gitignore_parity_test.go.
//
// Expectations are produced by running the real `git check-ignore` over a fixed file tree -
// git is used as an oracle, so no test data is copied from git's own GPL-licensed suite.
// Add patterns/paths below and re-run: node scripts/genGitignoreParityTest.js

const fs = require('fs')
const os = require('os')
const path = require('path')
const { execFileSync } = require('child_process')

const PATHS = [
  'readme.md', 'server.log', 'a.ts', 'a.tsx', 'a.d.ts', 'useThing.ts', 'Button.stories.tsx',
  'foo.test.ts', 'index.ts', '.hidden.ts', 'file-name.ts', 'boot',
  'logs/error.log', 'logs/deep/error.log', 'build-out/a.js', 'node_modules/react/index.js',
  'src/index.ts', 'src/a.ts', 'src/a.tsx', 'src/useThing.ts', 'src/Button.stories.tsx',
  'src/foo.test.ts', 'src/a.d.ts', 'src/cache-x/f.ts', 'src/vendor/lib.ts',
  'src/nested/index.ts', 'src/nested/deep/index.ts', 'src/nested/deep/a.ts',
  'src/pages/index.tsx', 'src/pages/admin/dash.tsx',
  'src/useSearch/common.ts', 'src/__tests__/a.ts', 'src/components/Button.ts',
  'a/c.ts', 'a/b/c.ts', 'a/x/b/c.ts', 'a/x/y/b/z/c.ts',
  'mm/b', 'mm/x/b', 'mm/x/y/b',
  'dist/app.js', 'dist/manifest.json', 'dist/public/app.js',
  'deep/nested/x.log', 'deep/nested/c.ts', 'pkg/node_modules/x/i.js',
  'docs/readme.md', 'sub/boot', 'sub/dist/app.js',
  'src/!bang.ts', 'logs/important/b.log', 'build/x.js', 'build/keep/y.js',
]

// Single-pattern cases.
const PATTERNS = [
  // literal names
  'readme.md', 'dist', 'boot', '/boot', 'node_modules', 'index.ts',
  // no-slash wildcard - matched against every path segment, at any depth
  '*.log', '*.ts', '*.tsx', 'use*.ts', 'log*', '*-out', 'cache-*', '*.d.ts', '*.test.ts',
  '?.ts', '[ab].ts', '[!a]*.ts', 'a.*',
  // containing a slash - anchored to the pattern root, '*' stays within one segment
  'src/*.ts', 'src/*', 'src/a.ts', 'mm/*/b', 'src/*/index.ts',
  // leading slash - anchored at the root only
  '/src/a.ts', '/dist', '/*.ts',
  // trailing slash - directories only
  'dist/', 'logs/', 'node_modules/', 'src/vendor/',
  // double star
  '**/*.ts', '**/*.log', '**/index.ts', '**/use*.ts', '**/__tests__/**', '**/node_modules/**',
  'src/**', 'src/**/*.ts', 'src/**/*', 'src/**/index.ts', 'src/**/*.ts*', 'dist/**',
  'mm/**/b', 'a/**/b/**/c.ts', 'src/pages/**/*.ts*', '**/*.stories.tsx', 'src/**/*.stories.tsx',
  // character class
  '**/*.[jt]s',
  // escaping
  'a\\.ts', '**/a?ts',
  // deeper combinations
  'src/nested/**', 'src/nested/**/*.ts', '**/deep/**', 'src/**/deep/**',
]

// Multi-pattern cases. gitignore is LAST-MATCH-WINS, so ordering is significant and is
// exercised deliberately here.
const MULTI = [
  [['dist/**', '!dist/manifest.json'], 'negation re-includes a file'],
  [['src/**', '!src/vendor/**'], 'negation cannot re-include under an excluded directory'],
  [['*.ts', '!src/a.ts'], 'negation against a no-slash wildcard'],
  [['logs/', '!logs/deep/error.log'], 'negation under a directory pattern'],
  [['**/*.ts', '!**/*.d.ts'], 'negation with double star'],
  [['/dist/**', '!/dist/public/**'], 'anchored negation under an excluded directory'],
  // order matters - the same two patterns in both orders
  [['!src/a.ts', '*.ts'], 'later positive overrides an earlier negation'],
  [['*.ts', '!src/a.ts', 'src/a.ts'], 'later positive overrides an earlier negation again'],
  [['!**/*.d.ts', '**/*.ts'], 'later double-star positive overrides earlier negation'],
  [['dist/**', '!dist/**'], 'trailing negation cancels the whole positive'],
  [['a/**', 'b/**', '!a/skip/**'], 'negation applies only to its own subtree'],
  // --- negation edge cases, all verified against git ---
  [['*.log', '!server.log', '*.log'], 'a later positive re-excludes what a negation re-included'],
  [['*.log', '!*.log', '*.log'], 'alternating positive/negation, last one wins'],
  [['**', '!*.ts'], 'negation only reaches paths whose parent directory is not excluded'],
  [['**', '!src/**'], 'negation cannot re-include anything under an excluded tree'],
  [['src/**', '!src/*.ts'], 'partial re-inclusion: direct children only, nested stay excluded'],
  [['**/*.ts', '!**/*.d.ts', '!src/vendor/**'], 'several negations applying to different subsets'],
  [['src/*', '!src/vendor'], 're-including a directory re-includes its contents'],
  [['dist/**', '!dist/public'], 're-including a directory does NOT help when contents match directly'],
  [['logs/**', '!logs/important/**'], 'negation under a directory excluded by /**'],
  [['docs/**', '!docs/**'], 'a trailing negation cancels the whole positive'],
  [['!*.log'], 'a set of only negations matches nothing'],
  [['\\!bang.ts'], 'escaped leading bang is a literal filename, not a negation'],
]

// Workspace scoping has no direct .gitignore syntax, but a .gitignore placed IN a
// subdirectory is exactly the same idea: its patterns are rooted at that directory and
// never reach a sibling. So a nested .gitignore is the oracle for patterns whose
// patternRoot is a workspace rather than the repo root.
const SCOPED_ROOT = 'apps/web'
const SCOPED_PATHS = [
  'apps/web/api/f.ts', 'apps/web/src/a.ts', 'apps/web/src/deep/b.ts', 'apps/web/x.test.ts',
  'apps/web/node_modules/p/i.js', 'apps/web/dist/out.js',
  'apps/mobile/api/f.ts', 'apps/mobile/src/a.ts', 'apps/mobile/src/deep/b.ts',
  'apps/mobile/x.test.ts', 'apps/mobile/node_modules/p/i.js',
  'packages/ui/api/f.ts', 'root.ts',
]
const SCOPED_PATTERNS = [
  'api', '/api', 'api/', '**/api/**', 'api/**',
  'src/**', 'src/*', '/src/a.ts', '*.ts', '**/*.test.*', 'node_modules', 'dist/**',
  'deep', '**/deep/**',
]

function q(s) {
  return '"' + s.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"'
}

function buildTree(root) {
  fs.rmSync(root, { recursive: true, force: true })
  fs.mkdirSync(root, { recursive: true })
  execFileSync('git', ['init', '-q', '.'], { cwd: root })
  // A path that is also a parent directory of another path cannot exist as a file.
  const dirs = new Set()
  for (const p of PATHS) {
    const parts = p.split('/')
    for (let i = 1; i < parts.length; i++) dirs.add(parts.slice(0, i).join('/'))
  }
  const dropped = PATHS.filter((p) => dirs.has(p))
  if (dropped.length) console.log('dropped (exist as directories):', dropped.join(', '))
  const paths = PATHS.filter((p) => !dirs.has(p))
  for (const p of paths) {
    const full = path.join(root, p)
    fs.mkdirSync(path.dirname(full), { recursive: true })
    fs.writeFileSync(full, 'x\n')
  }
  return paths
}

function oracle(root, paths, patternLines) {
  fs.writeFileSync(path.join(root, '.gitignore'), patternLines.join('\n') + '\n')
  // check-ignore exits 1 when nothing matches, which is not an error for us
  let stdout = ''
  try {
    stdout = execFileSync('git', ['check-ignore', '--stdin'], {
      cwd: root,
      input: paths.join('\n') + '\n',
      encoding: 'utf8',
    })
  } catch (e) {
    stdout = e.stdout || ''
  }
  return stdout.split('\n').map((l) => l.trim()).filter(Boolean).sort()
}

function buildScopedTree(root) {
  for (const p of SCOPED_PATHS) {
    const full = path.join(root, p)
    fs.mkdirSync(path.dirname(full), { recursive: true })
    if (!fs.existsSync(full)) fs.writeFileSync(full, 'x\n')
  }
}

function scopedOracle(root, pattern) {
  const nested = path.join(root, SCOPED_ROOT, '.gitignore')
  fs.mkdirSync(path.dirname(nested), { recursive: true })
  fs.writeFileSync(nested, pattern + '\n')
  let stdout = ''
  try {
    stdout = execFileSync('git', ['check-ignore', '--stdin'], {
      cwd: root, input: SCOPED_PATHS.join('\n') + '\n', encoding: 'utf8',
    })
  } catch (e) {
    stdout = e.stdout || ''
  }
  fs.rmSync(nested, { force: true })
  return stdout.split('\n').map((l) => l.trim()).filter(Boolean).sort()
}

const root = path.join(os.tmpdir(), 'rev-dep-gitignore-oracle')
const paths = buildTree(root)
const gitVersion = execFileSync('git', ['--version'], { encoding: 'utf8' }).trim()

const cases = []
for (const p of PATTERNS) cases.push({ patterns: [p], desc: '', matches: oracle(root, paths, [p]) })
for (const [pats, desc] of MULTI) cases.push({ patterns: pats, desc, matches: oracle(root, paths, pats) })

buildScopedTree(root)
const scopedCases = SCOPED_PATTERNS.map((p) => ({ pattern: p, matches: scopedOracle(root, p) }))

const out = []
out.push('package globutil')
out.push('')
out.push('import "testing"')
out.push('')
out.push('// Code generated by scripts/genGitignoreParityTest.js. DO NOT EDIT.')
out.push('//')
out.push('// Ground truth for .gitignore parity. Every expectation was produced by running the real')
out.push(`// \`git check-ignore\` (${gitVersion}) over a fixed file tree, one .gitignore per case. git is`)
out.push("// used as an oracle, so no test data is copied from git's own GPL-licensed test suite.")
out.push('//')
out.push('// A case lists the paths git ignores; every other path in gitignoreParityPaths must NOT')
out.push('// match, so each case asserts over the whole corpus rather than only the positives.')
out.push('')
out.push('var gitignoreParityPaths = []string{')
for (const p of paths) out.push('\t' + q(p) + ',')
out.push('}')
out.push('')
out.push('type gitignoreParityCase struct {')
out.push('\tpatterns []string')
out.push('\tdesc     string')
out.push('\tmatches  []string // paths git ignores; all others must not match')
out.push('}')
out.push('')
out.push('var gitignoreParityCases = []gitignoreParityCase{')
for (const c of cases) {
  const pats = c.patterns.map(q).join(', ')
  const ms = c.matches.map(q).join(', ')
  const desc = c.desc ? `desc: ${q(c.desc)}, ` : ''
  out.push(`\t{patterns: []string{${pats}}, ${desc}matches: []string{${ms}}},`)
}
out.push('}')
out.push('')
out.push('// Paths used by the scoped cases; the pattern root is ' + q(SCOPED_ROOT) + '.')
out.push('var gitignoreScopedPaths = []string{')
for (const p of SCOPED_PATHS) out.push('\t' + q(p) + ',')
out.push('}')
out.push('')
out.push('var gitignoreScopedCases = []gitignoreParityCase{')
for (const c of scopedCases) {
  out.push(`\t{patterns: []string{${q(c.pattern)}}, matches: []string{${c.matches.map(q).join(', ')}}},`)
}
out.push('}')
out.push(`
// TestGitignoreParityScoped covers patterns rooted at a workspace rather than the repo
// root. Ground truth comes from a .gitignore placed in ${SCOPED_ROOT}/ - git roots such a
// file's patterns at its own directory and never applies them to a sibling, which is
// exactly the scoping rev-dep gives a workspace.
func TestGitignoreParityScoped(t *testing.T) {
	const repo = "/repo/"
	const patternRoot = repo + "${SCOPED_ROOT}"
	for _, tc := range gitignoreScopedCases {
		t.Run(tc.patterns[0], func(t *testing.T) {
			want := map[string]bool{}
			for _, m := range tc.matches {
				want[m] = true
			}
			matchers := CreateGlobMatchers(tc.patterns, patternRoot)
			for _, p := range gitignoreScopedPaths {
				got := MatchesAnyGlobMatcher(repo+p, matchers, debug)
				if got != want[p] {
					t.Errorf("pattern %q rooted at %q vs %q: got match=%v, git says %v",
						tc.patterns[0], patternRoot, p, got, want[p])
				}
			}
		})
	}
}`)

out.push(`
func TestGitignoreParity(t *testing.T) {
	const root = "/repo/"
	for _, tc := range gitignoreParityCases {
		name := tc.desc
		if name == "" {
			name = tc.patterns[0]
		}
		t.Run(name, func(t *testing.T) {
			want := map[string]bool{}
			for _, m := range tc.matches {
				want[m] = true
			}
			matchers := CreateGlobMatchers(tc.patterns, root)
			for _, p := range gitignoreParityPaths {
				got := MatchesAnyGlobMatcher(root+p, matchers, debug)
				if got != want[p] {
					t.Errorf("patterns %v vs %q: got match=%v, git says %v", tc.patterns, p, got, want[p])
				}
			}
		})
	}
}`)

const dest = path.join(__dirname, '..', 'internal', 'glob', 'gitignore_parity_test.go')
fs.writeFileSync(dest, out.join('\n') + '\n')
console.log(`wrote ${path.relative(path.join(__dirname, '..'), dest)}`)
console.log(`cases: ${cases.length} | paths: ${paths.length} | assertions: ${cases.length * paths.length}`)
console.log(`oracle: ${gitVersion}`)
