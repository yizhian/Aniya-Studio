package hook

import "fmt"

// BlockedError is returned when a hook blocks an operation.
// It carries the hook name and a human-readable reason.
type BlockedError struct {
	HookName string
	Reason   string
}

func (e *BlockedError) Error() string {
	return formatBlockMessage(e.HookName, e.Reason)
}

func formatBlockMessage(hookName, reason string) string {
	return fmt.Sprintf(
		"[Blocked] %s\n\nOperation blocked. Reason: %s\n\nCorrect the issue and retry.",
		hookName, reason,
	)
}

func formatWarning(hookName, reason string) string {
	return fmt.Sprintf(
		"[System] %s\n%s",
		hookName, reason,
	)
}
