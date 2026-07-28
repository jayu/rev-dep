/**
 * Reveals the page once the webfonts have resolved.
 *
 * The document starts hidden via a `data-rd-boot` attribute, set by an inline
 * script in <head> (see docusaurus.config.ts) so it applies before the first
 * paint. This module removes it, which fades the page in.
 *
 * An attribute rather than a class because Docusaurus manages <html class>
 * through Helmet and replaces it on render, wiping anything set beforehand.
 *
 * Why not just swap fonts and let the text reflow? Geist is ~10% narrower than
 * the system fallback, so the hero headline lays out on a different number of
 * lines until the font arrives, and visibly re-breaks when it does.
 *
 * The inline script also carries its own timeout, so a font that never loads,
 * or a failure in this module, still ends with a visible page.
 */
function reveal() {
  document.documentElement.removeAttribute('data-rd-boot');
}

// No onRouteDidUpdate hook: Docusaurus fires it on the FIRST render as well as
// on navigation, so revealing there defeated the whole guard. The attribute is
// set once by the head script and removed once below - later routes reuse the
// already-loaded fonts and have nothing to wait for.

if (typeof document !== 'undefined') {
  if (document.fonts?.load) {
    // `document.fonts.ready` alone is not enough: early in the document there
    // are no pending font loads, so it resolves immediately and the page is
    // revealed before Geist has arrived. `load()` explicitly requests each
    // face and resolves once it is actually usable.
    Promise.all([
      document.fonts.load('800 3rem Geist'),
      document.fonts.load('400 1rem "Geist Mono"'),
    ])
      .then(() => document.fonts.ready)
      .then(reveal, reveal);

    // Belt and braces: never leave the page hidden on a slow or failed load.
    setTimeout(reveal, 2000);
  } else {
    reveal();
  }
}
