#!/usr/bin/env node
/**
 * Renders the landing page's hero terminal to demo.png, with arrows pointing at
 * the file count and the runtime.
 *
 * Why a separate renderer instead of screenshotting the live page? The hero
 * terminal is sized by the hero's grid, sits on a gradient, and plays a typing
 * animation - none of which we want baked into a README image. This draws the
 * same component in isolation, at a fixed width, in its finished state.
 *
 * Nothing here re-implements the terminal's look:
 *
 *   - Terminal.module.css is injected verbatim. It is written with single-class
 *     selectors only, so the raw file applies directly to plain class names -
 *     no CSS-modules hashing to reproduce. Restyle the component and this image
 *     follows on the next run.
 *   - The --rd-term-* custom properties are lifted straight out of custom.css,
 *     so the exported window is the colour the site renders, not a copy of it.
 *   - The lines come from data/demoOutput.ts, the same module Hero.tsx imports
 *     (Node strips the types). The image cannot drift from the page.
 *
 * The single override is the drop shadow, turned off below: on a transparent
 * PNG it exports as a soft grey halo that fringes against whatever background
 * the README is viewed on.
 *
 * RUN THIS ON macOS. The output contains emoji (folder, check mark, sparkles)
 * and the browser paints them with whatever emoji font the host has: Apple
 * Color Emoji on a Mac, Noto Color Emoji on Linux. They are visibly different
 * glyphs, and the Mac set is what most readers of the README will expect from a
 * terminal screenshot. Everything else - Geist Mono included - is embedded, so
 * the emoji are the only host-dependent part of the render.
 *
 * Usage:
 *   npx playwright install chromium        # once per machine
 *   node scripts/build-demo.mjs            # 3x, the default
 *   DEMO_SCALE=4 node scripts/build-demo.mjs
 */
import { platform } from 'node:os';
import { readFileSync, writeFileSync } from 'node:fs';
import { chromium } from 'playwright';
import { demoOutput, demoTitle } from '../src/components/landing/data/demoOutput.ts';

/**
 * Layout width. Narrower than the hero's ~635px because the hero's width comes
 * from its grid, not from the design: trimming the dead space on the right (the
 * arrows still fit) means the same text occupies more of the frame, so it lands
 * on more pixels wherever the image is displayed.
 */
const WIDTH = 560;

/**
 * Supersampling factor. The image is displayed at ~490 CSS px, which a retina
 * screen paints with 980-1470 device pixels - so the PNG has to carry at least
 * that many across, plus enough headroom that the downscale is a clean
 * resample rather than a blur. 3x gives 1680px wide; 4x is there for print-like
 * uses and costs ~2x the file size for no visible gain on screen.
 */
const SCALE = Number(process.env.DEMO_SCALE) || 3;

const OUT = ['../demo.png', 'static/img/demo.png'];

const FONT = readFileSync('static/fonts/GeistMono-Variable.woff2').toString('base64');
const TERMINAL_CSS = readFileSync(
  'src/components/landing/primitives/Terminal.module.css',
  'utf8',
);

/**
 * Pulls the terminal's design tokens out of custom.css so the export uses the
 * site's own colours. They are declared once, under :root, with no dark-theme
 * override - the window is a dark object in both themes - so a flat scrape is
 * enough and there is no cascade to resolve.
 */
function themeTokens() {
  const css = readFileSync('src/css/custom.css', 'utf8');
  const wanted = /^\s*(--rd-term-[\w-]+|--rd-card-radius)\s*:\s*([^;]+);/gm;
  const out = [];
  for (const [, name, value] of css.matchAll(wanted)) {
    out.push(`${name}: ${value.trim()};`);
  }
  if (!out.some((d) => d.startsWith('--rd-term-bg'))) {
    throw new Error('No --rd-term-* tokens found in custom.css - did they move or get renamed?');
  }
  return out.join('\n    ');
}

const escapeHtml = (s) =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

/** Mirrors Terminal.tsx's markup, minus the entrance-animation classes. */
function renderLines(lines) {
  return lines
    .map((line) => {
      const cls = ['line', `line${(line.tone ?? 'default').replace(/^./, (c) => c.toUpperCase())}`];
      cls.push(`indent${line.indent ?? 0}`);
      if (line.gap) cls.push('lineGap');

      const inner = line.parts
        ? line.parts
            .map((part) => {
              const pc = [`line${(part.tone ?? 'default').replace(/^./, (c) => c.toUpperCase())}`];
              if (part.strong) pc.push('strong');
              // Arrows anchor to the emphasised parts, so adding or removing a
              // `strong` part in demoOutput.ts adds or removes an arrow.
              const anchor = part.strong ? ' data-arrow' : '';
              return `<span class="${pc.join(' ')}"${anchor}>${escapeHtml(part.text)}</span>`;
            })
            .join('')
        : escapeHtml(line.text || ' ');

      return `<span class="${cls.join(' ')}">${inner}</span>`;
    })
    .join('');
}

/**
 * A tapered arrow: triangular head, shaft narrowing toward the tail.
 * Built pointing right from the origin, then rotated onto `angle` and moved so
 * the tip lands on (x, y).
 *
 * Sized to point without competing with the output it points at. Scale the four
 * constants together - the head reads as a wedge only while HEAD_HALF stays
 * roughly 2.5x SHAFT_HALF.
 */
