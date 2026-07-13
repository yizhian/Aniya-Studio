package agent

import (
	"context"
	"fmt"
	"testing"

	"agentgo/internal/model"
	"agentgo/internal/persistence"
	"agentgo/internal/provider"
)

func TestParseRecallResponse_SingleIndex(t *testing.T) {
	result := parseRecallResponse("0")
	if len(result) != 1 || result[0] != 0 {
		t.Errorf("expected [0], got %v", result)
	}
}

func TestParseRecallResponse_NegativeNumbers(t *testing.T) {
	result := parseRecallResponse("0,-1,2")
	if len(result) != 2 {
		t.Fatalf("expected 2 indices (negative filtered out), got %d", len(result))
	}
	if result[0] != 0 || result[1] != 2 {
		t.Errorf("expected [0,2], got %v", result)
	}
}

func TestParseRecallResponse_NonNumeric(t *testing.T) {
	result := parseRecallResponse("abc,def")
	if len(result) != 0 {
		t.Errorf("expected 0 indices for non-numeric, got %d", len(result))
	}
}

func TestParseRecallResponse_MixedValidInvalid(t *testing.T) {
	result := parseRecallResponse("0,invalid,2,xyz,4")
	if len(result) != 3 {
		t.Fatalf("expected 3 valid indices, got %d", len(result))
	}
	if result[0] != 0 || result[1] != 2 || result[2] != 4 {
		t.Errorf("expected [0,2,4], got %v", result)
	}
}

type fakeStreamingProvider struct {
	chatResp *provider.ChatResponse
	chatErr  error
}

func (f *fakeStreamingProvider) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeStreamingProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	return f.chatResp, f.chatErr
}

func (f *fakeStreamingProvider) Type() provider.ProviderType {
	return provider.ProviderOpenAI
}

func TestMatchMemories_Success(t *testing.T) {
	fake := &fakeStreamingProvider{
		chatResp: &provider.ChatResponse{
			Message: model.Message{Content: "0,2"},
		},
	}
	idx := []persistence.MemoryIndexEntry{
		{Path: "a.md", Summary: "Memory A"},
		{Path: "b.md", Summary: "Memory B"},
		{Path: "c.md", Summary: "Memory C"},
	}
	selected, err := matchMemories(context.Background(), "find a and c", idx, fake)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected, got %d", len(selected))
	}
	if selected[0].Path != "a.md" {
		t.Errorf("expected a.md, got %s", selected[0].Path)
	}
	if selected[1].Path != "c.md" {
		t.Errorf("expected c.md, got %s", selected[1].Path)
	}
}

func TestMatchMemories_NoMatch(t *testing.T) {
	fake := &fakeStreamingProvider{
		chatResp: &provider.ChatResponse{
			Message: model.Message{Content: "-1"},
		},
	}
	idx := []persistence.MemoryIndexEntry{
		{Path: "a.md", Summary: "Memory A"},
	}
	selected, err := matchMemories(context.Background(), "find nothing", idx, fake)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 0 {
		t.Errorf("expected 0 selected for -1, got %d", len(selected))
	}
}

func TestMatchMemories_OutOfBoundsIndex(t *testing.T) {
	fake := &fakeStreamingProvider{
		chatResp: &provider.ChatResponse{
			Message: model.Message{Content: "0,5"},
		},
	}
	idx := []persistence.MemoryIndexEntry{
		{Path: "a.md", Summary: "Memory A"},
	}
	selected, err := matchMemories(context.Background(), "test", idx, fake)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 {
		t.Fatalf("expected 1 selected (only index 0 valid), got %d", len(selected))
	}
	if selected[0].Path != "a.md" {
		t.Errorf("expected a.md, got %s", selected[0].Path)
	}
}

func TestMatchMemories_ErrorPath(t *testing.T) {
	// stubStreamingProvider.Chat always returns "not implemented" error.
	badProvider := &stubStreamingProvider{}
	idx := []persistence.MemoryIndexEntry{
		{Path: "test.md", Summary: "Test"},
	}
	_, err := matchMemories(context.Background(), "test query", idx, badProvider)
	if err == nil {
		t.Error("expected error from Chat on stub")
	}
}

func TestSemanticRecall_SkipsShortQuery(t *testing.T) {
	idx := []persistence.MemoryIndexEntry{
		{Path: "test.md", Summary: "Test"},
	}
	result, err := SemanticRecall(context.Background(), "OK", "", nil, idx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil result for skipped recall")
	}
}

func TestSemanticRecall_EmptyIndex(t *testing.T) {
	result, err := SemanticRecall(context.Background(), "hello world", "", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil result for empty index")
	}
}

// ---------------------------------------------------------------------------
// Semantic recall: shouldSkipRecall
// ---------------------------------------------------------------------------

func TestShouldSkipRecall_EmptyIndex(t *testing.T) {
	if !shouldSkipRecall("any query here", nil) {
		t.Error("should skip when index is empty")
	}
}

func TestShouldSkipRecall_ChineseQuery(t *testing.T) {
	idx := []persistence.MemoryIndexEntry{{Path: "design/theme.md"}}
	if shouldSkipRecall("把Hero背景换成视频", idx) {
		t.Error("Chinese query should NOT be skipped")
	}
	if shouldSkipRecall("怎么优化性能", idx) {
		t.Error("Chinese query should NOT be skipped")
	}
}

func TestShouldSkipRecall_ShortQuery(t *testing.T) {
	idx := []persistence.MemoryIndexEntry{{Path: "design/theme.md"}}
	if !shouldSkipRecall("OK", idx) {
		t.Error("'OK' should be skipped")
	}
	if !shouldSkipRecall("Yes", idx) {
		t.Error("'Yes' should be skipped")
	}
	if !shouldSkipRecall("继续", idx) {
		t.Error("'继续' (2 runes) should be skipped")
	}
	if !shouldSkipRecall("好的", idx) {
		t.Error("'好的' (2 runes) should be skipped")
	}
}

func TestShouldSkipRecall_LongQuery(t *testing.T) {
	idx := []persistence.MemoryIndexEntry{{Path: "design/theme.md"}}
	if shouldSkipRecall("Make the hero banner use a video background", idx) {
		t.Error("English long query should NOT be skipped")
	}
	if shouldSkipRecall("优化一下首页的加载性能", idx) {
		t.Error("Chinese long query should NOT be skipped")
	}
}

func TestShouldSkipRecall_JapaneseQuery(t *testing.T) {
	idx := []persistence.MemoryIndexEntry{{Path: "design/theme.md"}}
	if shouldSkipRecall("デザインを改善してください", idx) {
		t.Error("Japanese query should NOT be skipped")
	}
}

func TestSemanticRecall_ErrorFromProvider(t *testing.T) {
	idx := []persistence.MemoryIndexEntry{
		{Path: "test.md", Summary: "Test memory"},
	}
	provider := &stubStreamingProvider{}
	_, err := SemanticRecall(context.Background(), "long enough query", "", nil, idx, provider)
	if err == nil {
		t.Error("expected error from stub provider Chat")
	}
}
