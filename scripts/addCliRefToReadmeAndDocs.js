const fsSync = require('fs');
const fs = fsSync.promises;
const { execFileSync } = require('child_process');
const path = require('path');

const settings = {
  commands: [
    { name: 'circular' },
    {
      name: 'config',
      subcommands: ['run', 'init'],
    },
    { name: 'entry-points' },
    { name: 'files' },
    { name: 'imported-by' },
    { name: 'lines-of-code' },
    { name: 'list-cwd-files' },
    { name: 'unresolved' },
    {
      name: 'node-modules',
      subcommands: [
        'dirs-size',
        'installed-duplicates',
        'installed',
        'missing',
        'unused',
        'used',
      ],
    },
    { name: 'resolve' },
  ],
};

const TEMP_DOCS_DIR = path.join(process.cwd(), 'tmp', 'cli-docs-generated');
const README_PATH = path.join(process.cwd(), 'README.md');
const NPM_README_PATH = path.join(process.cwd(), 'npm', 'rev-dep', 'Readme.md');
const DOCS_REFERENCE_DIR = path.join(
  process.cwd(),
  'docs',
  'docs',
  'cli-reference',
  'generated',
);
const SIDEBAR_PATH = path.join(process.cwd(), 'docs', 'sidebars.ts');
const DOCS_START_MARKER = '<!-- cli-docs-start -->';
const DOCS_END_MARKER = '<!-- cli-docs-end -->';
const SIDEBAR_START_MARKER = '// cli-reference-generated-start';
const SIDEBAR_END_MARKER = '// cli-reference-generated-end';
const followAllToken = '[=__REV_DEP_FOLLOW_ALL__]';

function getBinaryPath() {
  const isLinux = process.platform === 'linux';
  return path.join(
    process.cwd(),
    'npm',
    '@rev-dep',
    isLinux ? 'linux-x64' : 'darwin-arm64',
    'bin',
    'rev-dep',
  );
}

function runDocGen(args) {
  const binaryPath = getBinaryPath();
  const hasPackagedBinary = fsSync.existsSync(binaryPath);

  if (hasPackagedBinary) {
    try {
      execFileSync(binaryPath, args, { stdio: 'inherit' });
      return;
    } catch (error) {
      console.warn(
        'Packaged rev-dep binary could not generate docs, falling back to `go run .`:',
        error.message,
      );
    }
  }

  execFileSync('go', ['run', '.', ...args], {
    cwd: process.cwd(),
    stdio: 'inherit',
    env: {
      ...process.env,
      GOCACHE: process.env.GOCACHE || path.join(process.cwd(), 'tmp', 'gocache'),
    },
  });
}

function getCommandPaths() {
  const commandPaths = [];

  for (const command of settings.commands) {
    commandPaths.push(command.name);
    if (command.subcommands) {
      for (const subcommand of command.subcommands) {
        commandPaths.push(`${command.name} ${subcommand}`);
      }
    }
  }

  return commandPaths;
}

function commandPathToFileName(commandPath) {
  return `rev-dep_${commandPath.replaceAll(' ', '_')}.md`;
}

function commandNameToDocId(commandName) {
  return `cli-reference/generated/rev-dep_${commandName}.md`.replace(/\.md$/, '');
}

function renderSidebarItem(command) {
  if (!command.subcommands || command.subcommands.length === 0) {
    return `        '${commandNameToDocId(command.name)}',`;
  }

  const subcommandItems = command.subcommands
    .map(
      (subcommand) =>
        `            '${commandNameToDocId(`${command.name}_${subcommand}`)}',`,
    )
    .join('\n');

  return [
    '        {',
    "          type: 'category',",
    `          label: 'rev-dep ${command.name}',`,
    '          items: [',
    `            '${commandNameToDocId(command.name)}',`,
    subcommandItems,
    '          ],',
    '        },',
  ].join('\n');
}

function renderSidebarGeneratedSection() {
  const items = [
    "        'cli-reference/overview',",
    ...settings.commands.map(renderSidebarItem),
  ].join('\n');

  return `${SIDEBAR_START_MARKER}\n${items}\n        ${SIDEBAR_END_MARKER}`;
}

async function generateDocs() {
  console.log('Generating CLI documentation...');
  await fs.rm(TEMP_DOCS_DIR, { recursive: true, force: true });
  await fs.mkdir(TEMP_DOCS_DIR, { recursive: true });

  const args = ['doc-gen', '--output-dir', TEMP_DOCS_DIR];
  for (const commandPath of getCommandPaths()) {
    args.push('--command-paths', commandPath);
  }

  try {
    runDocGen(args);
  } catch (error) {
    console.error('Failed to generate documentation:', error.message);
    process.exit(1);
  }
}

