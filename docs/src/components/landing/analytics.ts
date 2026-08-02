/**
 * Click tracking for the landing page.
 *
 * Two event names, not one: GA4 marks key events (conversions) by event name
 * alone - a parameter cannot define one - so the clicks worth counting as
 * conversions need their own name. Everything else shares `landing_click` and
 * is told apart by its parameters.
 *
 * The parameters only become reportable once they are registered as custom
 * dimensions in GA4 (Admin -> Custom definitions), and they populate from that
 * point on: registering later does not backfill.
 */

/** Which section of the page the click happened in. */
export type ClickSection =
  | 'hero'
  | 'final_cta'
  | 'checks'
  | 'compatibility'
  | 'replaces'
  | 'toolkit'
  | 'faq'
  | 'testimonials';

/**
 * What kind of thing was clicked. `cta` and `copy_command` are the conversion
 * intents, so they are the two that report as `cta_click`.
 */
export type ClickType =
  | 'cta'
  | 'copy_command'
  | 'docs_link'
  | 'comparison_link'
  | 'toolkit_tab'
  | 'faq_toggle'
  | 'external_link';

export type ClickEvent = {
  /** Stable slug for this specific element - keep it stable across copy edits. */
  id: string;
  section: ClickSection;
  type: ClickType;
  /** Href, or the copied command for the install button. */
  target: string;
};

const CONVERSION_TYPES: ReadonlySet<ClickType> = new Set<ClickType>(['cta', 'copy_command']);

/** GA4 silently truncates parameter values at 100 characters; do it here so
    what lands in the report is what we chose to send. */
const MAX_PARAM_LENGTH = 100;

/**
 * Fire and forget - never blocks or delays the navigation that triggered it.
 *
 * gtag.js sends GA4 hits with `navigator.sendBeacon`, so events survive the
 * page unload that follows a click on an external link; there is nothing to
 * await and no `transport_type` to set.
 */
export function trackClick({ id, section, type, target }: ClickEvent): void {
  // `gtagFallback.ts` installs a no-op `window.gtag` when the real script is
  // absent or blocked, so this guard is only about server-side rendering.
  if (typeof window === 'undefined' || typeof window.gtag !== 'function') return;

  window.gtag('event', CONVERSION_TYPES.has(type) ? 'cta_click' : 'landing_click', {
    click_id: id,
    click_section: section,
    click_type: type,
    click_target: target.slice(0, MAX_PARAM_LENGTH),
  });
}
