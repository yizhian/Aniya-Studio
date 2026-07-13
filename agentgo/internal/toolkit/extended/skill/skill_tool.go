package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"agentgo/internal/provider"
	"agentgo/internal/toolkit/contracts"
)

// SkillTool implements contracts.Tool, allowing the model to query, get, search, and list assets of skills.
type SkillTool struct {
	index  *SkillIndex
	ranker *SkillRanker
}

func NewSkillTool(index *SkillIndex, ranker *SkillRanker) *SkillTool {
	return &SkillTool{index: index, ranker: ranker}
}

func (t *SkillTool) Descriptor() contracts.ToolDescriptor {
	return contracts.ToolDescriptor{
		Name:               "skill",
		Description:        "Query, get, search, or list assets of available skills. Skills provide specialized capabilities and domain knowledge.",
		Prompt:             "Use the skill tool when you need to discover or retrieve details about available skills. Operations: query (list all), get (retrieve by name), search (semantic style matching), list_assets (list files and directories for a skill).",
		MaxResultSizeChars: 48000,
		InputJSONSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"operation": map[string]any{
					"type":        "string",
					"enum":        []string{"query", "get", "search", "list_assets"},
					"description": "query=list all skill names, get=retrieve full details by name, search=semantic search for design skills by style description, list_assets=list files and directories for a skill",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Skill name. Required for 'get' and 'list_assets' operations.",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Natural language style description. Required for 'search' operation. Semantically matched against design skills.",
				},
				"design_brief": map[string]any{
					"type":        "string",
					"description": "Accumulated design refinements from multi-turn chat. Optional.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of recommendations to return (default 3). Optional.",
				},
			},
			"required": []any{"operation"},
		},
		Flags: contracts.ToolBehaviorFlags{
			ConcurrencySafe: true,
			ReadOnly:        true,
		},
	}
}

func (t *SkillTool) Call(ctx context.Context, args contracts.ToolCallArgs) contracts.ToolResult {
	var input struct {
		Operation   string `json:"operation"`
		Name        string `json:"name"`
		Query       string `json:"query"`
		DesignBrief string `json:"design_brief"`
		Limit       int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(args.ArgsJSON), &input); err != nil {
		return errResult("invalid_json", err.Error())
	}

	switch input.Operation {
	case "query":
		return t.handleQuery()
	case "get":
		return t.handleGet(input.Name)
	case "search":
		r := t.handleSearch(ctx, args, input.Query, input.DesignBrief, input.Limit)
		if !r.IsError {
			r.Metadata = map[string]any{"skill_recommendations": r.Content}
		}
		return r
	case "list_assets":
		return t.handleListAssets(input.Name)
	default:
		return errResult("unknown_operation", fmt.Sprintf("unknown operation %q", input.Operation))
	}
}

func (t *SkillTool) handleQuery() contracts.ToolResult {
	names := t.index.AllNames()
	payload := map[string]any{
		"skills": names,
		"count":  len(names),
	}
	return okResult(payload)
}

func (t *SkillTool) handleGet(name string) contracts.ToolResult {
	if name == "" {
		return errResult("missing_name", "name is required for get operation")
	}
	s, ok := t.index.ByName(name)
	if !ok {
		return errResult("not_found", fmt.Sprintf("skill %q not found", name))
	}
	payload := map[string]any{
		"skill": map[string]any{
			"name":        s.Name,
			"description": s.Description,
			"when_to_use": s.WhenToUse,
			"type":        s.Type,
			"body":        s.Body,
			"source":      s.Source,
			"dir_path":    s.DirPath,
			"triggers":    s.Triggers,
			"mode":        s.Mode,
			"scenario":    s.Scenario,
			"has_assets":  s.HasAssets,
		},
	}
	return okResult(payload)
}

func (t *SkillTool) handleSearch(ctx context.Context, callArgs contracts.ToolCallArgs, query, designBrief string, limit int) contracts.ToolResult {
	if query == "" {
		return errResult("missing_query", "query is required for search operation")
	}
	if limit <= 0 {
		limit = 3
	}

	p, _ := callArgs.Context.Provider.(provider.StreamingProvider)
	if t.ranker == nil || p == nil {
		return errResult("provider_unavailable", "LLM provider not available for skill search")
	}

	recs, err := t.ranker.Recommend(ctx, p, query, designBrief, limit)
	if err != nil {
		return errResult("search_failed", err.Error())
	}

	return okResult(map[string]any{
		"recommendations": recs,
		"query":           query,
		"source":          "llm",
	})
}

func (t *SkillTool) handleListAssets(name string) contracts.ToolResult {
	if name == "" {
		return errResult("missing_name", "name is required for list_assets operation")
	}
	// Reject path traversal attempts.
	if strings.Contains(name, "..") {
		return errResult("invalid_name", "skill name must not contain '..'")
	}
	s, ok := t.index.ByName(name)
	if !ok {
		return errResult("not_found", fmt.Sprintf("skill %q not found", name))
	}
	entries, err := t.index.ListAssets(name, s.DirPath)
	if err != nil {
		return errResult("list_failed", err.Error())
	}
	var files, dirs []string
	for _, e := range entries {
		if e.IsDir {
			dirs = append(dirs, e.Name)
		} else {
			files = append(files, e.Name)
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)
	payload := map[string]any{
		"files": files,
		"dirs":  dirs,
		"count": len(entries),
	}
	return okResult(payload)
}

func okResult(v any) contracts.ToolResult {
	data, _ := json.Marshal(v)
	return contracts.ToolResult{Content: string(data)}
}

func errResult(code, msg string) contracts.ToolResult {
	data, _ := json.Marshal(map[string]string{"error_code": code, "error_message": msg})
	return contracts.ToolResult{
		Content:      string(data),
		IsError:      true,
		ErrorCode:    code,
		ErrorMessage: msg,
	}
}
