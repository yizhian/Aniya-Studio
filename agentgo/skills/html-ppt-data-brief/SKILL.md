---
name: html-ppt-data-brief
description: A formal, print-inspired slide system for data-rich geopolitical or strategic intelligence briefings. Uses warm parchment backgrounds, serif typography (Georgia/Palatino), and a restrained palette of deep navy, crimson, and gold to convey authority without aggression. Best for US-China comparisons, policy decks, think-tank presentations, and any scenario requiring sober, evidence-driven storytelling.
whentouse: ""
type: ""
triggers:
    - geopolitical
    - think-tank
    - strategic intelligence
    - US-China
    - policy briefing
    - data-driven diplomacy
od:
    mode: deck
    scenario: tech-sharing
    upstream: ""
    upstream_license: ""
    speaker_notes: false
    animations: false
    preview:
        type: html
        entry: example.html
    design_system:
        requires: false
---

# Geopolitical Data Briefing

## Design Overview

This design system evokes a printed strategic briefing document projected onto a screen. The warm parchment background (#f5f3ef) reduces glare compared to pure white, making it comfortable for conference-room projection and long reading sessions. Typography is rooted in editorial tradition — Georgia for authoritative headlines, Palatino for readable body copy, and Courier New for data labels — creating a "research report" atmosphere. The three-accent palette (navy for the US, crimson for China, gold for insight callouts) introduces controlled color only where it carries semantic weight, keeping the overall impression sober, credible, and intellectually rigorous.

## Hard Rules

### Canvas
- Slide dimensions: 1920×1080px, scaling responsively via CSS transform
- Background: **#f5f3ef** — a warm parchment off-white that prevents eye strain on projected screens (pure #FFFFFF creates glare on large displays)
- Slide padding: **56px top/bottom, 72px left/right** on all content slides except the title slide

### Color Palette (Exact Values)
| Role | Hex | Usage |
|------|-----|-------|
| Page background | `#f5f3ef` | All slide backgrounds |
| Card/container background | `#faf9f7` | Cards, stat boxes, source items — slightly lighter than page for subtle elevation |
| Primary text | `#1a1a1a` | Headlines, body text, KPI values |
| Secondary text | `#5c5c5c` | Subtitles, descriptions, chart labels |
| Muted text | `#7a7a7a` | Meta labels, axis labels, section overlines |
| Faded text | `#b0aaa0` | Slide numbers, inactive dots, tertiary chrome |
| Border / divider | `#e0ddd6` | Card borders, section dividers, timeline lines, bar tracks |
| Subtle inner border | `#eeeae4` | Row separators within cards and lists |
| Navy accent (US) | `#1a3a5c` | US bar fills, US section headers, US timeline dots, strong text highlights, navigation hover |
| Crimson accent (China) | `#8b1a1a` | China bar fills, China section headers, China timeline dots, secondary emphasis |
| Gold accent (insight) | `#d4a84b` | Key insight callouts, the title slide decorative line, progress bar fill, takeaway numbers, data highlights |
| Bar track background | `#e8e4dc` | Unfilled portion of bar charts (slightly darker than page to give the track definition) |
| Mini-bar neutral | `#d0cdc6` | Placeholder bar segments in mini-charts |
| Inactive dot | `#d0cdc6` | Navigation dot default state |
| Arrow between flow nodes | `#c0bbb2` | Flow diagram connector arrows |

### Typography System

| Element | Font Stack | Size | Weight | Letter-Spacing | Line-Height | Case |
|---------|-----------|------|--------|----------------|-------------|------|
| Slide title (h1, title slide) | Georgia, Times New Roman, serif | 82px | 400 | -0.03em | 1.15 | Normal |
| Slide title (h1, closing slide) | Georgia, Times New Roman, serif | 52px | 400 | -0.02em | 1.15 | Normal |
| Default h1 (other slides) | Georgia, Times New Roman, serif | 68px | 400 | -0.02em | 1.15 | Normal |
| Section heading (h2) | Georgia, Times New Roman, serif | 44px | 400 | -0.01em | — | Normal |
| Sub-heading (h3) | Georgia, Times New Roman, serif | 32px | 400 | — | — | Normal |
| Card title | Georgia, Times New Roman, serif | 22px | 400 | — | — | Normal |
| Overline label (h4) | Segoe UI, sans-serif | 26px | 500 | 0.02em | — | UPPERCASE |
| Subtitle / body | Palatino, Times New Roman, serif | 28px | 400 | — | 1.4 | Normal |
| Slide header description | Palatino, Times New Roman, serif | 22px | 400 | — | — | Normal |
| Timeline content | Palatino, Times New Roman, serif | 20px | 400 | — | 1.4 | Normal |
| KPI value | Georgia, Times New Roman, serif | 52px | 400 | — | 1.0 | Normal |
| KPI label | Segoe UI, sans-serif | 18px | 400 | 0.05em | — | UPPERCASE |
| KPI sub-label | Palatino, Times New Roman, serif | 20px | 400 | — | — | Normal |
| Bar label | Segoe UI, sans-serif | 18px | 400 | — | — | Normal |
| Bar value | Courier New, monospace | 16px | 400 | — | — | Normal |
| Slide number | Courier New, monospace | 18px | 400 | 0.05em | — | Normal |
| Title meta | Courier New, monospace | 18px | 400 | 0.06em | — | UPPERCASE |
| Timeline year | Courier New, monospace | 20px | 700 | — | — | Normal |
| Comparison header | Georgia, Times New Roman, serif | 28px | 400 | — | — | Normal |
| Comparison row label | Segoe UI, sans-serif | 20px | 400 | — | — | Normal |
| Comparison row value | Georgia, Times New Roman, serif | 24px | 700 | — | — | Normal |
| Takeaway number | Georgia, Times New Roman, serif | 36px | 400 | — | 1.0 | Normal |
| Takeaway text | Palatino, Times New Roman, serif | 24px | 400 | — | 1.4 | Normal |
| Source item | Courier New, monospace | 14px | 400 | — | — | Normal |
| Chart title | Segoe UI, sans-serif | 16px | 400 | 0.05em | — | UPPERCASE |
| Flow node description | Courier New, monospace | 18px | 400 | — | — | Normal |
| Small insight label | Courier New, monospace | 13px | 400 | 0.06em | — | UPPERCASE |

### Font Role Summary
- **Georgia** — All headlines, KPI values, comparison values, takeaway numbers, comparison headers, card titles. Conveys authority and editorial weight.
- **Palatino** — All body copy, subtitles, descriptions, timeline content. Provides readability with a slightly humanist warmth.
- **Segoe UI** — All labels, overlines, chart titles, KPI labels, bar labels, comparison row labels. The sole sans-serif provides structural contrast and legibility at small sizes.
- **Courier New** — All data values (bar values, slide numbers, source items, timeline years, title meta). The monospace font signals data precision and evokes research-report aesthetics.

### Spacing & Layout
- Content slides use a **slide-header + slide-body** pattern with a 1px `#e0ddd6` border separating them
- Slide header bottom border has **20px padding-bottom** and **28px margin-bottom**
- Slide body uses CSS Grid with **24px gap** as the default rhythm unit
- Cards use **28px 32px** internal padding with **0 border-radius** (sharp corners signal seriousness)
- Grid helpers: `.grid-2` (1fr 1fr, 24px gap), `.grid-3` (1fr 1fr 1fr, 24px gap), `.grid-4` (1fr 1fr 1fr 1fr, 20px gap), `.grid-12` (repeat 12, 20px gap)
- The thin rule divider (`.rule-thin`) is **1px #e0ddd6** used between sections
- Margin utility classes: `.mt-8`, `.mt-16`, `.mt-24`, `.mb-8`, `.mb-16`

### Cards
- Background: `#faf9f7` (slightly lighter than page)
- Border: 1px solid `#e0ddd6`
- Border-radius: **0** (never rounded — sharp corners are a rule of this system)
- Padding: 28px 32px

### Bar Charts
- Bar track: height 20px, background `#e8e4dc`, no border-radius (standard bar) or 4px border-radius (patent bars)
- Bar fill colors: `.us` = `#1a3a5c`, `.cn` = `#8b1a1a`, `.gold` = `#d4a84b`, neutral = `#b0aaa0`
- Bar row grid: 100px label / 1fr track / 80px value (with 12px gap), or compact 40px / 1fr / 40px for sector bars
- Bar labels: Segoe UI 18px `#5c5c5c`, right-aligned
- Bar values: Courier New 16px `#1a1a1a`, right-aligned
- Transition: `width 0.6s ease` (standard) or `0.8s ease` (patent bars)

### Mini Bar Charts (inside KPI cards)
- Container: display flex, align-items flex-end, height 80px, gap 4px
- Individual bars: flex 1, background `#e0ddd6`, border-radius 2px 2px 0 0 (slight rounding to distinguish from full charts)
- Semantic colors applied via `.us`, `.cn`, `.gold` classes

### Navigation Chrome
- Progress bar: fixed top, 3px height, track `#e0ddd6`, fill `#d4a84b`
- Navigation buttons: fixed bottom, 48×48px, background `#faf9f7`, border 1px `#e0ddd6`, Courier New 22px
- Button hover: background becomes `#1a3a5c`, text becomes white, border becomes `#1a3a5c`
- Dot indicators: 10px circles, default `#d0cdc6`, active `#1a3a5c` (expands to 28px wide pill), hover `#b0aaa0`
- Slide counter: fixed bottom center, Courier New 16px, `#7a7a7a`
- Edge click zones: fixed left/right 15% width transparent overlays for tap navigation

### Animation
- Fade-in animation: `fadeIn` keyframe — translates 12px up while fading opacity 0→1, 0.5s duration, ease timing
- Staggered delays: `.anim-delay-1` through `.anim-delay-5` (0.1s increments)
- Bar fill transitions: 0.6s ease (standard), 0.8s ease (patent bars)

## Banned

1. **Never use rounded corners on cards, containers, or major UI elements.** Sharp 0px border-radius is a defining characteristic. The sole exception is mini-bar tops (2px) and navigation dots/pills, which are UI chrome, not content containers.
2. **Never use pure white (#FFFFFF) or pure black (#000000) as content surface colors.** The system lives in warm off-whites. Pure black is only used for the HTML/body background behind the scaled presentation wrapper — never inside slides.
3. **Never use font-weights above 400 on Georgia headlines.** The system achieves hierarchy through size and letter-spacing, not boldness. Weight 700 appears only on Courier New timeline years, comparison row values, and bar-label value spans — never on Georgia or Palatino headings.
4. **Never introduce colors outside the defined palette.** The five semantic colors (#1a3a5c navy, #8b1a1a crimson, #d4a84b gold, #e0ddd6 border, #f5f3ef background) and their neutral derivatives form a closed system. No blues other than navy, no reds other than crimson, no greens, no purples.
5. **Never center-align body text or card content.** Text alignment is left by default. The only centered elements are: KPI cards, the closing slide, flow nodes, and compute stats — all of which are small contained modules, not running text.
6. **Never use sans-serif for body copy or headlines.** Segoe UI is strictly for labels, overlines, chart titles, and UI chrome. All readable prose must be Georgia or Palatino.

## Slide Archetypes

### Archetype 1 — Title Slide (Slide 1)
- **Structure**: Full-bleed parchment canvas with a 6px navy bar across the top edge. Content is vertically centered in a flex column with 80px/100px padding.
- **Elements (in order)**:
  - Gold decorative line: 80px wide, 3px tall, `#d4a84b`, margin-bottom 32px
  - Headline: Georgia 82px, weight 400, letter-spacing -0.03em, max-width 80%
  - Subtitle: Palatino 30px, `#5c5c5c`, max-width 65%
  - Meta row: horizontal flex with 48px gap, Courier New 18px uppercase `#7a7a7a`, each item preceded by a gold `▸` pseudo-element
  - Navigation hint: positioned absolute bottom-right, Courier New 16px `#7a7a7a`
- **Key rule**: The title slide has **no slide-header component** and **padding: 0** (internal padding handled by `.slide-title`)

### Archetype 2 — KPI Dashboard Grid (Slide 2)
- **Structure**: Slide-header + body with `.grid-4` KPI cards, a thin rule divider, then a `.grid-12` chart layout
- **KPI card internals**:
  - Large value: Georgia 52px `#1a1a1a`
  - Label: Segoe UI 18px uppercase `#7a7a7a`, letter-spacing 0.05em
  - Sub-label: Palatino 20px `#5c5c5c`
  - Mini bar chart: flex row of `.mini-bar` elements, height 80px, with semantic color classes
- **Chart areas**: span 4 or 8 columns in the 12-column grid, background `#f5f3ef`, border 1px `#e0ddd6`, 24px padding, with an uppercase Segoe UI chart title
- **Legend row**: centered flex with colored 12px squares + Segoe UI 14px labels

### Archetype 3 — Side-by-Side Comparison (Slide 3)
- **Structure**: Slide-header + two-column grid (`.comparison-grid`, 1fr 1fr, gap 0) + optional insight callout below
- **Column internals**:
  - Header: Georgia 28px with a 2px bottom border — `#1a3a5c` for US, `#8b1a1a` for China
  - Rows: flexbox space-between, border-bottom 1px `#eeeae4`
  - Row label: Segoe UI 20px `#5c5c5c`
  - Row value: Georgia 24px weight 700 in the column's accent color
  - Last row has no bottom border
- **Insight callout**: a flex row below the rule with a box that has a 3px left gold border (`#d4a84b`), background `#eae7e0`, Segoe UI 16px text
- **Key rule**: The two columns touch with a 1px `#e0ddd6` divider on the first column's right border; no gap between columns

### Archetype 4 — Dual Timeline (Slide 4)
- **Structure**: Slide-header + two-column grid of cards, each containing a vertical timeline
- **Timeline internals**:
  - Vertical line: 2px `#e0ddd6` pseudo-element at left: 60px
  - Timeline items: grid 80px / 1fr, 24px gap, 32px margin-bottom
  - Year: Courier New 20px weight 700, `#1a3a5c`, right-aligned
  - Dot: absolute positioned at left: 54px, top: 8px, 14px circle, background `#1a3a5c` (or `#8b1a1a` for `.cn`), with a 3px `#f5f3ef` border that creates a "cutout" effect against the timeline line
  - Content: Palatino 20px `#5c5c5c`, with `<strong>` in `#1a1a1a`
- **Below**: Optional KPI summary row in `.grid-3` with smaller font variants (Courier New 32px for values)

### Archetype 5 — Patent/Research Bar Comparison (Slide 5)
- **Structure**: Slide-header + two-column patent row + insight callout boxes + bottom summary grid
- **Patent column internals**:
  - Column heading: Georgia 26px (overriding default h3 size)
  - Thin rule divider
  - Bar groups with labels (Segoe UI 16px, space-between) and rounded bar tracks (8px height, 4px border-radius)
  - Bar fills use semantic colors for US/CN, neutral `#b0aaa0` for other nations
- **Insight callout boxes**: flex row with icon + stat + description, background `#faf9f7`, border 1px `#e0ddd6`, 16px 24px padding, 16px gap
  - Stat number: Georgia 44px in accent color
  - Label: Segoe UI 16px
  - Description: Palatino 15px `#7a7a7a`
- **Bottom row**: `.grid-3` with overline + Georgia 28px headline + Palatino 16px description, each separated by a 2px top border

### Archetype 6 — Flow Diagram (Slide 6)
- **Structure**: Slide-header + flow diagram + rule + three-column grid
- **Flow diagram**: three columns (1fr 1fr 1fr), nodes with arrows between
  - Node: centered text, 20px padding, background `#faf9f7`, border 1px `#e0ddd6`
  - Node heading: Georgia 26px in accent color
  - Node stat: Courier New 18px `#5c5c5c`
  - Arrow: 32px `#c0bbb2` between nodes
- **Below**: `.grid-3` cards with card titles and patent-style bar breakdowns

### Archetype 7 — Sector Grid (Slide 7)
- **Structure**: Slide-header + two-column sector grid
- **Sector card**: 20px 28px padding, 3px left border (navy for US-led sectors, crimson for China-led sectors)
  - Heading: Georgia 26px in the border's accent color
  - Description: Palatino 18px `#5c5c5c`
  - Compact bar chart: 40px/1fr/40px grid for label/track/value
- **Footer note**: centered Segoe UI 14px `#7a7a7a` below the grid

### Archetype 8 — Takeaways + Quote (Slide 9)
- **Structure**: Slide-header + two-column takeaway lists + dark quote banner
- **Takeaway list**:
  - Items: 48px / 1fr grid, 18px vertical padding, border-bottom 1px `#eeeae4`
  - Number: Georgia 36px `#d4a84b` (gold)
  - Text: Palatino 24px `#1a1a1a`, with `<strong>` in `#1a3a5c`
- **Quote banner**: full-width, background `#1a1a1a` (the single exception to the no-pure-black rule — used as a dramatic accent surface), 20px 32px padding, flex space-between
  - Quote: Georgia 22px `#d4a84b` (gold on dark)
  - Attribution: Segoe UI 14px `#7a7a7a`

### Archetype 9 — Compute & Infrastructure (Slide 8)
- **Structure**: Slide-header + two-column compute grid + bottom stat row
- **Compute capacity bars**: compact bar groups with Segoe UI 18px labels and 10px bar tracks
- **Supercomputer list**: unstyled list with Segoe UI 18px, flex space-between rows, 1px `#eeeae4` dividers, weight 600 values
- **Bottom stat cards**: `.grid-3` with large Courier New 28px gold numbers + Segoe UI 15px descriptions, in `#faf9f7` bordered containers

### Archetype 10 — Closing / Sources (Slide 10)
- **Structure**: Slide-header + centered flex column body
- **Body**: flex column, center-aligned, 24px gap
  - Thank-you headline: Georgia 52px `#1a1a1a`
  - Subtitle: Palatino 24px `#5c5c5c`
  - Source grid: two-column grid, max-width 70%, 16px gap
  - Source items: Courier New 14px `#7a7a7a`, background `#faf9f7`, border 1px `#e0ddd6`, each preceded by gold `▸` pseudo-element
  - Footer note: Courier New 15px `#7a7a7a`, centered

## Pre-flight

- [ ] All card elements have `border-radius: 0` (sharp corners) — confirm no `border-radius` declarations on `.card`, `.sector-card`, `.flow-node`, or `.source-item`
- [ ] Georgia is used for ALL headlines and KPI values — verify no sans-serif has leaked into h1, h2, h3, `.kpi-value`, `.comp-header`, `.timeline-year` (which uses Courier New, by design)
- [ ] The five core colors are the ONLY colors in use: `#f5f3ef` (page), `#faf9f7` (card), `#e0ddd6` (border), `#1a3a5c` (navy), `#8b1a1a` (crimson), `#d4a84b` (gold). No greens, no other blues, no other reds.
- [ ] Bar fills use semantic classes (`.us`, `.cn`, `.gold`) consistently — US data always maps to navy, China data always maps to crimson
- [ ] All slides have a `slide-header` component with the 1px `#e0ddd6` bottom border EXCEPT the title slide (which has no header) and the closing slide (which has one but uses centered body)
- [ ] Body text (Palatino) never exceeds 28px; labels (Segoe UI) never drop below 13px; Georgia headlines sit within the 32–82px range depending on slide archetype
- [ ] Navigation chrome (progress bar, buttons, dots, counter) is present and uses the defined chrome colors — never the semantic accent colors except for active dot and hover states
- [ ] Animation delays (`.anim-delay-1` through `.anim-delay-5`) are applied to elements that should stagger in, and only those elements
