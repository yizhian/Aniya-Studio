---
name: grapesjs-html-compliance
description: GrapesJS editor compatibility rules for HTML slide generation. Use when generating or editing HTML that will be loaded into the GrapesJS editor — covers canvas constraints, component model, script/style handling rules, and a mandatory self-check checklist. NOT for standalone browser HTML, server-rendered pages, or non-GrapesJS contexts.
---

# GrapesJS HTML Compliance

HTML generated for GrapesJS runs inside an isolated iframe with a fixed 1920x1080 canvas. The editor manages slide visibility via CssComposer, strips `<script>` tags outside custom-code blocks, and extracts `<style>` to its style manager. This skill defines the constraints that keep generated HTML compatible with this environment.

---

## Part 0: Compliant HTML Skeleton

Start every deck from this minimal, correct skeleton. Customize styles and content; keep the structure intact.

```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<style>
  /* Navigation control styles (counter, dots, progress bar, buttons) */
  /* Font family declarations — aniyaFonts auto-loads from Google Fonts */
  body { margin: 0; padding: 0; background: #000; }
  #nav-controls { position: absolute; bottom: 20px; right: 20px; z-index: 10; }
</style>
</head>
<body>

<section class="slide active" style="position:absolute;top:0;left:0;width:1920px;height:1080px;overflow:hidden;box-sizing:border-box;background:#fff;">
  <!-- Slide 1 content -->
</section>

<section class="slide" style="position:absolute;top:0;left:0;width:1920px;height:1080px;overflow:hidden;box-sizing:border-box;background:#fff;">
  <!-- Slide 2 content -->
</section>

<div data-gjs-type="custom-code">
<script>
(function() {
  // Keyboard navigation (← → arrows, Space to advance)
  // Click/tap on left/right edges as fallback
  // Slide counter (current / total)
  // Progress bar and dot indicators
  // localStorage persistence for current slide
})();
</script>
</div>

</body>
</html>
```

Key points:
- First `.slide` hardcodes `class="active"` — not added by JS
- All slides use explicit `width: 1920px; height: 1080px; overflow: hidden`
- All `<script>` inside `<div data-gjs-type="custom-code">`
- Opacity/transform-based slide transitions are allowed (they work in preview).
  The platform's scoped shell handles editor slide visibility.
- No `position: fixed` — chrome elements use `position: absolute` inside the
  1920×1080 stage. Combined with scaleDeck() this is functionally equivalent
  to fixed positioning in preview. (S6 enforced.)
- No viewport units (`vw`/`vh`) for layout containers.
- No `@import` or `<link>` for fonts — the aniyaFonts plugin auto-loads from
  Google Fonts. The platform injects font `<link>` tags during serialization.

---

## Part 0b: Using Design Skills

Design skills are **design directories, not code templates**. When a design skill is loaded, extract its design DNA and apply it to the compliant skeleton above. The compliance skill has authority over design skills when they conflict.

### Extract (keep these)

1. **Color tokens**: All `:root` CSS custom properties and their hex values. Note accent constraints (e.g., "single accent under 5% per slide").
2. **Typography system**: Font family names and role assignments (display/body/mono). The aniyaFonts plugin auto-loads any Google Font declared via `font-family`. Extract `font-family` values (e.g., `'Newsreader'`, `'JetBrains Mono'`) — these are safe. Also extract weight constraints, italic/uppercase rules, type scale ratios, letter-spacing values.
3. **Layout archetypes**: The slide type taxonomy and internal compositions (grid ratios, card structures, metric layouts, decorative elements).
4. **Visual style rules**: Constraints and banned patterns (no border-radius, no shadows, no gradients, single accent, monospace-only body, etc.).
5. **Content structure**: Deck rhythm rules, slide ordering conventions, slide count guidance.

### Editor Canvas Constraints (these break in the GrapesJS editor iframe)

1. **`position: fixed`** — relative to iframe viewport, not 1920×1080 canvas.
   Use `position: absolute` inside `.slide` or 1920px stage. Combined with
   scaleDeck(), this is functionally identical to fixed positioning in preview.
   (S6 enforced.)
