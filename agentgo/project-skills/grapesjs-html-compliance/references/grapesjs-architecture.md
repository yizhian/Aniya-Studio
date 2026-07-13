# GrapesJS Editor Architecture

Detailed technical reference for how GrapesJS processes imported HTML. This supplements Part 1 of the compliance skill with deeper implementation detail.

## Canvas Environment

- GrapesJS renders content in an **isolated iframe** with a fixed **1920x1080 pixel** canvas
- The editor enforces `overflow: hidden !important` on `<html>` and `<body>`
- Body-level scrolling is impossible — all content must layout within `.slide` containers
- Inter font is injected as the default font in the iframe

## Component System

GrapesJS stores page structure in a **component tree**, not raw HTML:

- **Custom components** use `data-gjs-type` attribute for identification
  - `data-gjs-type="custom-code"` — protects scripts and styles from stripping
- Elements **without** the `.slide` class are automatically grouped/wrapped by the editor, which can break layouts
- Charts use `class="chart"` for easy whole-chart selection (no `data-gjs-type` chart custom components needed)

## HTML Import Pipeline (14 Steps)

When HTML is imported via `importHtmlDocument()`:

1. Parse HTML
2. Extract `<style>` blocks from `<head>`
3. Extract `<style>` blocks from custom-code components
4. Merge CSS: `headCss + customCodeCss + ANIYA_DECK_SHELL_CSS`
5. Remove all `<style>` elements from HTML
6. Remove all `<script>` elements from HTML (unless in custom-code)
7. Set components on the GrapesJS component tree
8. `fixViewportContainers()` — auto-fix `absolute` centering to `relative`
9. `normalizeToSlides()` — ensure first slide has `active` class
10. Scan for fonts via `font-family` declarations
11. Load fonts into canvas iframe

## Slide Visibility

The editor platform manages slide visibility through CssComposer:

- **Platform rule** (always injected last): `.slide:not(.active) { display: none !important; }`
- This is defined as `ANIYA_DECK_SHELL_CSS` and inserted as the final CSS rule
- Author HTML must NOT define any slide visibility rules
- The first `.slide` element in the HTML source must hardcode `class="active"`
- Navigation scripts in custom-code toggle the `active` class to show/hide slides

## Script Handling

- `<script>` tags **outside** custom-code blocks are **stripped** on import
- Scripts inside `<div data-gjs-type="custom-code">` are **preserved**
- Custom-code scripts execute in the editor canvas only during preview mode
- Inline event handlers (`onclick`, `onload`, etc.) are **always stripped**

## Style Handling

- `<style>` blocks in `<head>` are extracted to CssComposer's style manager
- Extracted styles participate in the normal CSS cascade
- `<style>` blocks inside custom-code are normalized to CssComposer on import (fallback behavior)
- Inline `style` attributes on elements are the most reliable styling method

## Font Pipeline

The aniyaFonts plugin auto-loads Google Fonts:

1. `scanHTMLForFonts()` — scans raw HTML for `font-family` declarations in `<style>` blocks and inline styles
2. `component:add` listener — captures `font-family` from GrapesJS component styles
3. `loadFont()` — creates `<link data-aniya-font>` in the canvas iframe for each font

**Key**: Any valid Google Font name in a `font-family` declaration triggers auto-loading. No `@import`, `<link>`, or `@font-face` with external URLs needed.

## Viewport Container Fixing

`fixViewportContainers()` automatically corrects common issues:
- Changes `position: absolute; top: 50%; left: 50%` to `position: relative` on viewport-level containers
- This prevents them from being centered incorrectly in the iframe

## Dead Code Note

`syncImportedHeadAssets()` copies `<link rel="preconnect">` and `<link rel="stylesheet">` from imported HTML into the canvas iframe. This is legacy code from an older font pipeline — S7 validation blocks these tags at write time, so this function is effectively dead code.
