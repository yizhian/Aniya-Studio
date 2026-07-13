# agentgo

A Go streaming agent framework with SSE multi-turn dialogue and tool calling. Supports multiple providers (OpenAI-compatible / Anthropic), session persistence, parallel read-only tool execution, and semantic memory recall.

## Features

- **Streaming multi-round loop**: Streams model output (thinking + text + tool_calls), executes tools, injects results, loops until model stops requesting tools.
- **Parallel read-only tools**: Tools marked `ReadOnly && ConcurrencySafe` execute in parallel via goroutines; write operations are sequential.
- **Multi-provider abstraction**: `StreamingProvider` interface supports both OpenAI (DeepSeek) and Anthropic API formats with automatic tool format conversion.
- **Session persistence**: Auto-saves `LoopState` JSON to `.agentgo/sessions/{id}.json` after each round; auto-loads history on reconnect.
- **SSE HTTP server**: `POST /chat` endpoint pushes all events via `text/event-stream` (thinking, text, tool_call, tool_result, error).
- **Semantic memory**: Directory-tree memory store with LLM-driven semantic recall and context budget protection.
- **Retry with backoff**: Exponential backoff for 429 / 503 / network errors.

## Quick Start

### Docker

```bash
# 1. Configure environment
cp .env.example .env
# Edit .env: DEEPSEEK_API_KEY / DEEPSEEK_MODEL / DEEPSEEK_BASE_URL

# 2. Build and run
docker compose up -d

# 3. Verify
curl http://localhost:8080/health
```

### Go (without Docker)

```bash
# 1. Configure environment
cp .env.example .env
# Edit .env

# 2. Run
go run ./cmd/server
```

Default port: `8080`.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DEEPSEEK_API_KEY` | API key | **Required** |
| `DEEPSEEK_MODEL` | Model name, e.g. `deepseek-v4-flash` | **Required** |
| `DEEPSEEK_BASE_URL` | API base URL, e.g. `https://api.deepseek.com` | **Required** |
| `PROVIDER_TYPE` | `openai` or `anthropic` | `openai` |
| `PORT` | Server port | `8080` |
| `AGENTGO_DATA_DIR` | Persistence directory (sessions/memory/files) | `./.agentgo` |

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/chat` | POST | Streaming chat (SSE), body: `{message, session_id}` |
| `/edit` | POST | Direct file edit (no LLM), body: `{session_id, path, old_string, new_string}` |
| `/upload` | POST | Multipart file upload, field: `files[]`, max 50MB |
| `/history` | GET | List all session summaries |
| `/sessions/{id}` | GET | Load full session JSON |
| `/files` | GET | List generated files |
| `/health` | GET | Health check |

See `docs/api.md` for detailed request/response formats and SSE event protocol.

## Skill Directory Conventions

- User-level skills: `./skills/`
- Project-level skills: `./project-skills/`
- Priority: `./project-skills/` overrides `./skills/` for same-named skills.

Skills are loaded automatically at server startup.

## Project Structure

```
agentgo/
  cmd/server/main.go                # HTTP server entry point
  internal/
    agent/
      streaming_loop.go             # Streaming multi-round loop
      system_prompt.go              # System prompt builder
      memory_recall.go              # Semantic memory recall
    model/
      chat_protocol_types.go        # Message / ToolCall / ToolDefinition
    config/
      config.go                     # Environment config
    provider/
      provider.go                   # StreamingProvider interface
      openai_provider.go            # OpenAI provider
      anthropic_provider.go         # Anthropic provider
      stream_types.go               # StreamEvent types
      tool_format.go                # Tool format conversion
    persistence/
      persistence.go                # Session / memory / file persistence
      memory_store_v2.go            # Directory-tree memory store
      memory_index.go               # Memory index management
      memory_types.go               # Memory type definitions
    context/
      manager.go                    # Context assembly + budget
      trimmer.go                    # Sliding window message trim
    observability/
      observability.go              # Event emitter + console observer
    retry/
      retry.go                      # Exponential backoff
    toolkit/
      registry/                     # Tool registry
      bootstrap/                    # Tool registration
      core/                         # Built-in tool implementations
      contracts/                    # Tool interface definitions
      engine/                       # Tool execution engine
      extended/skill/               # Skill system
  skills/                          # User-level skills
  docs/
    api.md                          # API documentation
  .env.example                      # Environment template
```

## Tests

```bash
go test ./...
```

## Testing Session Persistence

```bash
# Start server
go run ./cmd/server &

# First message
curl -s -N -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"My name is Alice","session_id":"my-session"}' | tail -5

# Second message (same session_id)
curl -s -N -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"What is my name?","session_id":"my-session"}' | tail -5
# → Model should remember the name
```
