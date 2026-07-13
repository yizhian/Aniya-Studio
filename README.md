# Aniya Studio

**Language: English | [中文](READMEzh.md)**

![License](https://img.shields.io/badge/license-Apache--2.0-blue)
![Go](https://img.shields.io/badge/agentgo-Go%201.25-00ADD8)
![Python](https://img.shields.io/badge/BFF-FastAPI-009688)
![React](https://img.shields.io/badge/frontend-React%20%2B%20Vite-61DAFB)





https://github.com/user-attachments/assets/c0fc0dbf-5536-482a-9591-b873c5923bfa



> **In one line**: Aniya Studio is an open-source, self-hosted **AI slide / HTML PPT editor** — describe what you want in plain language, or drag-and-drop tweak it like a design tool, and switch between the two on the very same canvas.

---


## Why Aniya Studio

Aniya Studio focuses on one thing: an **AI-generation-plus-visual-editing workflow built specifically around HTML slide decks / presentations.** Around that single scenario, we've gone deep on the following:

- **Two editing modes on one canvas, not an either/or choice.** GrapesJS direct-editing mode (drag, select, inline text editing, undo/redo) and an AI Design Mode (select a DOM node → describe the change in natural language → a scoped style/text patch is applied) share the exact same canvas and can be switched at any time — describe what you want to the AI, or manually fine-tune the very same element like you would in a design tool.
- **AI-generated HTML is 100% compatible with the visual editor.** A three-layer defense (teach the Agent via a Skill → enforce it via a Hook → a frontend Adapter as the last line of defense) guarantees every AI-authored HTML file loads and edits cleanly in GrapesJS, the editor engine never mutates the original source, and preview/export always reflect the Agent's original output.
- **Built-in hook governance, not just "it runs".** File-type allow-lists, protected paths, read-before-write anti-hallucination checks, consecutive-failure detection, and a pre-publish compliance checklist all live in `agentgo/internal/hook`, enabled by default and overridable per project via `hooks.yaml`.
- **Genuinely lightweight and self-hostable.** The core is a from-scratch, single-binary Go streaming agent (`agentgo`) with SSE streaming, session persistence, and a directory-tree semantic memory store. Root `./start.sh` brings everything up in one command (Docker or pure local source), with no extra heavyweight runtime required.
- **BYOK, made simple.** Flip two environment variables (`PROVIDER_TYPE` + `DEEPSEEK_BASE_URL`) to switch between the OpenAI-compatible protocol (DeepSeek, OpenAI, etc.) and the Anthropic protocol.
- **Content depth aimed squarely at slides.** 58 presentation-specific skills, 30+ visual themes (from Xiaohongshu-style cards to cyberpunk terminals to minimalist editorial), a presenter mode, multi-page navigation, and chart/image replacement — all polished around this one scenario.

---

## Architecture

```
┌──────────────┐     SSE/HTTP      ┌──────────────────┐
│   agentgo    │ ◄──────────────► │  htmlslide/backend│
│  (Go Agent)  │                   │   (FastAPI BFF)   │
└──────────────┘                   └────────┬─────────┘
                                     │ REST API
                              ┌──────┴──────────┐
                              │  htmlslide/      │
                              │  frontend (React)│
                              └─────────────────┘
```

- **agentgo (Go)**: Streaming multi-round agent loop handling thinking / text / tool_call events; read-only tools run in parallel, writes run sequentially; ships with hook governance and directory-tree semantic memory.
- **htmlslide/backend (FastAPI)**: BFF layer that mediates between the frontend and the agent, handles file uploads, PPTX export, etc.
- **htmlslide/frontend (React + GrapesJS)**: The editor UI — direct-editing mode and AI Design Mode share the same canvas.

## Project Structure

```
aniya-studio/
├── start.sh           One-command launcher (recommended)
├── agentgo/           Go agent (SSE dialogue, tool calling, session persistence, hook governance)
│   ├── skills/        58 design skills
│   ├── project-skills/ project-level skills (e.g. GrapesJS compliance rules)
│   ├── internal/hook/ hook governance system (allow-lists, protected paths, compliance review)
│   └── vendor/        Go vendored dependencies
├── htmlslide/
│   ├── backend/       Python FastAPI BFF
│   ├── frontend/      React + Vite + Tailwind + GrapesJS editor
│   └── doc/           Design docs
├── README.md          This document (English, default)
└── READMEzh.md        Chinese version
```

## Provider Support

`agentgo` supports two LLM provider protocols:

| Provider type | Compatible API services | How to configure |
|---------------|--------------------------|-------------------|
| `openai` (default) | DeepSeek, OpenAI, and other OpenAI-compatible APIs | `PROVIDER_TYPE=openai` |
| `anthropic` | Anthropic Claude and Anthropic-protocol-compatible APIs | `PROVIDER_TYPE=anthropic` |

Switching providers only requires changing two environment variables: `PROVIDER_TYPE` and the matching `DEEPSEEK_BASE_URL`.

## Quick Start (Recommended: start.sh)

One command starts all three services. Default (hybrid): **AgentGo + Backend via Docker**, **Frontend via local Vite**.

1. Create `htmlslide/.env` with your API credentials:

   ```bash
   DEEPSEEK_API_KEY=your_api_key
   # Optional, with sensible defaults:
   DEEPSEEK_MODEL=deepseek-v4-flash
   DEEPSEEK_BASE_URL=https://api.deepseek.com
   PROVIDER_TYPE=openai
   ```

   > Using OpenAI: set `DEEPSEEK_BASE_URL=https://api.openai.com` and `DEEPSEEK_MODEL=gpt-4o`.
   > Using Anthropic: set `PROVIDER_TYPE=anthropic` and `DEEPSEEK_BASE_URL=https://api.anthropic.com`.

2. Start:

   ```bash
   ./start.sh              # default hybrid: Docker for backend/Agent, local frontend
   ./start.sh docker       # same as above (force Docker for backend/Agent)
   ./start.sh dev          # all local: Go / uv / npm from source
   ```

   Common controls:

   ```bash
   ./start.sh status
   ./start.sh logs         # agentgo|backend|frontend|all
   ./start.sh stop
   ./start.sh restart
   ```

3. Access:

   - Frontend editor: http://localhost:5173
   - Agent health check: http://localhost:8080/health
   - BFF API docs: http://localhost:8000/docs

Prerequisites:

- **hybrid / docker**: Docker Desktop, curl, Node.js/npm
- **dev**: Go, Python 3, uv, Node.js/npm (Docker optional)

## Manual Start (Optional)

### Docker Compose only

```bash
cd htmlslide
docker compose up -d
```

> Note: compose brings up AgentGo + Backend; the frontend still needs a local `npm run dev`, or just use `./start.sh` above.

### Run each process from source

#### agentgo (Go)

```bash
cd agentgo
cp .env.example .env    # fill in your API key
go run ./cmd/server     # defaults to port 8080
```

#### backend (Python)

```bash
cd htmlslide/backend
uv sync
uv run uvicorn src.main:app --reload
```

#### frontend (React)

```bash
cd htmlslide/frontend
npm install
npm run dev
```

## `.env` Files Explained

| File | Used by | Notes |
|------|---------|-------|
| `htmlslide/.env` | Docker Compose / `./start.sh` | **Most users only need this one**; never commit it |
| `agentgo/.env.example` | Standalone Go / `./start.sh dev` | Copy to `agentgo/.env` and fill in your key |
| `htmlslide/backend/.env.example` | Standalone BFF | Copy to `.env` when debugging the backend alone |

> `.env` files contain API keys and are excluded by `.gitignore` — do not commit them.

## Documentation

- [agentgo docs](agentgo/README.md)
- [htmlslide docs](htmlslide/README.md)

## License

Apache-2.0 — see the [LICENSE](LICENSE) file for details.
