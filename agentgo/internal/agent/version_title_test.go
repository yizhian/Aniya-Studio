package agent

import (
	"context"
	"testing"

	"agentgo/internal/model"
	"agentgo/internal/provider"
)

// stubTitleProvider implements provider.StreamingProvider for title generation tests.
type stubTitleProvider struct {
	chatResponse *provider.ChatResponse
	chatErr      error
}

func (s *stubTitleProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	if s.chatErr != nil {
		return nil, s.chatErr
	}
	return s.chatResponse, nil
}

func (s *stubTitleProvider) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	return nil, nil
}

func (s *stubTitleProvider) Type() provider.ProviderType {
	return provider.ProviderOpenAI
}

func TestGenerateVersionTitle_Success(t *testing.T) {
	prov := &stubTitleProvider{
		chatResponse: &provider.ChatResponse{
			Message: model.Message{Content: "疫情后的经济复苏"},
		},
	}
	title := generateVersionTitle(context.Background(), prov, "做个疫情后经济复苏的ppt", nil)
	if title != "疫情后的经济复苏" {
		t.Errorf("expected '疫情后的经济复苏', got %q", title)
	}
}

func TestGenerateVersionTitle_TruncateLong(t *testing.T) {
	prov := &stubTitleProvider{
		chatResponse: &provider.ChatResponse{
			Message: model.Message{Content: "这是一段非常长的标题，超过十五个字符的限制"},
		},
	}
	title := generateVersionTitle(context.Background(), prov, "做个很长的ppt", nil)
	if len([]rune(title)) > 15 {
		t.Errorf("expected title <= 15 runes, got %d runes: %q", len([]rune(title)), title)
	}
}

func TestGenerateVersionTitle_NilProvider(t *testing.T) {
	title := generateVersionTitle(context.Background(), nil, "做个ppt", nil)
	if title != "" {
		t.Errorf("expected empty title for nil provider, got %q", title)
	}
}

func TestGenerateVersionTitle_EmptyPrompt(t *testing.T) {
	prov := &stubTitleProvider{
		chatResponse: &provider.ChatResponse{
			Message: model.Message{Content: "标题"},
		},
	}
	title := generateVersionTitle(context.Background(), prov, "", nil)
	if title != "" {
		t.Errorf("expected empty title for empty prompt, got %q", title)
	}
}

func TestGenerateVersionTitle_ProviderError(t *testing.T) {
	prov := &stubTitleProvider{
		chatErr: context.DeadlineExceeded,
	}
	title := generateVersionTitle(context.Background(), prov, "做个ppt", nil)
	if title != "" {
		t.Errorf("expected empty title on provider error, got %q", title)
	}
}

func TestGenerateVersionTitle_StripQuotes(t *testing.T) {
	prov := &stubTitleProvider{
		chatResponse: &provider.ChatResponse{
			Message: model.Message{Content: `"经济复苏分析"`},
		},
	}
	title := generateVersionTitle(context.Background(), prov, "做个经济复苏的ppt", nil)
	if title != "经济复苏分析" {
		t.Errorf("expected quotes stripped, got %q", title)
	}
}
