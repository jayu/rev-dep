#!/usr/bin/env node
/**
 * Waits until the dev server has recompiled and is serving the expected text,
 * instead of guessing with `sleep`.
 *
 * Usage: node scripts/wait-for-hmr.mjs "<text that must be present>" [url]
 */
const needle = process.argv[2];
const url = process.argv[3] ?? 'http://localhost:3000/';
const deadline = Date.now() + 60000;

const { chromium } = await import('playwright');
const browser = await chromium.launch();
const page = await browser.newPage();

let ok = false;
while (Date.now() < deadline) {
  try {
    await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 15000 });
    await page.waitForTimeout(400);
    const body = await page.evaluate(() => document.body.innerText);
    if (!needle || body.includes(needle)) { ok = true; break; }
  } catch {}
  await new Promise((r) => setTimeout(r, 500));
}

await browser.close();
console.log(ok ? `ready in ~${((60000 - (deadline - Date.now())) / 1000).toFixed(1)}s` : 'TIMEOUT');
process.exit(ok ? 0 : 1);
