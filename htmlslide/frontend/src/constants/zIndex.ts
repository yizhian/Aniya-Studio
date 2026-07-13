/**
 * Single source of truth for the app's z-index layering.
 *
 * Overlays are rendered through `Portal` (mounted directly under
 * `document.body`), so within each tier, later-mounted elements simply
 * paint above earlier ones — these tiers only need to express relative
 * *priority* between different kinds of UI, not exact stacking order.
 *
 * Tiers, low to high:
 * - CHROME: always-visible background controls (locale switch, theme
 *   toggle, settings entry point). Must never outrank a real overlay.
 * - DOCK: panels docked inline within a view's own flex layout (e.g. the
 *   advanced editor's right inspector dock). Not portaled — they live in
 *   the page's normal layout flow.
 * - OVERLAY_BACKDROP / OVERLAY: modals, drawers, and preview panels that
 *   must sit above all page chrome.
 * - FULLSCREEN: the true fullscreen live-preview, which must win over
 *   any other overlay that might still be closing.
 * - CRITICAL: blocking system dialogs (e.g. unsaved-changes confirm)
 *   that must win over every other overlay, including FULLSCREEN.
 */
export const Z_INDEX = {
  CHROME: 20,
  DOCK: 30,
  OVERLAY_BACKDROP: 90,
  OVERLAY: 95,
  FULLSCREEN: 100,
  CRITICAL: 105,
} as const;
