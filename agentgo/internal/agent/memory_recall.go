package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"agentgo/internal/model"
	"agentgo/internal/persistence"
	"agentgo/internal/provider"
)

const recallSystemPrompt = `You are a memory matcher. Based on the user's question, select the memory entries from the candidate list that are semantically relevant.
Return only the index numbers separated by commas, e.g. "0,3". Return "-1" if none are relevant.
Do not return any other text.`

// SemanticRecall matches the user query against the memory index and returns relevant memories.
// It uses a lightweight LLM call to select semantically relevant entries, then loads the full files.
func SemanticRecall(
	ctx context.Context,
	query string,
	workspacePath string,
	memStore persistence.MemoryStore,
	idx []persistence.MemoryIndexEntry,
	p provider.StreamingProvider,
) ([]persistence.RecalledMemory, error) {
	if shouldSkipRecall(query, idx) {
		return nil, nil
	}

	selected, err := matchMemories(ctx, query, idx, p)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, nil
	}

	var paths []string
	for _, s := range selected {
		paths = append(paths, s.Path)
	}
	return memStore.LoadRecalled(workspacePath, paths)
}

// minRecallQueryLen is the minimum character count for a query to trigger semantic recall.
// Queries this short (e.g. "OK", "Yes", "继续", "好的") carry too little semantic signal
// to reliably match memory entries, so we skip the LLM call and save an API round-trip.
const minRecallQueryLen = 4

func shouldSkipRecall(query string, idx []persistence.MemoryIndexEntry) bool {
	if len(idx) == 0 {
		return true
	}
	// Count runes, not bytes — works correctly for CJK and other multi-byte scripts.
	if len([]rune(strings.TrimSpace(query))) <= minRecallQueryLen {
		return true
	}
	return false
}

func matchMemories(
	ctx context.Context,
	query string,
	idx []persistence.MemoryIndexEntry,
	p provider.StreamingProvider,
) ([]persistence.MemoryIndexEntry, error) {
	var candidates strings.Builder
	for i, e := range idx {
		fmt.Fprintf(&candidates, "[%d] %s: %s\n", i, e.Path, e.Summary)
	}

	userMsg := fmt.Sprintf("User question: %s\n\nCandidates:\n%s", query, candidates.String())

	resp, err := p.Chat(ctx, provider.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: recallSystemPrompt},
			{Role: "user", Content: userMsg},
		},
		Stream:    false,
		MaxTokens: 128,
	})
	if err != nil {
		return nil, fmt.Errorf("semantic recall: %w", err)
	}

	indices := parseRecallResponse(resp.Message.Content)
	if len(indices) == 0 {
		return nil, nil
	}

	var selected []persistence.MemoryIndexEntry
	for _, i := range indices {
		if i >= 0 && i < len(idx) {
			selected = append(selected, idx[i])
		}
	}
	return selected, nil
}

func parseRecallResponse(content string) []int {
	content = strings.TrimSpace(content)
	if content == "-1" || content == "" {
		return nil
	}

	// Strip any markdown formatting.
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var result []int
	parts := strings.FieldsFunc(content, func(r rune) bool {
		return r == ',' || r == ' ' || r == '，'
	})
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if n, err := strconv.Atoi(p); err == nil && n >= 0 {
			result = append(result, n)
		}
	}
	return result
}
