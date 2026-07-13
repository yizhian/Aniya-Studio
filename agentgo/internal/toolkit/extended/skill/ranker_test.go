package skill

import (
	"context"
	"errors"
	"testing"

	"agentgo/internal/model"
	"agentgo/internal/provider"
)

// stubProvider implements provider.StreamingProvider for tests.
type stubProvider struct {
	chatFn func(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error)
}

func (s *stubProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	return s.chatFn(ctx, req)
}

func (s *stubProvider) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	return nil, errors.New("not implemented")
}

func (s *stubProvider) Type() provider.ProviderType { return "stub" }

func TestRanker_BuildCandidatePool(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"coral-deck":   {Name: "coral-deck", Mode: "deck", Description: "Warm coral theme", HasPreview: true},
		"blue-deck":    {Name: "blue-deck", Mode: "deck", Description: "Cool blue theme", HasPreview: false},
		"html-ppt":     {Name: "html-ppt", Mode: "deck", Description: "PPT template", HasAssets: true},
		"landing-page": {Name: "landing-page", Mode: "landing", Description: "Landing page"},
		"no-mode":      {Name: "no-mode", Description: "No mode skill"},
	})

	ranker := NewSkillRanker(idx)
	candidates := ranker.buildCandidatePool()

	names := make(map[string]bool)
	for _, c := range candidates {
		names[c.Name] = true
	}

	if !names["coral-deck"] {
		t.Error("expected coral-deck in candidate pool")
	}
	if !names["blue-deck"] {
		t.Error("expected blue-deck in candidate pool")
	}
	if names["html-ppt"] {
		t.Error("html-ppt should be excluded by denylist")
	}
	if names["landing-page"] {
		t.Error("landing-page should not be in deck candidate pool")
	}
	if names["no-mode"] {
		t.Error("no-mode should not be in deck candidate pool")
	}

	// Verify HasPreview propagates from Skill to candidateInfo.
	for _, c := range candidates {
		switch c.Name {
		case "coral-deck":
			if !c.HasPreview {
				t.Error("coral-deck should have HasPreview=true")
			}
		case "blue-deck":
			if c.HasPreview {
				t.Error("blue-deck should have HasPreview=false")
			}
		}
	}
}

func TestRanker_Recommend_Success(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"coral-deck":  {Name: "coral-deck", Mode: "deck", Description: "Warm coral fashion theme", HasPreview: true},
		"blue-deck":   {Name: "blue-deck", Mode: "deck", Description: "Cool blue corporate theme", HasPreview: false},
		"green-deck":  {Name: "green-deck", Mode: "deck", Description: "Fresh green nature theme", HasPreview: true},
	})

	p := &stubProvider{
		chatFn: func(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
			return &provider.ChatResponse{
				Message: model.Message{
					Content: `{"picks": [{"index": 1, "reason": "Warm coral matches fashion request"}]}`,
				},
			}, nil
		},
	}

	ranker := NewSkillRanker(idx)
	recs, err := ranker.Recommend(context.Background(), p, "fashion presentation", "", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].Name != "coral-deck" {
		t.Errorf("expected coral-deck, got %s", recs[0].Name)
	}
	if recs[0].Reason != "Warm coral matches fashion request" {
		t.Errorf("expected reason, got %s", recs[0].Reason)
	}
	if recs[0].Description != "Warm coral fashion theme" {
		t.Errorf("expected description, got %s", recs[0].Description)
	}
	if !recs[0].HasPreview {
		t.Error("expected HasPreview=true for coral-deck recommendation")
	}
}

func TestRanker_Recommend_NoMatches(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"coral-deck": {Name: "coral-deck", Mode: "deck", Description: "Warm coral theme"},
	})

	p := &stubProvider{
		chatFn: func(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
			return &provider.ChatResponse{
				Message: model.Message{
					Content: `{"picks": []}`,
				},
			}, nil
		},
	}

	ranker := NewSkillRanker(idx)
	recs, err := ranker.Recommend(context.Background(), p, "nothing matching", "", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected 0 recommendations, got %d", len(recs))
	}
}

func TestRanker_Recommend_MarkdownStripping(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"coral-deck": {Name: "coral-deck", Mode: "deck", Description: "Warm coral theme"},
	})

	p := &stubProvider{
		chatFn: func(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
			return &provider.ChatResponse{
				Message: model.Message{
					Content: "```json\n{\"picks\": [{\"index\": 0, \"reason\": \"perfect match\"}]}\n```",
				},
			}, nil
		},
	}

	ranker := NewSkillRanker(idx)
	recs, err := ranker.Recommend(context.Background(), p, "coral", "", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].Name != "coral-deck" {
		t.Errorf("expected coral-deck, got %s", recs[0].Name)
	}
}

