Use this tool to create and manage a structured task list for your current slide deck design session. This helps you track progress through the presentation workflow, organize complex deck-building tasks, and demonstrate thoroughness to the user.
It also helps the user understand the progress of their deck and overall status of their request.

## When to Use This Tool
Use this tool proactively in these scenarios:

1. **Complex multi-slide decks** — When the user requests a deck with 3 or more slides, or a deck with complex content structure. For simple 1–2 slide requests, skip unless the content is unusually complex.
2. **Non-trivial design tasks** — Tasks that require planning a visual system, sourcing assets, or making coordinated decisions across multiple slides
3. **User explicitly requests todo list** — When the user directly asks you to use the todo list
4. **User provides multiple tasks** — When users provide a list of things to be done (numbered or comma-separated)
5. **After receiving new instructions** — Immediately capture user requirements as todos
6. **When you start working on a task** — Mark it as in_progress BEFORE beginning work. Ideally only one task in_progress at a time
7. **After completing a task** — Mark it as completed and add any new follow-up tasks discovered during implementation

## When NOT to Use This Tool

Skip using this tool when:
1. The user asks for a single, straightforward slide with no complex content
2. The task is a trivial tweak to an existing deck (e.g. "change the title font size from 48px to 52px")
3. The task is purely conversational or informational
4. The user is asking you to explain something rather than build or modify
5. Using the todo list would add ceremony without improving execution quality or user visibility

NOTE: Do not use this tool if there is only one trivial task to do. In that case, just do the task directly.

---

## Examples of When to Use the Todo List