function arrowPath(x, y, angle, length) {
  const HEAD_LEN = 21;
  const HEAD_HALF = 12;
  const SHAFT_HALF = 4.5;
  const TAIL_HALF = 2;

  const pts = [
    [0, 0],
    [-HEAD_LEN, -HEAD_HALF],
    [-HEAD_LEN, -SHAFT_HALF],
    [-length, -TAIL_HALF],
    [-length, TAIL_HALF],
    [-HEAD_LEN, SHAFT_HALF],
    [-HEAD_LEN, HEAD_HALF],
  ];

  const rad = (angle * Math.PI) / 180;
  const cos = Math.cos(rad);
  const sin = Math.sin(rad);

  return (
    pts
      .map(([px, py]) => `${(x + px * cos - py * sin).toFixed(1)},${(y + px * sin + py * cos).toFixed(1)}`)
      .join(' ')
  );
}

const html = `<!doctype html><meta charset="utf-8">
<style>
  @font-face {
    font-family: 'Geist Mono';
    src: url(data:font/woff2;base64,${FONT}) format('woff2-variations');
    font-weight: 100 900;
  }
  :root {
    --ifm-font-family-monospace: 'Geist Mono', ui-monospace, monospace;
    ${themeTokens()}
    /* Overrides the value scraped above: a drop shadow exports as a soft grey
       halo on a transparent PNG and fringes against whatever background the
       README is viewed on. Must stay last so it wins. */
    --rd-term-shadow: none;
  }
  html, body { margin: 0; background: transparent; }
  #shot { width: ${WIDTH}px; position: relative; }
  #arrows {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }
${TERMINAL_CSS}
</style>
<div id="shot">
  <div class="terminal">
    <div class="chrome">
      <span class="dotRed"></span><span class="dotAmber"></span><span class="dotGreen"></span>
      <span class="chromeTitle">${escapeHtml(demoTitle)}</span>
    </div>
    <pre class="body">${renderLines(demoOutput)}</pre>
  </div>
  <svg id="arrows" xmlns="http://www.w3.org/2000/svg">
    <defs>
      <linearGradient id="g" x1="0" y1="0" x2="1" y2="0">
        <stop offset="0" stop-color="#b237e2"/>
        <stop offset="1" stop-color="#df734f"/>
      </linearGradient>
    </defs>
  </svg>
</div>`;

// Loud rather than silent: a regeneration on CI or a Linux box would swap every
// emoji for the Noto set without changing anything else, and the diff would
// look like a routine re-render.
if (platform() !== 'darwin') {
  console.warn(
    `! Running on ${platform()}, not macOS - emoji will render with the host's\n` +
      '  emoji font (Noto on Linux) instead of Apple Color Emoji. The image is\n' +
      '  still valid, it just will not match the committed one.',
  );
}

const browser = await chromium.launch();
const page = await browser.newPage({ deviceScaleFactor: SCALE });
await page.setContent(html, { waitUntil: 'load' });
await page.evaluate(() => document.fonts.ready);
await page.waitForTimeout(200);

// Arrows are placed from the measured DOM rather than hard-coded coordinates,
// so editing the output lines moves them instead of breaking them.
await page.evaluate(
  ({ pathFn }) => {
    const buildPath = new Function('return ' + pathFn)();
    const host = document.getElementById('shot').getBoundingClientRect();
    const svg = document.getElementById('arrows');
    svg.setAttribute('viewBox', `0 0 ${host.width} ${host.height}`);

    /* Both arrows come in from the upper right, as they always have. Pointing
       one up from below looks livelier but there is no room: the runtime is the
       last line in the window, so a tail heading down leaves the canvas. */
    const ANGLE = 160;
    const LENGTH = 100;
    const MARGIN = 16;

    const rad = (ANGLE * Math.PI) / 180;
    const dx = Math.cos(rad);
    const dy = Math.sin(rad);

    document.querySelectorAll('[data-arrow]').forEach((el) => {
      const r = el.getBoundingClientRect();
      // Clear of the closing ")" / "." that follows each highlighted value.
      const tipX = r.right - host.left + 16;
      const tipY = r.top - host.top + r.height / 2;

      // Shorten rather than overflow if the tail would leave the window - the
      // image is cropped to the terminal, so anything outside is simply gone.
      let length = LENGTH;
      const limit = (avail, delta) => (delta > 0 ? Math.min(length, avail / delta) : length);
      // tail = tip - length * (dx, dy), so a negative component grows +x / +y.
      length = limit(host.width - MARGIN - tipX, -dx);
      length = limit(tipX - MARGIN, dx);
      length = limit(host.height - MARGIN - tipY, -dy);
      length = limit(tipY - MARGIN, dy);

      const poly = document.createElementNS('http://www.w3.org/2000/svg', 'polygon');
      poly.setAttribute('points', buildPath(tipX, tipY, ANGLE, length));
      poly.setAttribute('fill', 'url(#g)');
      svg.appendChild(poly);
    });
  },
  { pathFn: arrowPath.toString() },
);

await page.waitForTimeout(120);

const buf = await page.locator('#shot').screenshot({ omitBackground: true });
for (const out of OUT) {
  writeFileSync(out, buf);
  console.log(`${out}  ${(buf.length / 1024).toFixed(0)}KB`);
}

await browser.close();
