package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agentgo/internal/toolkit/contracts"
	"agentgo/internal/toolkit/registry"
)

// ToolSearchTool 按需激活延迟注册的工具（只发名称占位符的工具）。
type ToolSearchTool struct {
	reg *registry.ToolRegistry
}

func NewToolSearchTool(reg *registry.ToolRegistry) *ToolSearchTool {
	return &ToolSearchTool{reg: reg}
}

func (t *ToolSearchTool) Descriptor() contracts.ToolDescriptor {
	return contracts.ToolDescriptor{
		Name:               "tool_search",
		Description:        "Activate deferred tools by name or substring. After activation, the next model request should include their full JSON schemas.",
		Prompt:             "When you need a capability that appears only in the deferred tool list, call tool_search with a short query matching that tool name.",
		MaxResultSizeChars: 8000,
		InputJSONSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Substring to match against deferred tool names (case-insensitive)",
				},
				"exact": map[string]any{
					"type":        "boolean",
					"description": "If true, query must equal the tool name",
				},
			},
			"required": []any{"query"},
		},
		Flags: contracts.ToolBehaviorFlags{
			ConcurrencySafe: true,
			ReadOnly:        true,
		},
	}
}

func (t *ToolSearchTool) Call(ctx context.Context, args contracts.ToolCallArgs) contracts.ToolResult {
	_ = ctx
	if t.reg == nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "registry_nil", ErrorMessage: "tool registry is nil"}
	}
	var input struct {
		Query string `json:"query"`
		Exact bool   `json:"exact"`
	}
	if err := json.Unmarshal([]byte(args.ArgsJSON), &input); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_json", ErrorMessage: err.Error()}
	}
	q := strings.TrimSpace(input.Query)
	if q == "" {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_query", ErrorMessage: "query is required"}
	}

	candidates := t.reg.GetDeferredToolNames()
	qLower := strings.ToLower(q)
	var matched []string
	for _, name := range candidates {
		nl := strings.ToLower(name)
		if input.Exact {
			if nl == qLower {
				matched = append(matched, name)
			}
			continue
		}
		if strings.Contains(nl, qLower) {
			matched = append(matched, name)
		}
	}
	if len(matched) == 0 {
		payload := map[string]any{
			"activated":      []string{},
			"message":        fmt.Sprintf("no deferred tools matched query %q", q),
			"still_deferred": t.reg.GetDeferredToolNames(),
		}
		return contracts.ToolResult{
			Content:  mustMarshalJSON(payload),
			Metadata: map[string]any{"matched": 0},
		}
	}

	var activated []string
	var errs []string
	for _, name := range matched {
		if err := t.reg.ActivateDeferred(name); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		activated = append(activated, name)
	}

	payload := map[string]any{
		"activated":      activated,
		"errors":         errs,
		"still_deferred": t.reg.GetDeferredToolNames(),
		"message":        "activated tools will appear with full schemas on the next API tools list",
	}
	return contracts.ToolResult{
		Content:  mustMarshalJSON(payload),
		Metadata: map[string]any{"activated_count": len(activated)},
	}
}

func mustMarshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}
