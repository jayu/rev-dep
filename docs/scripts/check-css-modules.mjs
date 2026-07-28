#!/usr/bin/env node
/**
 * Enforces the landing page CSS rule: every selector is a single class name.
 *
 *   allowed    .card            .card:hover        .tab:focus-visible
 *   rejected   .card .title     .list li           [data-theme='dark'] .card
 *              div              .a > .b            .a, div
 *
 * Theming must go through custom properties defined in src/css/custom.css,
 * because `[data-theme='dark'] .foo` is a two-part selector.
 *
 * Usage: node scripts/check-css-modules.mjs [dir ...]
 */

import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';

const roots = process.argv.slice(2);

// src/theme/** is deliberately excluded: swizzled Docusaurus components have to
// target Infima's own global class names, which by definition cannot be given a
// local class. Everything we author ourselves must follow the rule.
const searchRoots = roots.length > 0 ? roots : ['src/components', 'src/pages'];

/** A single class, optionally with pseudo-classes/elements. */
const VALID_SELECTOR = /^\.[A-Za-z_][\w-]*(::?[\w-]+(\([^)]*\))?)*$/;

function collectCssModules(dir, found = []) {
  for (const entry of readdirSync(dir)) {
    if (entry === 'node_modules' || entry.startsWith('.')) continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) collectCssModules(full, found);
    else if (entry.endsWith('.module.css')) found.push(full);
  }
  return found;
}

/** Strip comments, then read the selector text preceding each `{`. */
function findViolations(css) {
  const stripped = css.replace(/\/\*[\s\S]*?\*\//g, '');
  const violations = [];
  let depth = 0;
  let buffer = '';
  // Depth at which a @keyframes block opened. Its children are keyframe
  // selectors (`from`, `to`, `50%`), not CSS selectors, so they are skipped.
  let keyframesDepth = -1;

  for (const char of stripped) {
    if (char === '{') {
      const prelude = buffer.trim();
      buffer = '';
      depth += 1;
      // At depth 0 a prelude is a selector; at-rules (@media, @supports) open a
      // block whose children are selectors, so only skip the at-rule itself.
      if (prelude.startsWith('@')) {
        if (/^@(-\w+-)?keyframes\b/.test(prelude)) keyframesDepth = depth;
        continue;
      }
      if (keyframesDepth !== -1 && depth > keyframesDepth) continue;
      if (!prelude) continue;

      for (const selector of prelude.split(',')) {
        const trimmed = selector.trim();
        if (trimmed && !VALID_SELECTOR.test(trimmed)) violations.push(trimmed);
      }
      continue;
    }
    if (char === '}') {
      if (keyframesDepth !== -1 && depth === keyframesDepth) keyframesDepth = -1;
      depth -= 1;
      buffer = '';
      continue;
    }
    buffer += char;
  }
  return violations;
}

let failures = 0;
let checked = 0;

for (const root of searchRoots) {
  for (const file of collectCssModules(root)) {
    checked += 1;
    const violations = findViolations(readFileSync(file, 'utf8'));
    if (violations.length > 0) {
      failures += violations.length;
      console.error(`\n✗ ${relative(process.cwd(), file)}`);
      for (const selector of violations) console.error(`    ${selector}`);
    }
  }
}

if (failures > 0) {
  console.error(
    `\n${failures} selector(s) in ${checked} file(s) are not a single class name.\n` +
      'Give the element its own class, and move any theming into custom.css tokens.\n',
  );
  process.exit(1);
}

console.log(`✓ ${checked} CSS module(s): every selector is a single class name.`);
