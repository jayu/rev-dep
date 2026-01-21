const fs = require('fs').promises;
const { execSync } = require('child_process');
const path = require('path');

const settings = {
  commands: [
    { name: "circular" },
    {
      name: "config",
      subcommands: [
        "run",
        "init"
      ]
    },
    { name: "entry-points" },
    { name: "files" },
    { name: "imported-by" },
    { name: "lines-of-code" },
    { name: "list-cwd-files" },
    {
      name: "node-modules",
      subcommands: [
        "dirs-size",
        "installed-duplicates",
        "installed",
        "missing",
        "unused",
        "used"
      ]
    },
    { name: "resolve" }
  ]
};

const DOCS_DIR = path.join(process.cwd(), 'docs');
const README_PATH = path.join(process.cwd(), 'Readme.md');
const NPM_README_PATH = path.join(process.cwd(), 'npm', 'rev-dep', 'Readme.md');
const DOCS_START_MARKER = '<!-- cli-docs-start -->';
const DOCS_END_MARKER = '<!-- cli-docs-end -->';

async function generateDocs() {
  console.log('Generating documentation...');
  await fs.mkdir(DOCS_DIR);
  const isLinux = process.platform === 'linux';
  const cmd = path.join(process.cwd(), 'npm', '@rev-dep', isLinux ? 'linux-x64' : 'darwin-arm64', 'bin', 'rev-dep')
  try {
    execSync(cmd + ' doc-gen', { stdio: 'inherit' });
  } catch (error) {
    console.error('Failed to generate documentation:', error.message);
    process.exit(1);
  }
}

function increaseHeaderLevel(content) {
  // First, replace the deepest headers (#######) to avoid multiple replacements
  // We'll work from deepest to shallowest
  return content
    // Replace ###### with #######
    .replace(/^######\s+(.*$)/gm, '####### $1')
    // Replace ##### with ######
    .replace(/^#####\s+(.*$)/gm, '###### $1')
    // Replace #### with #####
    .replace(/^####\s+(.*$)/gm, '##### $1')
    // Replace ### with ####
    .replace(/^###\s+(.*$)/gm, '#### $1')
    // Replace ## with ###
    .replace(/^##\s+(.*$)/gm, '### $1')
    // Replace # with ##
    .replace(/^#\s+(.*$)/gm, '## $1');
}

function cleanContent(content) {
  // Remove SEE ALSO sections and Auto generated lines
  let cleaned = content.replace(/### SEE ALSO[\s\S]*?###### Auto generated[^\n]*/g, '');

  // Replace current working directory with $PWD
  const cwd = process.cwd();
  cleaned = cleaned.replace(new RegExp(cwd.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'g'), '$PWD');

  // Increase header levels
  return increaseHeaderLevel(cleaned);
}

async function collectDocumentation() {
  let result = '\n';

  for (const cmd of settings.commands) {
    const cmdFileName = `rev-dep_${cmd.name}.md`;
    const cmdPath = path.join(DOCS_DIR, cmdFileName);

    try {
      // Add main command documentation
      let content = await fs.readFile(cmdPath, 'utf8');
      content = cleanContent(content);
      result += `${content}`;

      // Add subcommands if they exist
      if (cmd.subcommands) {
        for (const subcmd of cmd.subcommands) {
          const subcmdFileName = `rev-dep_${cmd.name}_${subcmd}.md`;
          const subcmdPath = path.join(DOCS_DIR, subcmdFileName);
          try {
            let subcontent = await fs.readFile(subcmdPath, 'utf8');
            subcontent = cleanContent(subcontent);
            result += `${subcontent}`;
          } catch (err) {
            console.warn(`Warning: Could not read documentation for ${cmd.name} ${subcmd}:`, err.message);
          }
        }
      }
    } catch (err) {
      console.warn(`Warning: Could not read documentation for ${cmd.name}:`, err.message);
    }
  }

  return result;
}

async function updateReadme(docsContent) {
  try {
    let readme = await fs.readFile(README_PATH, 'utf8');

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
  } catch (err) {
    console.error('Error updating Readme.md:', err.message);
    throw err;
  }
}

async function updateNpmReadme() {
  try {
    const mainReadme = await fs.readFile(README_PATH, 'utf8');
    await fs.writeFile(NPM_README_PATH, mainReadme, 'utf8');
    console.log('Successfully updated npm/rev-dep/Readme.md');
  } catch (err) {
    console.error('Error updating npm/rev-dep/Readme.md:', err.message);
    throw err;
  }
}

async function cleanup() {
  try {
    await fs.rm(DOCS_DIR, { recursive: true, force: true });
    console.log('Cleaned up documentation directory');
  } catch (err) {
    console.warn('Warning: Could not clean up documentation directory:', err.message);
  }
}

async function main() {
  try {
    // Generate documentation
    await generateDocs();

    // Collect documentation for specified commands
    const docsContent = await collectDocumentation();

    // Update Readme.md
    await updateReadme(docsContent);

    // Copy main README content to npm/rev-dep README
    await updateNpmReadme();

    // Clean up
    await cleanup();

    console.log('Documentation update completed successfully!');
  } catch (err) {
    console.error('Error:', err.message);
    process.exit(1);
  }
}

main();