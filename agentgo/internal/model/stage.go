package model

// Stage classifies the current phase of a conversation.
type Stage string

const (
	StageInitialGeneration Stage = "initial_generation"
	StageIterativeEdit     Stage = "iterative_edit"
)