function increaseHeaderLevel(content) {
  return content
    .replace(/^######\s+(.*$)/gm, '####### $1')
    .replace(/^#####\s+(.*$)/gm, '###### $1')
    .replace(/^####\s+(.*$)/gm, '##### $1')
    .replace(/^###\s+(.*$)/gm, '#### $1')
    .replace(/^##\s+(.*$)/gm, '### $1')
    .replace(/^#\s+(.*$)/gm, '## $1');
}

function sanitizeFollowAllToken(content) {
  return content.replaceAll(followAllToken, ' '.repeat(followAllToken.length));
}

function replaceCwdWithPwd(content) {
  const cwd = process.cwd();
  return content.replace(
    new RegExp(cwd.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'g'),
    '$PWD',
  );
}

function stripSeeAlsoAndFooter(content) {
  return content
    .replace(/### SEE ALSO[\s\S]*?###### Auto generated[^\n]*/g, '')
    .replace(/\n*###### Auto generated[^\n]*/g, '');
}

function cleanReadmeContent(content) {
  return sanitizeFollowAllToken(
    increaseHeaderLevel(replaceCwdWithPwd(stripSeeAlsoAndFooter(content))),
  );
}

function extractTitle(content) {
  const match = content.match(/^##\s+(.+)$/m);
  if (!match) {
    throw new Error('Could not extract title from generated CLI doc');
  }
  return match[1].trim();
}

function cleanDocsContent(content) {
  const stripped = replaceCwdWithPwd(stripSeeAlsoAndFooter(content));
  return sanitizeFollowAllToken(stripped).replace(/^##\s+.+\n+/, '');
}

function renderDocsPage(content) {
  const title = extractTitle(content);
  const body = cleanDocsContent(content).trim();
  return `---\ntitle: ${title}\n---\n\n${body}\n`;
}

async function readGeneratedCommandDoc(commandPath) {
  const docPath = path.join(TEMP_DOCS_DIR, commandPathToFileName(commandPath));
  return fs.readFile(docPath, 'utf8');
}

async function collectDocumentation() {
  let result = '\n';

  for (const commandPath of getCommandPaths()) {
    try {
      const content = await readGeneratedCommandDoc(commandPath);
      result += cleanReadmeContent(content);
    } catch (err) {
      console.warn(
        `Warning: Could not read documentation for ${commandPath}:`,
        err.message,
      );
    }
  }

  return result;
}

async function updateReadme(docsContent) {
  const readme = await fs.readFile(README_PATH, 'utf8');

  const startIndex = readme.indexOf(DOCS_START_MARKER);
  const endIndex = readme.indexOf(DOCS_END_MARKER);

  if (startIndex === -1 || endIndex === -1) {
    throw new Error('Documentation markers not found in Readme.md');
  }

  const beforeContent = readme.substring(0, startIndex + DOCS_START_MARKER.length);
  const afterContent = readme.substring(endIndex);
  const updatedReadme = `${beforeContent}\n${docsContent}\n${afterContent}`;

  await fs.writeFile(README_PATH, updatedReadme, 'utf8');
  console.log('Successfully updated Readme.md');
}

async function updateNpmReadme() {
  const mainReadme = await fs.readFile(README_PATH, 'utf8');
  await fs.writeFile(NPM_README_PATH, mainReadme, 'utf8');
  console.log('Successfully updated npm/rev-dep/Readme.md');
}

async function updateDocsReference() {
  await fs.rm(DOCS_REFERENCE_DIR, { recursive: true, force: true });
  await fs.mkdir(DOCS_REFERENCE_DIR, { recursive: true });

  for (const commandPath of getCommandPaths()) {
    const fileName = commandPathToFileName(commandPath);
    const content = await readGeneratedCommandDoc(commandPath);
    await fs.writeFile(
      path.join(DOCS_REFERENCE_DIR, fileName),
      renderDocsPage(content),
      'utf8',
    );
  }

  console.log('Successfully updated docs CLI reference');
}

async function updateDocsSidebar() {
  const sidebar = await fs.readFile(SIDEBAR_PATH, 'utf8');
  const startIndex = sidebar.indexOf(SIDEBAR_START_MARKER);
  const endIndex = sidebar.indexOf(SIDEBAR_END_MARKER);

  if (startIndex === -1 || endIndex === -1) {
    throw new Error('CLI sidebar markers not found in docs/sidebars.ts');
  }

  const before = sidebar.substring(0, startIndex);
  const after = sidebar.substring(endIndex + SIDEBAR_END_MARKER.length);
  const generated = renderSidebarGeneratedSection();

  await fs.writeFile(SIDEBAR_PATH, `${before}${generated}${after}`, 'utf8');
  console.log('Successfully updated docs CLI sidebar');
}

async function cleanup() {
  await fs.rm(TEMP_DOCS_DIR, { recursive: true, force: true });
  console.log('Cleaned up temporary CLI docs directory');
}

async function main() {
  try {
    await generateDocs();
    const docsContent = await collectDocumentation();
    await updateReadme(docsContent);
    await updateNpmReadme();
    await updateDocsReference();
    await updateDocsSidebar();
    await cleanup();
    console.log('CLI documentation update completed successfully!');
  } catch (err) {
    console.error('Error:', err.message);
    process.exit(1);
  }
}

main();
