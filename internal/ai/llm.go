package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type llmMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatRequest struct {
	Model    string     `json:"model"`
	Messages []llmMessage `json:"messages"`
	Tools    []toolDef  `json:"tools,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message llmMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (m *Manager) callLLM(ctx context.Context, messages []llmMessage, tools []toolDef) (*chatResponse, error) {
	var failures []string
	for _, model := range m.models {
		req := chatRequest{Model: model, Messages: messages, Tools: tools}
		body, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
			"https://api.groq.com/openai/v1/chat/completions",
			bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create req: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)

		resp, err := m.httpClient.Do(httpReq)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", model, err))
			continue
		}

		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
		resp.Body.Close()
		if readErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", model, readErr))
			continue
		}

		var chatResp chatResponse
		if err := json.Unmarshal(respBody, &chatResp); err != nil {
			failures = append(failures, fmt.Sprintf("%s: invalid json", model))
			continue
		}

		if resp.StatusCode == http.StatusOK && chatResp.Error == nil && len(chatResp.Choices) > 0 {
			return &chatResp, nil
		}

		msg := http.StatusText(resp.StatusCode)
		if chatResp.Error != nil && chatResp.Error.Message != "" {
			msg = chatResp.Error.Message
		}
		failures = append(failures, fmt.Sprintf("%s: %s", model, msg))

		if resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusForbidden ||
			resp.StatusCode == http.StatusNotFound ||
			resp.StatusCode >= 500 {
			continue
		}
		return nil, fmt.Errorf("model %s: %s", model, msg)
	}
	return nil, fmt.Errorf("all models failed: %s", strings.Join(failures, "; "))
}
