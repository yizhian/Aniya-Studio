package context

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"agentgo/internal/model"
	"agentgo/internal/observability"
	"agentgo/internal/persistence"
)

// contextWindow is the assumed context window size in tokens (approx 128k).
const contextWindow = 128000

// AssembleMessages builds the LLM-bound message array with budget-aware memory injection
// and progress summary.
func (m *ContextManager) AssembleMessages(full []model.Message) []model.Message {
	if m.latestSnapshot != nil {
		m.latestSnapshot.ActiveFile = m.activeFile
	}
	result := TrimMessagesWithEmitter(full, m.keepRounds, m.latestSnapshot, m.emitter)

	if progress := FormatProgressSummary(m.currentVersion, m.latestTodos); progress != "" {
		result = injectProgressAfterSnapshot(result, progress)
	}

	if len(m.recalled) > 0 {
		result = m.applyBudget(result)
		result = injectMemoryContext(result, m.recalled)
	}

	return result
}

func (m *ContextManager) applyBudget(messages []model.Message) []model.Message {
	currentEstimate := EstimateMessageTokens(messages)
	recallEstimate := estimateRecallTokens(m.recalled)
	budget := int(float64(contextWindow) * 0.85)

	if currentEstimate+recallEstimate <= budget {
		return messages
	}

	originalKeep := m.keepRounds
	originalRecallCount := len(m.recalled)

	if currentEstimate+recallEstimate < int(float64(budget)*1.1) {
		newKeep := max(1, m.keepRounds/2)
		if newKeep != m.keepRounds {
			m.keepRounds = newKeep
		}
	} else {
		m.recalled = prioritiseMemory(m.recalled, budget-currentEstimate)
		newKeep := max(1, m.keepRounds-1)
		if newKeep != m.keepRounds {
			m.keepRounds = newKeep
		}
	}

	if originalKeep != m.keepRounds || originalRecallCount != len(m.recalled) {
		observability.EmitOrLog(m.emitter, observability.AgentEvent{
			Type: observability.EventContextBudget,
			Data: map[string]any{
				"total_tokens_est":   currentEstimate,
				"recall_tokens_est":  recallEstimate,
				"budget":             budget,
				"keep_rounds_before": originalKeep,
				"keep_rounds_after":  m.keepRounds,
				"memories_before":    originalRecallCount,
				"memories_after":     len(m.recalled),
			},
		})
	}

	return messages
}

func memoryPriority(t persistence.MemoryType) int {
	switch t {
	case persistence.MemoryTypeFeedback:
		return 0
	case persistence.MemoryTypeDesign:
		return 1
	case persistence.MemoryTypeComponent:
		return 2
	case persistence.MemoryTypeTask:
		return 3
	default:
		return 4
	}
}

func prioritiseMemory(recalled []persistence.RecalledMemory, budget int) []persistence.RecalledMemory {
	sort.Slice(recalled, func(i, j int) bool {
		return memoryPriority(recalled[i].Type) < memoryPriority(recalled[j].Type)
	})
	var allowed []persistence.RecalledMemory
	used := 0
	for _, r := range recalled {
		if used+len(r.Content) > budget {
			break
		}
		allowed = append(allowed, r)
		used += len(r.Content)
	}
	return allowed
}

// EstimateMessageTokens approximates token count from byte length.
func EstimateMessageTokens(messages []model.Message) int {
	raw, err := json.Marshal(messages)
	if err != nil {
		return 0
	}
	return len(raw) / 4
}

func estimateRecallTokens(recalled []persistence.RecalledMemory) int {
	total := 0
	for _, r := range recalled {
		total += len(r.Content)
	}
	return total / 4
}

func injectMemoryContext(messages []model.Message, recalled []persistence.RecalledMemory) []model.Message {
	if len(recalled) == 0 {
		return messages
	}

	memoryMsg := model.Message{
		Role:    "user",
		Content: FormatMemoryContext(recalled),
	}

	result := make([]model.Message, 0, len(messages)+1)
	if len(messages) > 0 && messages[0].Role == "system" {
		result = append(result, messages[0])
		result = append(result, memoryMsg)
		result = append(result, messages[1:]...)
	} else {
		result = append(result, memoryMsg)
		result = append(result, messages...)
	}
	return result
}

