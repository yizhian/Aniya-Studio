package context

import (
	"fmt"

	"agentgo/internal/model"
	"agentgo/internal/observability"
)

// FinalizeSnapshot creates a single version snapshot from the current HTML state.
func (m *ContextManager) FinalizeSnapshot(title string) (int, error) {
	if m.htmlFilePath == "" {
		return 0, fmt.Errorf("no HTML file path recorded")
	}

	snapshot, err := ExtractDesignSnapshot(m.htmlFilePath)
	if err != nil {
		observability.EmitOrLog(m.emitter, observability.AgentEvent{
			Type: observability.EventError,
			Data: map[string]any{"message": "extract design snapshot: " + err.Error()},
		})
		return 0, err
	}

	newVersion, err := m.store.CreateVersion(m.htmlFilePath, m.sessionID, title, snapshot, m.latestTodos)
	if err != nil {
		observability.EmitOrLog(m.emitter, observability.AgentEvent{
			Type: observability.EventError,
			Data: map[string]any{"message": "create version: " + err.Error()},
		})
		return 0, err
	}

	m.currentVersion = newVersion
	m.latestSnapshot = snapshot

	observability.EmitOrLog(m.emitter, observability.AgentEvent{
		Type: observability.EventContextSnapshot,
		Data: map[string]any{
			"version":     newVersion,
			"title":       title,
			"slide_count": snapshot.SlideCount,
			"html_file":   m.htmlFilePath,
		},
	})

	return newVersion, nil
}

// RevertVersion marks a version as invalid, removes it, and rolls back currentVersion.
func (m *ContextManager) RevertVersion(version int) error {
	if err := m.store.MarkInvalid(version); err != nil {
		return err
	}
	m.currentVersion = m.store.DiscoverVersion()
	return nil
}

// CompressMessages replaces st.Messages with [system] + [compression summary] +
// [last 3 rounds].
func (m *ContextManager) CompressMessages(messages []model.Message, summary string) []model.Message {
	preCount := len(messages)

	savedKeep := m.keepRounds
	m.keepRounds = 3
	defer func() { m.keepRounds = savedKeep }()

	var system model.Message
	hasSystem := len(messages) > 0 && messages[0].Role == "system"
	if hasSystem {
		system = messages[0]
	}

	last := TrimMessagesWithEmitter(messages, 3, m.latestSnapshot, m.emitter)

	result := make([]model.Message, 0, 3+len(last))
	if hasSystem {
		result = append(result, system)
	}
	result = append(result, model.Message{
		Role:    "user",
		Content: summary,
	})
	for _, msg := range last {
		if msg.Role == "system" {
			continue
		}
		result = append(result, msg)
	}

	observability.EmitOrLog(m.emitter, observability.AgentEvent{
		Type: observability.EventContextCompress,
		Data: map[string]any{
			"pre_count":  preCount,
			"post_count": len(result),
		},
	})

	return result
}
