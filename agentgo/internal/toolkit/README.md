# Toolkit Framework

## 目录命名

- `contracts/`：Tool 接口与入参/出参契约
- `registry/`：`ToolRegistry`（注册、延迟占位、`GetActiveToolDefinitions` / `GetDeferredToolNames` / `ActivateDeferred`）
- `engine/`：`StreamingToolExecutor`（执行入口，后续扩展权限与并发）
- `bootstrap/`：`RegisterAllTools`（进程启动时调用一次）
- `core/`：已实现的核心工具
- `extended/`：预留扩展工具包
- `render/`：预留「工具卡片」渲染协议
- `schemas/`：预留 Zod / JSON Schema 映射

## 当前已注册的核心工具

| 工具名 | 说明 |
|--------|------|
| `read_file` | 读文件（可带行范围）；metadata 含 **`read_mtime_unix_ns`（字符串）**，供 `edit_file` 前置校验 |
| `write_file` | **仅新建文件**；路径已存在则失败（应改用 `read_file` + `edit_file`） |
| `edit_file` | **search-and-replace**：强制 `read_mtime_unix_ns`（与 `read_file` 一致）、mtime 双检、**唯一匹配**（0/>1 均失败）、**弯/直引号容错**、`.claude/settings.json` 编辑后 **JSON 语法校验** |
| `list_files` | 列目录（可选递归） |
| `grep_search` | 在工作区内用正则搜索文件内容 |
| `web_fetch` | HTTP(S) 拉取：30s 超时、HTML 去标签、原始体最多读 50KB、输出再截断；`ConcurrencySafe` |
| `tool_search` | 按名称子串激活**延迟工具**（见下） |

已移除占位：`file_write` / `file_edit` / `shell`；`search` 已统一为 **`grep_search`**。

## 延迟工具（ToolSearch 工作流）

1. `RegisterDeferred` 注册的工具（当前为 `skill`、`agent`、`plan_mode`）在首次调用前**不加载实现**。
2. `GetActiveToolDefinitions()`：未激活的延迟工具在发给模型的 `tools` 里仅为**名称占位**（`description` 提示用 `tool_search` 激活；`parameters` 为空 object）。
3. `GetDeferredToolNames()`：供拼进 **system prompt**，列出尚未激活的延迟工具名。
4. 模型调用 `tool_search`（参数 `query`，可选 `exact`）后，匹配项会 `ActivateDeferred`：加载实现（当前为 **Stub**，执行时返回 `is_error` 数据）。
5. 下一次 `GetActiveToolDefinitions()` 会对已激活项返回**完整** `InputJSONSchema`。

## Tool 契约入口

见 `contracts/tool_contract.go`：`Tool`、`ToolDescriptor`、`ToolCallArgs`、`ToolResult`。

## 如何测试这些工具

### 1. 跑全部单元测试（推荐）

在项目根目录执行：

```bash
cd agentgo
go test ./...
```

### 2. 只测 toolkit（更快）

```bash
go test ./internal/toolkit/...
```

### 3. 按包 / 按用例名过滤

```bash
# 文件类工具
go test -v ./internal/toolkit/core -run 'TestReadFileTool|TestListFilesTool|TestGrepSearchTool|TestWebFetchTool'

# 注册 + tool_search + 延迟激活整条链
go test -v ./internal/toolkit/bootstrap -run TestRegisterAllToolsAndDeferredFlow
```

说明：

- `read_file` / `list_files` / `grep_search`：测试里会建临时目录当 **workspace**，在 `ToolCallArgs.Context.WorkspacePath` 里传入。
- `web_fetch`：测试里用 **`httptest` 假 HTTP 服务**，不访问公网、也不需要任何 API Key。
- `tool_search`：在 `bootstrap` 测试里和 `RegisterAllTools` 一起跑，会激活 `skill` 等 deferred stub。

### 4. 想「手动」试（不写测试）

可以在任意 `_test.go` 里临时写一个 `TestManual` 用 `t.Skip()` 控制，或在本机起一个 `main` 包：调用 `registry.NewToolRegistry()` → `bootstrap.RegisterAllTools` → `engine.StreamingToolExecutor{Registry}` → `Execute(ctx, "grep_search", args)`。要点与单测相同：**必须设置 `Context.WorkspacePath` 为你要扫的目录**，否则路径解析行为与测试不一致。

## web_fetch 要不要绑定什么 API？

**当前实现不需要绑定第三方 API。**

它就是 Go 标准库里的 **`net/http.Client`** 对目标 URL 发 **GET**，属于「通用 HTTP 客户端」：

- 不需要像「地图 / 支付」那样申请专用商业 API。
- 若你要抓的是 **需要鉴权** 的站点（Cookie、Bearer、API Key），那是**业务层**在后续加：`ToolCallContext.Metadata` 里带 token、或专用 `headers` 参数、或走你们自己的 **反向代理 / BFF**，再扩展 `web_fetch` 即可。
- 生产环境通常还会加：**SSRF 防护**（禁止访问内网 IP、仅允许域名白名单）、固定 **User-Agent**、代理、速率限制等；这些目前还没做，属于架构加固项。

简言之：**架构上就是「HTTP 客户端工具」**；绑定的是「网络与你们自己的安全策略」，不是某个固定厂商 API。

## 后续（未实现）

- Zod / JSON Schema 双阶段校验管线
- 权限 Hook、Bash 分类器、并发门控、大结果落盘
- `skill` / `agent` / `plan_mode` 的真实实现（替换 Stub）
