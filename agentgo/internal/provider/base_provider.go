package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agentgo/internal/observability"
	"agentgo/internal/retry"
)

// baseProvider holds fields shared by all StreamingProvider implementations.
type baseProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
	emitter *observability.Emitter
}

// providerAdapter bridges the common base with provider-specific behavior.
type providerAdapter interface {
	// buildBody builds the JSON request body for the provider's API.
	buildBody(req ChatRequest, stream bool) map[string]any
	// setHeaders sets provider-specific headers on the HTTP request.
	setHeaders(req *http.Request)
	// setStreamHeaders sets additional headers for streaming requests (optional).
	setStreamHeaders(req *http.Request)
	// endpoint returns the API path (e.g. "/v1/chat/completions").
	endpoint() string
	// providerName returns a prefix for error messages (e.g. "openai", "anthropic").
	providerName() string
}

// doChatRequest executes a non-streaming Chat request against the provider.
// It handles retry logic, status checking, and error wrapping.
func (b *baseProvider) doChatRequest(ctx context.Context, req ChatRequest, a providerAdapter, decodeJSON func(io.Reader) (*ChatResponse, error)) (*ChatResponse, error) {
	body := a.buildBody(req, false)
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%s marshal: %w", a.providerName(), err)
	}

	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+a.endpoint(), bytes.NewReader(raw))
	a.setHeaders(httpReq)

	start := time.Now()
	httpResp, err := b.doRetryCall(ctx, httpReq, a)
	if err != nil {
		b.emitProviderResponse(a.providerName(), req.Model, 0, time.Since(start), 0, err)
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		b.emitProviderResponse(a.providerName(), req.Model, httpResp.StatusCode, time.Since(start), 0, nil)
		return nil, b.handleHTTPError(httpResp, a)
	}

	resp, err := decodeJSON(httpResp.Body)
	if resp != nil {
		b.emitProviderResponse(a.providerName(), req.Model, httpResp.StatusCode, time.Since(start), resp.Usage.TotalTokens, nil)
	}
	return resp, err
}

// doStreamRequest executes a streaming Chat request with retry on network errors.
// HTTP status errors are not retried — they are returned to the caller immediately.
func (b *baseProvider) doStreamRequest(ctx context.Context, req ChatRequest, a providerAdapter) (*http.Response, error) {
	body := a.buildBody(req, true)
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("%s marshal: %w", a.providerName(), err)
	}

	start := time.Now()
	msgCount := len(req.Messages)
	toolCount := len(req.Tools)
	b.emitProviderRequest(a.providerName(), req.Model, msgCount, toolCount)

	var httpResp *http.Response
	err = retry.Do(ctx, 2, func() error {
		// Recreate the body reader for each attempt (consumed by client.Do).
		bodyReader := bytes.NewReader(raw)
		httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+a.endpoint(), bodyReader)
		a.setHeaders(httpReq)
		a.setStreamHeaders(httpReq)

		var innerErr error
		httpResp, innerErr = b.client.Do(httpReq)
		if innerErr != nil {
			b.emitProviderRetry(a.providerName(), req.Model, innerErr)
		}
		return innerErr
	})
	if err != nil {
		b.emitProviderResponse(a.providerName(), req.Model, 0, time.Since(start), 0, err)
		return nil, fmt.Errorf("%s stream chat: %w", a.providerName(), err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		b.emitProviderResponse(a.providerName(), req.Model, httpResp.StatusCode, time.Since(start), 0, nil)
		return nil, fmt.Errorf("%s HTTP %d: %s", a.providerName(), httpResp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	// Note: token usage is emitted after the stream completes (in streaming_loop.go)
	return httpResp, nil
}

// doRetryCall performs a retryable HTTP call with standard error wrapping.
func (b *baseProvider) doRetryCall(ctx context.Context, httpReq *http.Request, a providerAdapter) (*http.Response, error) {
	var httpResp *http.Response
	err := retry.Do(ctx, 2, func() error {
		var innerErr error
		httpResp, innerErr = b.client.Do(httpReq)
		if innerErr != nil {
			return innerErr
		}
		if retry.IsRetryableHTTPStatus(httpResp.StatusCode) {
			return &retry.RetryableHTTPError{Code: httpResp.StatusCode}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%s chat: %w", a.providerName(), err)
	}
	return httpResp, nil
}

// handleHTTPError reads the error body and wraps it in a provider-specific error.
func (b *baseProvider) handleHTTPError(httpResp *http.Response, a providerAdapter) error {
	bodyBytes, _ := io.ReadAll(httpResp.Body)
	bodyStr := strings.TrimSpace(string(bodyBytes))
	wrapped := fmt.Errorf("%s HTTP %d: %s", a.providerName(), httpResp.StatusCode, bodyStr)
	if retry.IsRetryableHTTPStatus(httpResp.StatusCode) {
		return fmt.Errorf("%w: %w", wrapped, &retry.RetryableHTTPError{Code: httpResp.StatusCode})
	}
	return wrapped
}

// emitProviderRequest emits a request-start event.
func (b *baseProvider) emitProviderRequest(providerName, model string, msgCount, toolCount int) {
	observability.EmitOrLog(b.emitter, observability.AgentEvent{
		Type: observability.EventProviderRequest,
		Data: map[string]any{
			"provider":      providerName,
			"model":         model,
			"message_count": msgCount,
			"tool_count":    toolCount,
		},
	})
}

// emitProviderResponse emits a response event with timing and token info.
func (b *baseProvider) emitProviderResponse(providerName, model string, status int, dur time.Duration, totalTokens int, err error) {
	data := map[string]any{
		"provider":    providerName,
		"model":       model,
		"duration_ms": dur.Milliseconds(),
	}
	if status > 0 {
		data["status"] = status
	}
	if totalTokens > 0 {
		data["total_tokens"] = totalTokens
	}
	if err != nil {
		data["error"] = err.Error()
	}
	observability.EmitOrLog(b.emitter, observability.AgentEvent{
		Type: observability.EventProviderResponse,
		Data: data,
	})
}

// emitProviderRetry emits a retry event.
func (b *baseProvider) emitProviderRetry(providerName, model string, err error) {
	observability.EmitOrLog(b.emitter, observability.AgentEvent{
		Type: observability.EventProviderRetry,
		Data: map[string]any{
			"provider": providerName,
			"model":    model,
			"error":    err.Error(),
		},
	})
}
