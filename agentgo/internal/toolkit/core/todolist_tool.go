package core

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"agentgo/internal/toolkit/contracts"
)

//go:embed prompts/todolist.md
var todolistPromptText string

type TodoWriteTool struct{}

func NewTodoWriteTool() *TodoWriteTool {
	return &TodoWriteTool{}
}

func (t *TodoWriteTool) Descriptor() contracts.ToolDescriptor {
	return contracts.ToolDescriptor{
		Name:        "todo_write",
		Description: "Create and manage a structured task list for your current slide deck design session. Tracks progress through the presentation workflow and organizes complex deck-building tasks.",
		Prompt:      todolistPromptText,
		InputJSONSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"todos": map[string]any{
					"type": "array",
					"description": "The updated todo list",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"content": map[string]any{
								"type":        "string",
								"description": "The imperative form describing what needs to be done (e.g. 'Build the Agenda slide', 'Define the visual system')",
							},
							"status": map[string]any{
								"type":        "string",
								"enum":        []string{"pending", "in_progress", "completed"},
								"description": "Task status: pending, in_progress, or completed",
							},
							"activeForm": map[string]any{
								"type":        "string",
								"description": "The present continuous form shown during execution (e.g. 'Building the Agenda slide', 'Defining the visual system')",
							},
						},
						"required": []string{"content", "status", "activeForm"},
					},
				},
			},
			"required": []string{"todos"},
		},
		Flags: contracts.ToolBehaviorFlags{
			ConcurrencySafe: true,
			ReadOnly:        true,
		},
	}
}

type todoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

func (t *TodoWriteTool) Call(ctx context.Context, args contracts.ToolCallArgs) contracts.ToolResult {
	_ = ctx
	var input struct {
		Todos []todoItem `json:"todos"`
	}
	if err := json.Unmarshal([]byte(args.ArgsJSON), &input); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_json", ErrorMessage: err.Error()}
	}

	for i, todo := range input.Todos {
		if todo.Content == "" {
			return contracts.ToolResult{
				IsError: true, ErrorCode: "invalid_todo",
				ErrorMessage: fmt.Sprintf("todo item %d: content is required", i),
			}
		}
		if todo.ActiveForm == "" {
			return contracts.ToolResult{
				IsError: true, ErrorCode: "invalid_todo",
				ErrorMessage: fmt.Sprintf("todo item %d: activeForm is required", i),
			}
		}
		switch todo.Status {
		case "pending", "in_progress", "completed":
		default:
			return contracts.ToolResult{
				IsError: true, ErrorCode: "invalid_status",
				ErrorMessage: fmt.Sprintf("todo item %d: status must be pending, in_progress, or completed, got %q", i, todo.Status),
			}
		}
	}

	out, _ := json.MarshalIndent(input.Todos, "", "  ")
	return contracts.ToolResult{
		Content:  fmt.Sprintf("Todo list updated (%d items):\n%s", len(input.Todos), string(out)),
		Metadata: map[string]any{"count": len(input.Todos)},
	}
}
