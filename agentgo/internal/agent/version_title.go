package agent

import (
	"context"
	"log"
	"strings"

	"agentgo/internal/model"
	"agentgo/internal/observability"
	"agentgo/internal/provider"
)

func generateVersionTitle(ctx context.Context, prov provider.StreamingProvider, userPrompt string, emitter *observability.Emitter) string {
	if prov == nil || userPrompt == "" {
		return ""
	}
	resp, err := prov.Chat(ctx, provider.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: "将用户指令总结为不超过15个字的标题。只返回标题，不要引号、标点或解释。"},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 60,
		Stream:    false,
	})
	if err != nil {
		log.Printf("generateVersionTitle: %v", err)
		observability.EmitOrLog(emitter, observability.AgentEvent{
			Type: observability.EventError,
			Data: map[string]any{"message": "generate version title: " + err.Error()},
		})
		return ""
	}
	title := strings.TrimSpace(resp.Message.Content)
	title = strings.Trim(title, "\"'")
	if len([]rune(title)) > 15 {
		runes := []rune(title)
		title = string(runes[:15])
	}
	return title
}