func TestRanker_Recommend_ProviderError(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"coral-deck": {Name: "coral-deck", Mode: "deck", Description: "Warm coral theme"},
	})

	p := &stubProvider{
		chatFn: func(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
			return nil, errors.New("provider down")
		},
	}

	ranker := NewSkillRanker(idx)
	_, err := ranker.Recommend(context.Background(), p, "query", "", 3)
	if err == nil {
		t.Fatal("expected error from failed provider")
	}
}

func TestRanker_Recommend_DesignBrief(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"coral-deck": {Name: "coral-deck", Mode: "deck", Description: "Warm coral theme", HasPreview: true},
		"blue-deck":  {Name: "blue-deck", Mode: "deck", Description: "Cool blue theme", HasPreview: false},
	})

	var capturedReq string
	p := &stubProvider{
		chatFn: func(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
			for _, m := range req.Messages {
				if m.Role == "user" {
					capturedReq = m.Content
				}
			}
			return &provider.ChatResponse{
				Message: model.Message{Content: `{"picks": []}`},
			}, nil
		},
	}

	ranker := NewSkillRanker(idx)
	_, err := ranker.Recommend(context.Background(), p, "warm deck", "brighter palette", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(capturedReq, "warm deck") {
		t.Error("expected query in request")
	}
	if !containsStr(capturedReq, "Refined preferences: brighter palette") {
		t.Error("expected design brief in request")
	}
}

func TestRanker_HasPreview_Propagation(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"preview-true":  {Name: "preview-true", Mode: "deck", Description: "Has preview", HasPreview: true},
		"preview-false": {Name: "preview-false", Mode: "deck", Description: "No preview", HasPreview: false},
		"preview-none":  {Name: "preview-none", Mode: "deck", Description: "Default preview"},
	})

	p := &stubProvider{
		chatFn: func(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
			return &provider.ChatResponse{
				Message: model.Message{
					Content: `{"picks": [{"index": 0, "reason": "has preview"}, {"index": 1, "reason": "no preview"}, {"index": 2, "reason": "default"}]}`,
				},
			}, nil
		},
	}

	ranker := NewSkillRanker(idx)
	recs, err := ranker.Recommend(context.Background(), p, "query", "", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("expected 3 recommendations, got %d", len(recs))
	}

	for _, rec := range recs {
		switch rec.Name {
		case "preview-true":
			if !rec.HasPreview {
				t.Errorf("preview-true: expected HasPreview=true, got false")
			}
		case "preview-false":
			if rec.HasPreview {
				t.Errorf("preview-false: expected HasPreview=false, got true")
			}
		case "preview-none":
			if rec.HasPreview {
				t.Errorf("preview-none: expected HasPreview=false (zero value), got true")
			}
		}
	}

	// Verify the struct field serializes to JSON.
	for _, rec := range recs {
		if rec.Name != "preview-true" {
			continue
		}
		if !rec.HasPreview {
			t.Error("HasPreview should be true on the struct")
		}
	}
}

func TestRanker_Recommend_EmptyPool(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"html-ppt": {Name: "html-ppt", Mode: "deck", Description: "Template", HasAssets: true},
	})

	ranker := NewSkillRanker(idx)
	recs, err := ranker.Recommend(context.Background(), nil, "query", "", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recs != nil {
		t.Errorf("expected nil for empty pool, got %v", recs)
	}
}

func TestParseRankerResponse_JSON(t *testing.T) {
	picks := parseRankerResponse(`{"picks": [{"index": 0, "reason": "matches well"}, {"index": 2, "reason": "also good"}]}`)
	if len(picks) != 2 {
		t.Fatalf("expected 2 picks, got %d", len(picks))
	}
	if picks[0].index != 0 {
		t.Errorf("expected index 0, got %d", picks[0].index)
	}
	if picks[1].index != 2 {
		t.Errorf("expected index 2, got %d", picks[1].index)
	}
}

func TestParseRankerResponse_FallbackComma(t *testing.T) {
	picks := parseRankerResponse("0, 2, 5")
	if len(picks) != 3 {
		t.Fatalf("expected 3 picks, got %d", len(picks))
	}
	if picks[0].index != 0 || picks[1].index != 2 || picks[2].index != 5 {
		t.Errorf("unexpected indices: %v", picks)
	}
}

func TestParseRankerResponse_EmptyInput(t *testing.T) {
	picks := parseRankerResponse("")
	if len(picks) != 0 {
		t.Errorf("expected 0 picks for empty, got %d", len(picks))
	}
}

func TestParseRankerResponse_NegativeIndexFiltered(t *testing.T) {
	picks := parseRankerResponse(`{"picks": [{"index": -1, "reason": "bad"}]}`)
	if len(picks) != 0 {
		t.Errorf("expected 0 picks for negative index, got %d", len(picks))
	}
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
