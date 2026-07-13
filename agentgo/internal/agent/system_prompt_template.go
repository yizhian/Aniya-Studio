package agent

import _ "embed"

//go:embed prompts/system.md
var defaultSystemPromptTemplate string

func DefaultSystemPromptTemplate() string {
	return defaultSystemPromptTemplate
}
