package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"agentgo/internal/retry"
	"agentgo/internal/toolkit/contracts"
)

const (
	webFetchTimeout          = 30 * time.Second
	webFetchMaxBodyBytes     = 50 * 1024
	webFetchMaxDiscardBytes  = 64 << 10
	webFetchMaxResultChars   = 48000
)

var htmlTagRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>|<[^>]+>`)

// WebFetchTool 拉取 URL 内容（只读），HTML 去标签并截断体积。
type WebFetchTool struct {
	client *http.Client
}

func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		client: &http.Client{Timeout: webFetchTimeout},
	}
}

func (t *WebFetchTool) Descriptor() contracts.ToolDescriptor {
	return contracts.ToolDescriptor{
		Name:               "web_fetch",
		Description:        "Fetch a URL over HTTP/HTTPS, strip HTML tags, return plain text up to 50KB of raw response body.",
		Prompt:             "Use web_fetch for public documentation or API examples; respect robots and rate limits.",
		MaxResultSizeChars: webFetchMaxResultChars,
		InputJSONSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "http or https URL",
				},
			},
			"required": []any{"url"},
		},
		Flags: contracts.ToolBehaviorFlags{
			ConcurrencySafe: true,
			ReadOnly:        true,
		},
	}
}

func (t *WebFetchTool) Call(ctx context.Context, args contracts.ToolCallArgs) contracts.ToolResult {
	var input struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(args.ArgsJSON), &input); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_json", ErrorMessage: err.Error()}
	}
	u := strings.TrimSpace(input.URL)
	if u == "" {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_url", ErrorMessage: "url is required"}
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_url", ErrorMessage: "only http/https URLs are allowed"}
	}


	var result contracts.ToolResult
	err := retry.Do(ctx, 2, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "agentgo-web-fetch/1.0")

		resp, err := t.client.Do(req)
		if err != nil {
			return err // transport/timeout → existing isRetryable covers net.Error
		}
		defer resp.Body.Close()

		// ORDER IS LOAD-BEARING: check 5xx/429/408 FIRST, then 4xx terminal.
		if retry.IsRetryableHTTPStatus(resp.StatusCode) {
			_, _ = io.CopyN(io.Discard, resp.Body, webFetchMaxDiscardBytes)
			return &retry.RetryableHTTPError{Code: resp.StatusCode}
		}
		if resp.StatusCode >= 400 {
			_, _ = io.CopyN(io.Discard, resp.Body, webFetchMaxDiscardBytes)
			return fmt.Errorf("non-retryable HTTP status %d", resp.StatusCode)
		}

		// 2xx: read and process body.
		limited := io.LimitReader(resp.Body, webFetchMaxBodyBytes+1)
		raw, err := io.ReadAll(limited)
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}
		truncated := len(raw) > webFetchMaxBodyBytes
		if truncated {
			raw = raw[:webFetchMaxBodyBytes]
		}

		ct := resp.Header.Get("Content-Type")
		body := string(raw)
		if strings.Contains(strings.ToLower(ct), "html") || looksLikeHTML(body) {
			body = htmlTagRe.ReplaceAllString(body, " ")
			body = strings.Join(strings.Fields(body), " ")
		}
		if !utf8.ValidString(body) {
			body = strings.ToValidUTF8(body, "")
		}

		meta := map[string]any{
			"url":            u,
			"status":         resp.StatusCode,
			"content_type":   ct,
			"truncated_50kb": truncated,
		}
		if len(body) > webFetchMaxResultChars {
			body = body[:webFetchMaxResultChars] + "\n\n[truncated for tool output cap]"
			meta["truncated_output"] = true
		}

		result = contracts.ToolResult{Content: body, Metadata: meta}
		return nil
	})
	if err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "fetch_failed", ErrorMessage: err.Error()}
	}
	return result
}

func looksLikeHTML(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 20 {
		return false
	}
	return strings.Contains(s, "<html") || strings.Contains(s, "<HTML") || strings.Contains(s, "<body") || strings.Contains(s, "<BODY")
}
