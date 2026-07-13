package contracts

import (
	"context"
	"encoding/json"
	"testing"

	"agentgo/internal/model"
)

func TestToolResult_Defaults(t *testing.T) {
	r := ToolResult{}
	if r.Content != "" {
		t.Error("Content should default to empty")
	}
	if r.IsError {
		t.Error("IsError should default to false")
	}
	if r.ErrorCode != "" {
		t.Error("ErrorCode should default to empty")
	}
	if r.ErrorMessage != "" {
		t.Error("ErrorMessage should default to empty")
	}
	if r.Metadata != nil {
		t.Error("Metadata should default to nil")
	}
}

func TestToolResult_Error(t *testing.T) {
	r := ToolResult{
		Content:      "something went wrong",
		IsError:      true,
		ErrorCode:    "ERR_TIMEOUT",
		ErrorMessage: "request timed out after 30s",
	}
	if !r.IsError {
		t.Fatal("expected IsError=true")
	}
	if r.ErrorCode != "ERR_TIMEOUT" {
		t.Fatalf("expected ERR_TIMEOUT, got %s", r.ErrorCode)
	}
}

func TestToolResult_Success(t *testing.T) {
	r := ToolResult{
		Content: "file written successfully",
		IsError: false,
		Metadata: map[string]any{
			"bytes_written": 1024,
		},
	}
	if r.IsError {
		t.Fatal("expected IsError=false")
	}
	if r.Content != "file written successfully" {
		t.Fatalf("unexpected content: %s", r.Content)
	}
	if v, ok := r.Metadata["bytes_written"].(int); !ok || v != 1024 {
		t.Fatalf("unexpected metadata: %v", r.Metadata)
	}
}