2. **Viewport units (`vw`/`vh`)** — iframe viewport ≠ canvas. Use px for
   `.slide` containers. Fluid typography (`clamp()`) inside slides is acceptable.
3. **Scripts execute only in preview** — custom-code blocks show placeholder
   in editor canvas. All scripts MUST be inside `<div data-gjs-type="custom-code">`.

### Publish Document Capabilities (allowed in the full HTML)

1. **Slide visibility** — opacity/transform transitions work in preview/export.
   The editor's scoped shell CSS does not affect the published document.
2. **`position: absolute` chrome** — nav bars, dots, progress bars placed with
   `position: absolute` inside the 1920px stage work correctly in both editor
   and preview. Combined with scaleDeck() they behave like fixed elements.
   Note: `position: fixed` is blocked by S6 — use absolute instead.
3. **`scaleDeck()`** — include in custom-code for viewport-adaptive scaling.
4. **Font loading** — declare via `font-family` only. The platform injects
   Google Fonts `<link>` tags during serialization. Do NOT write `<link>` tags
   yourself.

### Discard (these break in GrapesJS)

1. **`position: fixed`** — blocked by S6; use `position: absolute` instead.
2. **Font LOADING mechanisms** (but KEEP font NAMES):
   - Strip: `@import url('https://fonts.googleapis.com/...')` — blocked by S7
   - Strip: `<link href="https://fonts.googleapis.com/...">` — agent must not write
   - Strip: `<link rel="preconnect">` for font domains — blocked by S7
   - Strip: `@font-face { src: url('https://...') }` — blocked by S7
   - Keep: `font-family: 'Font Name', fallback` — aniyaFonts auto-loads from Google Fonts
   - Keep: System font stacks (they need no loading) — `-apple-system, BlinkMacSystemFont, ...`
3. **Other external resources** — `<img src="https://...">`, `<script src="https://...">`, `url('https://...')` in CSS (except `data:` URIs which are safe). Platform-injected Google Fonts links are exempt.
4. **`pointer-events: none`** and **`overflow: hidden` on body/html**

### Apply

Build each slide from the compliant skeleton, styled with the extracted design DNA. Use px-based dimensions on `.slide` containers. Chrome elements (page numbers, brand marks, slide labels) are part of slide content, not fixed overlays.

---

## Part 1: Architecture Overview

GrapesJS renders content in an **isolated iframe** with a fixed **1920x1080** canvas. Key facts:

- The editor sets `overflow: hidden !important` on `<html>` and `<body>` — no body-level scrolling
- Slide visibility in the editor canvas is managed by a scoped shell CSS rule (`html[data-aniya-editor] .slide:not(.active) { display: none !important; }`). In published documents (which lack `data-aniya-editor`), the author's own visibility rules (opacity, transform) take effect.
- `<script>` tags are stripped on import unless inside `<div data-gjs-type="custom-code">`
- `<style>` tags are extracted to CssComposer on import
- Custom components use `data-gjs-type` attribute (`data-gjs-type="custom-code"`)
- Charts: layout charts (KPIs, bar comparisons, data grids, progress bars) use pure div + CSS; complex geometry (pie, radar, line) uses inline `<svg viewBox="...">`
- No bare `<table>` tags — `<td>` elements get split into individually-selectable components; use div grid instead

For detailed architecture reference, see `references/grapesjs-architecture.md`.

---

## Part 2: Mandatory Constraints

### 2.1 Structure

1. **Canvas size**: All `.slide` must have explicit `width: 1920px; height: 1080px`
2. **No `position: fixed`**: Fixed positioning is relative to the iframe viewport, not the canvas. Use `position: absolute` inside `.slide` or 1920px stage instead. Combined with scaleDeck(), this is functionally identical to fixed positioning in preview. (S6 enforced.)
3. **Viewport containers use `position: relative`**: `#stage`, `#presentation`, `#scale-wrap` must use `position: relative`
4. **Explicit `.slide` structure**: Every slide is `<section class="slide" style="position:absolute;top:0;left:0;width:1920px;height:1080px;overflow:hidden;box-sizing:border-box">`
5. **First slide hardcodes `active`**: The first `.slide` element in HTML source must include `class="... active"` (e.g., `<section class="slide active">`). Do not rely on JS to add it — custom-code scripts do not execute in the editor canvas.
6. **Slide overflow**: All `.slide` must use `overflow: hidden` (not `overflow-y: auto` or `scroll`)