func injectProgressAfterSnapshot(messages []model.Message, progress string) []model.Message {
	if len(messages) == 0 {
		return messages
	}
	progressMsg := model.Message{
		Role:    "user",
		Content: progress,
	}
	result := make([]model.Message, 0, len(messages)+1)
	if len(messages) > 0 && messages[0].Role == "system" {
		result = append(result, messages[0])
		if len(messages) > 1 && isSnapshotMessage(messages[1].Content) {
			result = append(result, messages[1])
			result = append(result, progressMsg)
			result = append(result, messages[2:]...)
		} else {
			result = append(result, progressMsg)
			result = append(result, messages[1:]...)
		}
	} else {
		result = append(result, progressMsg)
		result = append(result, messages...)
	}
	return result
}

// FormatMemoryContext formats recalled memories for injection into the context.
func FormatMemoryContext(recalled []persistence.RecalledMemory) string {
	if len(recalled) == 0 {
		return ""
	}

	groups := make(map[persistence.MemoryType][]persistence.RecalledMemory)
	for _, r := range recalled {
		groups[r.Type] = append(groups[r.Type], r)
	}

	var b strings.Builder
	b.WriteString("[Memory Context]\n")
	b.WriteString("Here are relevant memories from previous sessions. Use them to inform your response.\n")

	order := []persistence.MemoryType{
		persistence.MemoryTypeComponent,
		persistence.MemoryTypeFeedback,
		persistence.MemoryTypeDesign,
		persistence.MemoryTypeTask,
	}

	for _, t := range order {
		items, ok := groups[t]
		if !ok {
			continue
		}
		b.WriteString("\n### ")
		switch t {
		case persistence.MemoryTypeComponent:
			b.WriteString("Related Components\n")
		case persistence.MemoryTypeFeedback:
			b.WriteString("Related Feedback\n")
		case persistence.MemoryTypeDesign:
			b.WriteString("Related Design Decisions\n")
		case persistence.MemoryTypeTask:
			b.WriteString("Related Tasks\n")
		default:
			b.WriteString("Other\n")
		}

		for _, r := range items {
			warning := ""
			if r.DaysAgo > 1 {
				warning = fmt.Sprintf(" (⚠️ %d days old — verify this still applies)", r.DaysAgo)
			}
			b.WriteString(fmt.Sprintf("- %s%s\n  %s\n", r.Summary, warning, truncateForDisplay(r.Content, 300)))
		}
	}

	b.WriteString("\n[/Memory Context]")
	return b.String()
}

func truncateForDisplay(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// DetectHTMLModification checks whether any tool call in this round wrote or edited
// an HTML file. If so, it normalizes the file and updates the tracked HTML path.
func (m *ContextManager) DetectHTMLModification(toolCalls []model.ToolCall, toolResults []ToolExecSummary) bool {
	detectedPath, toolID := m.detectHTMLWrite(toolCalls)
	if toolID == "" {
		return false
	}
	if !m.toolSucceeded(toolID, toolResults) {
		return false
	}

	targetFile := detectedPath
	if targetFile == "" {
		targetFile = m.activeFile
	}
	if targetFile == "" {
		return false
	}

	if targetFile != m.activeFile {
		m.activeFile = targetFile
	}

	htmlPath := targetFile
	if !filepath.IsAbs(htmlPath) {
		htmlPath = filepath.Join(filepath.Dir(m.store.BaseDir()), htmlPath)
	}

	if m.htmlFilePath == "" {
		m.htmlFilePath = htmlPath
	} else if !sameFile(m.htmlFilePath, htmlPath) {
		m.htmlFilePath = htmlPath
	}

	return true
}

func (m *ContextManager) toolSucceeded(toolCallID string, results []ToolExecSummary) bool {
	for _, r := range results {
		if r.ToolCallID == toolCallID {
			return r.Success
		}
	}
	return false
}

func (m *ContextManager) detectHTMLWrite(toolCalls []model.ToolCall) (path, toolCallID string) {
	for i := range toolCalls {
		name := toolCalls[i].Function.Name
		if name != "write_file" && name != "edit_file" {
			continue
		}
		args := toolCalls[i].Function.Arguments
		p := extractPath(args)
		if p == "" {
			continue
		}
		if strings.HasSuffix(strings.ToLower(p), ".html") {
			return filepath.Clean(p), toolCalls[i].ID
		}
	}
	return "", ""
}

func extractPath(argsJSON string) string {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return ""
	}
	return args.Path
}

func sameFile(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