func TestToolResult_JSONRoundTrip(t *testing.T) {
	r := ToolResult{
		Content:      "done",
		IsError:      true,
		ErrorCode:    "ERR_UNKNOWN",
		ErrorMessage: "unknown error",
		Metadata:     map[string]any{"key": "val"},
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ToolResult
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Content != "done" {
		t.Errorf("Content mismatch: %s", decoded.Content)
	}
	if !decoded.IsError {
		t.Error("IsError mismatch")
	}
	if decoded.ErrorCode != "ERR_UNKNOWN" {
		t.Errorf("ErrorCode mismatch: %s", decoded.ErrorCode)
	}
	if decoded.ErrorMessage != "unknown error" {
		t.Errorf("ErrorMessage mismatch: %s", decoded.ErrorMessage)
	}
	if decoded.Metadata["key"] != "val" {
		t.Error("Metadata mismatch")
	}
}

func TestProgressEvent_Fields(t *testing.T) {
	ev := ProgressEvent{
		Stage:   "writing",
		Message: "writing index.html...",
		Data:    map[string]any{"file": "index.html", "bytes": 512},
	}
	if ev.Stage != "writing" {
		t.Errorf("Stage mismatch: %s", ev.Stage)
	}
	if ev.Message != "writing index.html..." {
		t.Errorf("Message mismatch: %s", ev.Message)
	}
	if v, ok := ev.Data["file"].(string); !ok || v != "index.html" {
		t.Errorf("Data[file] mismatch: %v", ev.Data["file"])
	}
}

func TestProgressEvent_JSONRoundTrip(t *testing.T) {
	ev := ProgressEvent{
		Stage:   "searching",
		Message: "searching for TODO patterns",
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ProgressEvent
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Stage != "searching" {
		t.Errorf("Stage mismatch: %s", decoded.Stage)
	}
	if decoded.Message != "searching for TODO patterns" {
		t.Errorf("Message mismatch: %s", decoded.Message)
	}
}

func TestToolCallContext_Fields(t *testing.T) {
	ctx := ToolCallContext{
		SessionID:      "sess-1",
		UserID:         "user-99",
		WorkspacePath:  "/tmp/ws",
		Round:          3,
		ConversationID: "conv-42",
		Metadata:       map[string]any{"source": "test"},
	}
	if ctx.SessionID != "sess-1" {
		t.Errorf("SessionID mismatch: %s", ctx.SessionID)
	}
	if ctx.UserID != "user-99" {
		t.Errorf("UserID mismatch: %s", ctx.UserID)
	}
	if ctx.Round != 3 {
		t.Errorf("Round mismatch: %d", ctx.Round)
	}
	if ctx.ConversationID != "conv-42" {
		t.Errorf("ConversationID mismatch: %s", ctx.ConversationID)
	}
	if ctx.Metadata["source"] != "test" {
		t.Errorf("Metadata mismatch: %v", ctx.Metadata)
	}
}

func TestToolDescriptor_AllFields(t *testing.T) {
	jsonSchema := InputJSONSchema{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string"},
		},
	}
	td := ToolDescriptor{
		Name:               "read_file",
		Aliases:            []string{"cat", "read"},
		Description:        "Read contents of a file",
		Prompt:             "Use read_file to read file contents",
		MaxResultSizeChars: 10000,
		InputJSONSchema:    jsonSchema,
		Flags:              ToolBehaviorFlags{ReadOnly: true, ConcurrencySafe: true},
	}
	if td.Name != "read_file" {
		t.Errorf("Name mismatch: %s", td.Name)
	}
	if len(td.Aliases) != 2 || td.Aliases[0] != "cat" {
		t.Errorf("Aliases mismatch: %v", td.Aliases)
	}
	if !td.Flags.ReadOnly {
		t.Error("expected ReadOnly flag")
	}
	if !td.Flags.ConcurrencySafe {
		t.Error("expected ConcurrencySafe flag")
	}
	if td.MaxResultSizeChars != 10000 {
		t.Errorf("MaxResultSizeChars mismatch: %d", td.MaxResultSizeChars)
	}
}

func TestToolCallArgs_Fields(t *testing.T) {
	progressCalls := make([]ProgressEvent, 0)
	onProgress := func(ev ProgressEvent) {
		progressCalls = append(progressCalls, ev)
	}

	args := ToolCallArgs{
		ArgsJSON:      `{"path": "index.html"}`,
		Context:       ToolCallContext{SessionID: "sess-2", Round: 1},
		CanUseTool:    true,
		ParentMessage: model.Message{Role: "user", Content: "read index.html"},
		OnProgress:    onProgress,
	}
	if args.ArgsJSON != `{"path": "index.html"}` {
		t.Errorf("ArgsJSON mismatch: %s", args.ArgsJSON)
	}
	if !args.CanUseTool {
		t.Error("CanUseTool should be true")
	}
	if args.ParentMessage.Role != "user" {
		t.Errorf("Role mismatch: %s", args.ParentMessage.Role)
	}
	if args.OnProgress == nil {
		t.Error("OnProgress should not be nil")
	}

	// Fire progress callback.
	args.OnProgress(ProgressEvent{Stage: "started"})
	args.OnProgress(ProgressEvent{Stage: "done"})
	if len(progressCalls) != 2 {
		t.Errorf("expected 2 progress calls, got %d", len(progressCalls))
	}
}

func TestToolBehaviorFlags_Defaults(t *testing.T) {
	f := ToolBehaviorFlags{}
	if f.ConcurrencySafe {
		t.Error("ConcurrencySafe should default to false")
	}
	if f.ReadOnly {
		t.Error("ReadOnly should default to false")
	}
	if f.Destructive {
		t.Error("Destructive should default to false")
	}
	if f.RequiresAuth {
		t.Error("RequiresAuth should default to false")
	}
	if f.Deferred {
		t.Error("Deferred should default to false")
	}
}

func TestToolBehaviorFlags_DestructiveTool(t *testing.T) {
	f := ToolBehaviorFlags{
		Destructive:  true,
		RequiresAuth: true,
	}
	if !f.Destructive {
		t.Error("Destructive should be true")
	}
	if !f.RequiresAuth {
		t.Error("RequiresAuth should be true")
	}
	if f.ReadOnly {
		t.Error("Destructive tool should not be ReadOnly")
	}
}

func TestInputJSONSchema_TypedAlias(t *testing.T) {
	var s InputJSONSchema = map[string]any{
		"type":       "object",
		"required":   []string{"pattern"},
		"properties": map[string]any{"pattern": map[string]any{"type": "string"}},
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["type"] != "object" {
		t.Errorf("type mismatch: %v", decoded["type"])
	}
}

// dummyTool implements the Tool interface for compile-time verification.
type dummyTool struct {
	name string
}

func (d dummyTool) Descriptor() ToolDescriptor {
	return ToolDescriptor{Name: d.name}
}

func (d dummyTool) Call(_ context.Context, _ ToolCallArgs) ToolResult {
	return ToolResult{Content: "ok"}
}

func TestToolInterface_Implementation(t *testing.T) {
	var t1 Tool = dummyTool{name: "dummy"}
	td := t1.Descriptor()
	if td.Name != "dummy" {
		t.Errorf("Name mismatch: %s", td.Name)
	}
	result := t1.Call(context.Background(), ToolCallArgs{})
	if result.Content != "ok" {
		t.Errorf("Content mismatch: %s", result.Content)
	}
}

func TestToolInterface_NilOnProgressSafe(t *testing.T) {
	// Calling with nil OnProgress should not panic.
	dt := dummyTool{name: "safe"}
	args := ToolCallArgs{
		ArgsJSON:   `{}`,
		CanUseTool: true,
	}
	// This must not panic.
	result := dt.Call(context.Background(), args)
	if result.Content != "ok" {
		t.Error("unexpected result")
	}
}
