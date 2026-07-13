package core

import (
	"encoding/json"

	"agentgo/internal/toolkit/contracts"
)

// parseToolArgs unmarshals the JSON args string into a typed struct.
// Replaces the hand-rolled json.Unmarshal([]byte(args.ArgsJSON), &input) pattern.
func parseToolArgs[T any](argsJSON string) (T, error) {
	var input T
	if err := json.Unmarshal([]byte(argsJSON), &input); err != nil {
		return input, err
	}
	return input, nil
}


// newToolError creates a ToolResult representing an error.
func newToolError(code, msg string) contracts.ToolResult {
	return contracts.ToolResult{
		IsError:      true,
		ErrorCode:    code,
		ErrorMessage: msg,
	}
}