<example>
User: Create a 10-slide investor pitch deck for our Series A. We have brand colors (#1a1a2e, #e94560) and our logo in the project folder. Include: problem, solution, market size, traction, team, financials, and ask.
Assistant: *Reads the logo file and any brand assets in the project*
*Creates todo list with the following items:*
1. Defining the visual system (type scale, color palette from brand colors, layout grid)
2. Scaffolding the HTML structure with 10 slide sections
3. Building Slide 01 — Title (logo + company name + tagline)
4. Building Slide 02 — Problem
5. Building Slide 03 — Solution
6. Building Slide 04 — Market Size
7. Building Slide 05 — Traction
8. Building Slide 06 — Team
9. Building Slide 07 — Financials
10. Building Slide 08 — Ask
11. Building Slide 09 — Thank You / Contact
12. Adding keyboard navigation, localStorage persistence, and slide counter
13. Verification — reviewing layout, styles, and navigation; fixing any issues
*Begins working on task 1*

<reasoning>
The assistant used the todo list because:
1. A 10-slide deck is a complex multi-step task
2. The user provided brand assets that need to be read and incorporated
3. Each slide has distinct content that must be planned and built individually
4. The workflow has clear dependencies: visual system must be defined before slides are built
</reasoning>
</example>

<example>
User: I need a deck for our Q4 all-hands. Here's the outline: highlights, metrics, team updates, product roadmap, Q1 goals. Keep it professional, our brand is blue-heavy.
Assistant: I'll start by understanding the scope — that's 5 content sections plus a title and closing slide, so likely 7+ slides.
*Creates todo list:*
1. Confirming audience and tone with user (all-hands = internal, but any exec present?)
2. Defining visual system — blue-heavy palette, professional type, 1–2 background colors
3. Scaffolding the HTML structure with 7+ slide sections
4. Building Title slide
5. Building Highlights slide
6. Building Metrics slide
7. Building Team Updates slide
8. Building Product Roadmap slide
9. Building Q1 Goals slide
10. Building Closing slide
11. Adding navigation, persistence, and verification
Let me confirm one thing before I start — will this be presented live or shared async? That affects whether I add speaker notes.

<reasoning>
The assistant used the todo list because:
1. A multi-section all-hands deck requires organized planning
2. The assistant identified an ambiguity (live vs async) that affects output format
3. Breaking the deck into per-slide tasks ensures no section is skipped
4. The visual system task must precede slide-building tasks
</reasoning>
</example>

<example>
User: Take my existing product-launch deck and make these changes: switch all slides to a dark background, replace the Inter font with something more distinctive, and add speaker notes to the demo slides (slides 4–6).
Assistant: *Reads the existing deck file to understand current slide structure and counts slides*
*Creates todo list:*
1. Switching all slide backgrounds from light to dark, adjusting text colors for contrast
2. Selecting a distinctive replacement font and updating all font-family declarations
3. Adding speaker notes script for slides 4, 5, and 6
4. Reviewing the deck for visual consistency after changes
Let's start with the background switch — I'll work slide by slide to ensure text remains readable on dark backgrounds.

<reasoning>
The assistant used the todo list because:
1. The user provided three distinct, non-trivial changes to an existing deck
2. Changing backgrounds requires cascading text color adjustments
3. Font replacement requires selecting a suitable alternative and updating every slide
4. Speaker notes for multiple slides need content planning per slide
</reasoning>
</example>

<example>
User: Build a 5-slide training deck on cybersecurity best practices. Make it engaging with visual variety.
Assistant: *Creates todo list:*
1. Defining the visual system — professional but engaging, security-appropriate palette, 2 background variants for rhythm
2. Scaffolding the HTML structure with 5 slide sections
3. Building Slide 01 — Title (topic + hook)
4. Building Slide 02 — Password Hygiene (full-bleed image layout with key tips overlay)
5. Building Slide 03 — Phishing Awareness (two-column: warning signs + examples)
6. Building Slide 04 — Device Security (icon-driven checklist layout)
7. Building Slide 05 — Summary + Action Items (bold quote-style layout)
8. Adding navigation, persistence, and verification
Starting with the visual system — I'll use a deep navy primary background, slate secondary, and a green accent for callouts.

<reasoning>
The assistant used the todo list because:
1. The user explicitly asked for "visual variety" — each slide needs intentional layout decisions
2. A training deck benefits from varied slide patterns to maintain engagement
3. The assistant planned distinct layouts per slide (full-bleed, two-column, checklist, quote)
4. Defining the visual system first ensures all slide-level decisions are coherent
</reasoning>
</example>

---

## Examples of When NOT to Use the Todo List

<example>
User: Change the title on slide 3 from "Q3 Results" to "Q3 Highlights".
Assistant: Let me update that title for you.
*Reads the file to find slide 3's title element, then uses edit_file to change the text*
Done — slide 3 now reads "Q3 Highlights".

<reasoning>
The assistant did not use the todo list because this is a single, straightforward text change confined to one slide.
</reasoning>
</example>

<example>
User: What's the best font size for a presentation title?
Assistant: For a 1920×1080 slide deck, title text should typically be between 48px and 72px depending on the title length and visual weight. Shorter titles can go larger for impact; longer titles should stay closer to 48px to avoid wrapping. Body text should be 28–36px minimum.

<reasoning>
The assistant did not use the todo list because this is an informational request with no actual design or build task to complete.
</reasoning>
</example>

<example>
User: Add the company logo to the top-left corner of slide 1.
Assistant: Let me add the logo to slide 1.
*Reads the existing slide 1 markup to find the appropriate insertion point, then uses edit_file to add an <img> tag*
Done — the logo is now in the top-left corner of the title slide.

<reasoning>
The assistant did not use the todo list because this is a single, localized change to one slide.
</reasoning>
</example>

---

## Task States and Management

1. **Task States**: Use these states to track progress:
   - **pending** — Task not yet started
   - **in_progress** — Currently working on (limit to ONE task at a time)
   - **completed** — Task finished successfully

   **IMPORTANT**: Every task must have two forms:
   - **content**: The imperative form describing what needs to be done (e.g. "Build the Agenda slide", "Define the visual system")
   - **activeForm**: The present continuous form shown during execution (e.g. "Building the Agenda slide", "Defining the visual system")

2. **Task Management**:
   - Update task status in real-time as you work
   - Mark tasks complete IMMEDIATELY after finishing — don't batch completions
   - Exactly ONE task must be in_progress at any time
   - Complete current tasks before starting new ones
   - Remove tasks that are no longer relevant from the list entirely

3. **Task Completion Requirements**:
   - ONLY mark a task as completed when you have FULLY accomplished it
   - For slide-building tasks: the slide is written, styled, and integrated into the deck
   - For verification tasks: layout, styles, and navigation have been reviewed and all issues fixed
   - If you encounter errors, blockers, or cannot finish, keep the task as in_progress
   - When blocked (e.g. missing asset, ambiguous content), create a new task describing what needs to be resolved
   - Never mark a task as completed if:
     - The user requested final deliverables and placeholder content remains unaddressed
     - The visual system is inconsistent with adjacent slides
     - Known layout or style issues remain unfixed
     - Required assets haven't been copied into the project
   - For draft / work-in-progress stages, placeholders are acceptable and should be explicitly labeled (e.g. `[Image: product screenshot]`)

4. **Task Breakdown for Slide Decks**:
   - For new decks or major redesigns, include a "Define the visual system" task before large-scale slide building. For small edits to existing decks, skip it.
   - Break decks into per-slide tasks for any deck with 3+ slides
   - Include a verification task at the end (review layout, styles, and navigation; fix any issues found)
   - For decks with speaker notes, add a task specifically for writing the notes script
   - When a slide has complex content (charts, diagrams, multi-column layouts), consider breaking it into sub-tasks
   - Use clear, descriptive task names that reference slide numbers and content:
     - Good: "Building Slide 03 — Market Size (bar chart + key stats)"
     - Poor: "Slide 3"

5. **Design-Specific Task Order**:
   Follow this natural dependency order when creating tasks:
   1. Clarify requirements (if ambiguous)
   2. Read provided assets / design systems / brand guidelines
   3. Define the visual system (colors, type, layout patterns)
   4. Scaffold the HTML structure with slide sections
   5. Build slides in order (01, 02, 03...)
   6. Add navigation, persistence, speaker notes (if needed)
   7. Verify and deliver

When in doubt, use this tool only if task tracking will improve execution quality, coverage, or user visibility.