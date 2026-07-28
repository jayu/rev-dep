#!/usr/bin/env node
/**
 * Renders the logo SVGs to transparent PNGs with Geist baked into the
 * letterforms.
 *
 * Why not ship the SVGs directly? Both draw their text with `font-family:
 * system-ui`, and an SVG loaded via <img> cannot see the page's @font-face -
 * so the wordmark renders in a different typeface on every OS, at a different
 * width. Rendering here, in a browser that has Geist, freezes one correct
 * result.
 *
 * Originals are kept in static/img/source/.
 *
 * Usage: node scripts/build-logos.mjs
 */
import { readFileSync, writeFileSync } from 'node:fs';
import { chromium } from 'playwright';

const FONT = readFileSync('static/fonts/Geist-Variable.woff2').toString('base64');

const faviconSrc = readFileSync('static/img/source/favicon.svg', 'utf8');

/** Mark on its dark rounded card, as the favicon looks. */
const markSvg = faviconSrc.replace(/font-family="[^"]*"/, 'font-family="Geist"');

/** Same mark with the card removed, for surfaces that supply their own. */
const markBareSvg = markSvg.replace(/<rect[^>]*fill="url\(#revdepBg\)"[^>]*\/>/, '');

/** Wordmark: graph "R" is already vector; only "ev-dep" is text. */
const wordSvg = readFileSync('static/img/source/logo-text.svg', 'utf8')
  .replace('font-family: system-ui;', 'font-family: Geist;')
  // The source declares height 560, which clips the descender of the "p".
  // Give it room; the render is trimmed afterwards anyway.
  .replace('width="1500" height="560"', 'width="1560" height="620" viewBox="0 0 1560 620"');

const page = `<!doctype html><meta charset="utf-8"><style>
  @font-face{font-family:'Geist';src:url(data:font/woff2;base64,${FONT}) format('woff2-variations');font-weight:100 900;}
  html,body{margin:0;background:transparent}
  #a,#b{display:block}
</style>
<div id="a">${markSvg}</div><div id="b">${wordSvg}</div><div id="c">${markBareSvg}</div>`;

const browser = await chromium.launch();
const p = await browser.newPage({ deviceScaleFactor: 3 });
await p.setContent(page, { waitUntil: 'load' });
await p.evaluate(() => document.fonts.ready);
await p.waitForTimeout(300);

for (const [sel, out, w] of [
  ['#a svg', 'static/img/logo-mark-card.png', 320],
  ['#c svg', 'static/img/logo-mark.png', 320],
  ['#b svg', 'static/img/logo-wordmark.png', 1180],
]) {
  await p.$eval(sel, (el, width) => { el.setAttribute('width', String(width)); el.removeAttribute('height'); }, w);
  await p.waitForTimeout(120);
  const buf = await (await p.$(sel)).screenshot({ omitBackground: true });
  writeFileSync(out, buf);
  console.log(`${out}  ${(buf.length / 1024).toFixed(0)}KB`);
}

await browser.close();