### 2.2 Scripts

7. **Scripts in custom-code only**: All `<script>` tags must be inside `<div data-gjs-type="custom-code">` — otherwise stripped on import
8. **Charts: div+CSS or inline SVG**:
   - Layout charts (KPIs, bar comparisons, data grids, progress bars) → pure div + CSS (flex/grid + inline width%)
   - Data tables → div grid (no bare `<table>` — `<td>` gets split into separate components)
   - Complex geometry (pie, radar, line) → inline `<svg viewBox="...">`
   - Chart containers should use `class="chart"` for easy selection
9. **No inline event handlers**: `onclick`, `onload`, `onerror`, `onmouseover` etc. are stripped by the editor

### 2.3 Styles

10. **Prefer inline styles**: Use element `style` attributes — most reliable
11. **`<style>` in `<head>`**: All style rules in `<head>` `<style>` blocks — extracted to CssComposer on import. Do NOT define `.slide:not(.active) { display: none !important; }` — the platform provides the scoped equivalent. Opacity/transform-based visibility rules are acceptable.
12. **No author-level `!important`**: Generates thousands of duplicate rules on re-render. The platform's scoped shell (`html[data-aniya-editor] .slide:not(.active) { display: none !important; }`) is a platform-reserved rule — do NOT repeat or override it in your HTML.
13. **No CSS-in-JS**: Silently fails in the iframe (styled-components, emotion, etc.)

### 2.4 SVG

14. **SVG in containers**: Wrap each chart in a container div (e.g., `<div class="chart">`) for easy selection
15. **SVG uses `viewBox`**: All `<svg>` elements use `viewBox`, not fixed pixel dimensions
16. **SVG uses native elements only**: `<circle>`, `<path>`, `<polygon>`, `<text>` — no HTML divs inside SVG

### 2.5 Resources

17. **No external CDN resources**: No `http://` or `https://` URLs in `src`, `href`, `url()`, or `@import`. Exception: `data:` URIs are safe. Platform-injected Google Fonts links (`fonts.googleapis.com`, `fonts.gstatic.com`) are exempt — the Agent must NOT write these itself.
18. **Fonts via `font-family` only**: Any Google Font works — declare with `font-family: 'Font Name', fallback` in CSS. The aniyaFonts plugin auto-loads them. Do NOT use `@import url()` or `<link>` tags to load fonts. `.slidecraft/available_fonts.json` is a convenience reference, not a whitelist.

### 2.6 Five Universal Failure Patterns

These patterns appear in design skill examples and cause failures in GrapesJS:

| # | Pattern | Why It Fails | Correct Approach |
|---|---------|-------------|-----------------|
| 1 | Viewport units (`vw`, `vh`, `%`) | iframe viewport ≠ 1920x1080 canvas | px-based dimensions on `.slide` containers |
| 2 | Custom visibility (`opacity:0`, `visibility:hidden`) | In editor: display:none overrides it via scoped shell. In publish: transitions work. | Default to `opacity:1`; animate only on `:hover` or in preview. Opacity/transform transitions on `.slide` are allowed. |
| 3 | Zero-dimension wrappers with `overflow:hidden` | Absolutely-positioned children don't expand parent | Explicit `width`/`height` on all wrapper containers |
| 4 | Animation starting at `opacity:0` | Animations don't run in editor mode, content stays invisible | Default to `opacity:1`; animate only on `:hover` or in preview |
| 5 | Flex children with percentage heights | Undefined height on flex container collapses children | Use explicit px heights or flex ratios instead of percentages |

---

## Part 3: Self-Check Checklist

After generating HTML, check every item:

- [ ] Starts with `<!DOCTYPE html>`
- [ ] `<html>` and `<body>` tags present and complete
- [ ] All `.slide` have explicit `width: 1920px; height: 1080px`
- [ ] All `.slide` use `overflow: hidden` (not `overflow-y: auto` or `scroll`)
- [ ] All `<script>` inside `<div data-gjs-type="custom-code">`
- [ ] First `.slide` element includes `class="slide active"` (hardcoded, not from JS)
- [ ] No bare `<table>` tags (use div grid for table layouts)
- [ ] Charts use div+CSS (layout) or inline SVG (complex geometry); SVG uses `viewBox` and is wrapped in a container div
- [ ] Viewport containers (`#stage`/`#presentation`/`#scale-wrap`) use `position: relative`
- [ ] No `position: fixed` (S6 enforced — use `position: absolute` instead)
- [ ] No external resource URLs (check all `src`, `href` attributes; fonts via `font-family` declaration only). Platform-injected Google Fonts links are exempt.
- [ ] No inline event handlers (`onclick`/`onload`/`onerror` etc.)
- [ ] No unclosed tags (`<div>`/`</div>` count matches, etc.)
- [ ] No unclosed CSS comments `/* ...` (will freeze the editor)
- [ ] No `.slide:not(.active) { display: none !important; }` — platform provides scoped equivalent. Opacity/transform visibility rules (for publish mode transitions) are allowed.

---

## Part 4: Common Pitfalls

| Pitfall | Consequence | Correct Approach |
|---------|------------|-----------------|
| `<script>` outside custom-code | Scripts stripped on import | Wrap in `<div data-gjs-type="custom-code">` |
| `<style>` in custom-code | Normalized to CssComposer on import (fallback; don't rely on it) | Place styles in `<head>` `<style>` |
| `position: fixed` | Element positioned relative to iframe viewport, not canvas (S6 enforced) | Use `position: absolute` relative to `.slide` or 1920px stage. scaleDeck() makes this visually identical to fixed in preview. |
| Unclosed CSS comment `/*` | **Editor freezes** | Ensure every `/*` has a matching `*/` |
| Repeated `!important` | Thousands of duplicate CSS rules generated | Use more specific selectors; platform scoped shell `html[data-aniya-editor] .slide:not(.active){display:none!important}` is reserved |
| Chart container without explicit size | Chart renders at 0×0 | Set explicit `width` and `height` on container |
| CSS-in-JS | Silently fails in iframe | Use inline `style` or `<head>` `<style>` |
| External CDN references | Resource load failure (S7 enforced) | Inline all resources (SVG, data: URIs); fonts via `font-family`. Platform-injected Google Fonts links are exempt. |
| `<iframe>` elements | Known custom-code plugin bug | Avoid `<iframe>` |
| `on*` event attributes | Stripped by editor | Use `addEventListener` in custom-code scripts |
| Bare `<table>` tags | `<td>` split into separate selectable components | Use div grid for table layouts |
| First slide active via JS | Editor canvas shows blank | Hardcode `class="slide active"` in HTML source |
| `opacity:0` on `.slide` | In editor: display:none overrides it via scoped shell. In publish: transitions work. | Default to `opacity:1`; animate only on `:hover` or in preview. Opacity/transform transitions on `.slide` are allowed. |
| `write_file` to fix violations | `file_exists` error, destroys editor state | Always use `read_file` then `edit_file` |

---

## Part 5: Mandatory Audit Workflow

After generating or editing HTML, follow this exact sequence. Do NOT skip steps.

1. **Read**: `read_file` the HTML file to get current content and `read_mtime_unix_ns`.
2. **Check**: Go through every item in Part 3's self-check checklist against the file content.
3. **Record**: Note each violation with its line number and the fix needed.
4. **Fix**: Use `edit_file` to fix one violation at a time. CRITICAL: Never use `write_file` on an existing file — it will fail with `file_exists` and destroys GrapesJS editor state.
5. **Re-verify**: After fixing, the platform re-checks automatically. Repeat the read_file → edit_file cycle until the checklist is fully clean.

**Gate**: Do not declare the task complete until all 15 checklist items pass.

---

## When NOT to Apply This Skill

- **Standalone browser HTML**: If the output is viewed directly in a browser (not GrapesJS), use standard web practices instead.
- **Print-only decks**: If the output is for print/PDF, viewport constraints and script rules are irrelevant.
- **Non-HTML output**: This skill only applies to `.html` files destined for GrapesJS import.
