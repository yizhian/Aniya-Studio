# Aniya Studio

**语言：中文 | [English](README.md)**

![License](https://img.shields.io/badge/license-Apache--2.0-blue)
![Go](https://img.shields.io/badge/agentgo-Go%201.25-00ADD8)
![Python](https://img.shields.io/badge/BFF-FastAPI-009688)
![React](https://img.shields.io/badge/frontend-React%20%2B%20Vite-61DAFB)

> **一句话介绍**：Aniya Studio 是一个开源、自托管的 **AI 幻灯片 / HTML PPT 编辑器**——你可以像聊天一样描述需求，也可以像在设计软件里一样直接拖拽微调，两种方式在同一张画布上无缝切换。

---

## 为什么选择 Aniya Studio

Aniya Studio 专注于一件事：**AI 生成 + 可视化编辑一体化的幻灯片 / HTML PPT 工作流**。围绕这个场景，我们把下面几件事做深了：

- **双模式编辑，不是二选一。** GrapesJS 直接编辑模式（拖拽、选中、文本编辑、撤销重做）与 AI 设计模式（选中某个 DOM 节点 → 自然语言描述 → 定点样式/文案 patch）共享同一张画布，随时切换——既能对着 AI 说需求，也能像用设计软件一样手动微调同一个元素。
- **AI 产出的 HTML 100% 兼容可视化编辑器。** 三层防御架构（Skill 教学 Agent → Hook 强制规范 → 前端 Adapter 兜底）确保 Agent 写出来的 HTML 永远能被 GrapesJS 正常加载和编辑，原始文件永不被引擎"翻译"破坏，预览与导出始终是 Agent 的原始产物。
- **内置 Hook 治理系统，不只是"能跑"。** 文件类型白名单、受保护路径、"先读后写"防幻觉校验、连续失败检测、发布前合规检查清单——这些安全护栏写在 `agentgo/internal/hook` 里，默认开启，可通过 `hooks.yaml` 按项目覆盖。
- **极轻量、真自托管。** 核心是一个自研的 Go 流式 Agent（`agentgo`），单二进制、SSE 流式输出、会话持久化、目录树语义记忆；根目录 `./start.sh` 一键拉起（Docker 或纯本地源码均可），不需要额外的重型运行环境。
- **BYOK 且简单。** 只改两个环境变量（`PROVIDER_TYPE` + `DEEPSEEK_BASE_URL`）即可在 OpenAI 兼容协议（DeepSeek / OpenAI 等）和 Anthropic 协议之间切换。
- **面向"幻灯片"这一场景的内容深度。** 58 个演示文稿专属 skill、30+ 主题风格（含小红书图文卡片、赛博终端、极简编辑风等）、演讲者模式、多页导航、图表/图片替换，都是围绕这一个场景垂直打磨的。

---

## 架构 Architecture

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

- **agentgo（Go）**：流式多轮 Agent 循环，处理 thinking / text / tool_call 事件；只读工具并行执行，写操作顺序执行；内置 Hook 治理与目录树语义记忆。
- **htmlslide/backend（FastAPI）**：BFF 层，转发/整理前端与 Agent 之间的请求，处理文件上传、PPTX 导出等。
- **htmlslide/frontend（React + GrapesJS）**：编辑器主界面，直接编辑模式与 AI 设计模式共享同一画布。

## Project Structure

```
aniya-studio/
├── start.sh           一键启动脚本（推荐）
├── agentgo/           Go Agent（SSE 对话、工具调用、会话持久化、Hook 治理）
│   ├── skills/        58 个设计 skill
│   ├── project-skills/项目级 skill（如 GrapesJS 兼容性规范）
│   ├── internal/hook/ Hook 治理系统（白名单、受保护路径、合规校验）
│   └── vendor/        Go vendor 依赖
├── htmlslide/
│   ├── backend/       Python FastAPI BFF
│   ├── frontend/      React + Vite + Tailwind + GrapesJS 编辑器
│   └── doc/           设计文档
├── README.md          英文版说明（默认展示版本）
└── READMEzh.md        本文档（中文版）
```

## Provider Support

agentgo 支持两种 LLM Provider 协议：

| Provider 类型 | 兼容的 API 服务 | 配置方式 |
|--------------|----------------|---------|
| `openai`（默认） | DeepSeek、OpenAI、及其他 OpenAI 兼容 API | `PROVIDER_TYPE=openai` |
| `anthropic` | Anthropic Claude、及兼容 Anthropic 协议的 API | `PROVIDER_TYPE=anthropic` |

切换 provider 只需要改两个环境变量：`PROVIDER_TYPE` 和对应的 `DEEPSEEK_BASE_URL`。

## Quick Start（推荐：start.sh）

一条命令拉起三个服务。默认（hybrid）：**AgentGo + Backend 用 Docker**，**Frontend 本地 Vite**。

1. 创建 `htmlslide/.env`，填入你的 API 凭据：

   ```bash
   DEEPSEEK_API_KEY=你的API_Key
   # 以下可选（有默认值）：
   DEEPSEEK_MODEL=deepseek-v4-flash
   DEEPSEEK_BASE_URL=https://api.deepseek.com
   PROVIDER_TYPE=openai
   ```

   > 如果使用 OpenAI：改 `DEEPSEEK_BASE_URL=https://api.openai.com`、`DEEPSEEK_MODEL=gpt-4o`。
   > 如果使用 Anthropic：改 `PROVIDER_TYPE=anthropic`、`DEEPSEEK_BASE_URL=https://api.anthropic.com`。

2. 启动：

   ```bash
   ./start.sh              # 默认 hybrid：Docker 起后端/Agent，本地起前端
   ./start.sh docker       # 同上（强制 Docker 起后端/Agent）
   ./start.sh dev          # 纯源码：Go / uv / npm 全部本地运行
   ```

   常用管理命令：

   ```bash
   ./start.sh status
   ./start.sh logs         # agentgo|backend|frontend|all
   ./start.sh stop
   ./start.sh restart
   ```

3. 访问：

   - 前端编辑器：http://localhost:5173
   - Agent 健康检查：http://localhost:8080/health
   - BFF API 文档：http://localhost:8000/docs

前置要求：

- **hybrid / docker**：Docker Desktop、curl、Node.js/npm
- **dev**：Go、Python 3、uv、Node.js/npm（可不依赖 Docker）

## 手动启动（可选）

### 仅 Docker Compose

```bash
cd htmlslide
docker compose up -d
```

> 说明：compose 默认拉起 AgentGo + Backend；前端仍需本地 `npm run dev`，或直接用上面的 `./start.sh`。

### 纯源码分进程

#### agentgo (Go)

```bash
cd agentgo
cp .env.example .env    # 编辑填入 API Key
go run ./cmd/server     # 默认端口 8080
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

## .env 文件说明

| 文件 | 谁在用 | 说明 |
|------|-------|------|
| `htmlslide/.env` | Docker Compose / `./start.sh` | **大多数用户只需要这个**；不要提交到 Git |
| `agentgo/.env.example` | Go 独立开发 / `./start.sh dev` | 复制为 `agentgo/.env` 后填 Key |
| `htmlslide/backend/.env.example` | BFF 独立开发 | 单独调试 backend 时复制为 `.env` |

> `.env` 含 API Key，已在 `.gitignore` 中排除，切勿提交。

## Documentation

- [agentgo 文档](agentgo/README.md)
- [htmlslide 文档](htmlslide/README.md)

## License

Apache-2.0 — 详见 [LICENSE](LICENSE) 文件。
