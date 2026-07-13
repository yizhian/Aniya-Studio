# Aniya Studio

AI-driven HTML slide editor — BFF (Backend for Frontend) + Go Agent + React Frontend.

## Project Structure

```
htmlslide/
├── frontend/        # React + Vite + Tailwind CSS frontend
│   ├── src/         # React source code
│   ├── dist/        # Built output
│   └── package.json
├── backend/         # Python FastAPI BFF backend
│   ├── src/         # Python source code
│   └── tests/       # Test suite
├── doc/             # Design docs and planning
├── docker-compose.yml
└── docker-compose.prod.yml
```

## Running the code

### Frontend

```bash
cd frontend
npm install
npm run dev
```

### Backend

```bash
cd backend
uv sync
uv run uvicorn src.main:app --reload
```

### Full Stack (Docker)

```bash
docker compose up
```

## Editor features

- Direct edit mode keeps the GrapesJS editing workflow for text editing, dragging, selection, undo and redo.
- Design mode links the selected DOM node to the bottom prompt, then applies controlled style or text patches with visible error messages.
- The editor supports preview, image replacement, multi-page navigation, ECharts insertion/editing, font family changes, and light/dark theme switching.
- Imported slide-style HTML files that use `.slide` sections can be switched with the editor side arrows or keyboard arrows.

See commit history for implementation notes.

---

## GrapesJS Compatibility Architecture

The Agent (SlideCraft) generates HTML slide decks. These are loaded into GrapesJS for visual editing. To keep the Agent focused on design and prevent it from learning editor internals, compatibility is handled by a three-layer defense:

### Principle: "System guarantees, Agent decides"

The Agent only knows **what not to write** — it does not know GrapesJS exists. Engineering handles everything else.

### Agent Prompt Constraints (minimal)

Only two format rules in the system prompt (`agentgo/prompts/system.md`):

- `<!DOCTYPE html>` with complete `<html>`, `<head>`, `<body>` structure
- No `data-gjs-*` attributes

### Engineering Defense

| Layer | Location | What it does |
|-------|----------|---------------|
| **Project Skill** | `agentgo/project-skills/grapesjs-html-compliance/SKILL.md` | Teaches the AI agent GrapesJS compatibility rules before generating HTML; agent self-reviews against checklist after every write |
| **Hook System** | `agentgo/internal/hook/builtin/builtin.go` | Enforces skill loading before HTML creation in InitialGeneration; injects workflow instructions and review reminders |
| **Frontend Adapter** | `htmlslide/frontend/src/services/editorApi.ts` | On import: extracts `<script>` for preview, merges `<style>` blocks into `<body>`, syncs `<link>` assets |

All layers run automatically. The Project Skill teaches the agent before generation. The Hook system enforces the skill-then-generate workflow. The Frontend Adapter runs when the editor imports HTML.

### Original files are never modified

The agent generates GrapesJS-compatible HTML directly, following the compliance skill rules. Preview and export use the Agent's original output — preserving full JS navigation, scaling, and interactivity.

---

## Rebuilding After Code Changes

### AgentGo (Go backend + prompts)

```bash
# From htmlslide/ directory — rebuilds the agentgo image
docker compose build agentgo

# Then restart
docker compose up -d agentgo
```

If only the system prompt (`agentgo/prompts/system.md`) changed, you must rebuild because the Dockerfile copies `prompts/` into the image.

### Backend (Python BFF)

```bash
docker compose build backend
docker compose up -d backend
```

### Frontend (React/Vite)

The frontend runs outside Docker in development (`npm run dev`). Hot-reload picks up changes automatically.

### Full restart after all changes

```bash
docker compose build
docker compose up -d
```
