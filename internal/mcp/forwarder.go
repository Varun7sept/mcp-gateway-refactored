package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/varunbanda/mcp-gateway/internal/common"
)

var forwardClient = &http.Client{Timeout: 30 * time.Second}

type ForwardResult struct {
	ServerName string        `json:"server_name"`
	Response   any           `json:"response"`
	Latency    time.Duration `json:"latency_ms"`
}

func (g *Gateway) ForwardToolCall(ctx context.Context, req common.MCPRequest) (*ForwardResult, error) {
	toolName, _ := req.Params["name"].(string)
	if toolName == "" {
		return nil, fmt.Errorf("missing tool name")
	}

	server, err := g.registry.FindServerByTool(toolName)
	if err != nil {
		return nil, fmt.Errorf("route to tool %q: %w", toolName, err)
	}

	start := time.Now()
	response, err := forwardToServer(ctx, server, req)
	latency := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("forward to %s: %w", server.Name, err)
	}

	return &ForwardResult{ServerName: server.Name, Response: response, Latency: latency}, nil
}

func forwardToServer(ctx context.Context, server ConnectedServer, req common.MCPRequest) (any, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := server.URL + "/mcp/message"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := forwardClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var response any
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return response, nil
}
