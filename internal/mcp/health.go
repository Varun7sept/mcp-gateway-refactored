package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

var healthClient = &http.Client{Timeout: 5 * time.Second}

type mcpRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

func (g *Gateway) StartHealthChecker(ctx context.Context, interval time.Duration) {
	g.checkAll()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("Health checker stopped")
				return
			case <-ticker.C:
				g.checkAll()
			}
		}
	}()
	log.Printf("Health checker started (%s interval)", interval)
}

func (g *Gateway) checkAll() {
	servers := g.registry.ListServers()
	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(s ConnectedServer) {
			defer wg.Done()
			tools, latency, err := checkServer(s)
			if err != nil {
				log.Printf("  [%s] OFFLINE — %v", s.Name, err)
				g.registry.UpdateStatus(s.Name, StatusOffline, nil, 0)
			} else {
				log.Printf("  [%s] ONLINE — %d tools, %s", s.Name, len(tools), latency)
				g.registry.UpdateStatus(s.Name, StatusOnline, tools, latency)
			}
		}(s)
	}
	wg.Wait()
}

func checkServer(server ConnectedServer) ([]Tool, time.Duration, error) {
	start := time.Now()
	url := server.URL + "/mcp/message"

	initReq := mcpRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "mcp-gateway", "version": "1.0.0"},
		},
	}
	if _, err := sendMCP(healthClient, url, initReq); err != nil {
		return nil, 0, fmt.Errorf("initialize: %w", err)
	}

	toolsReq := mcpRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"}
	resp, err := sendMCP(healthClient, url, toolsReq)
	if err != nil {
		return nil, 0, fmt.Errorf("tools/list: %w", err)
	}

	latency := time.Since(start)
	tools, err := parseTools(resp, server.Name)
	if err != nil {
		return nil, latency, fmt.Errorf("parse tools: %w", err)
	}
	return tools, latency, nil
}

func sendMCP(client *http.Client, url string, req mcpRequest) (*mcpResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var mcpResp mcpResponse
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if mcpResp.Error != nil {
		return nil, fmt.Errorf("mcp error: %v", mcpResp.Error)
	}
	return &mcpResp, nil
}

func parseTools(resp *mcpResponse, serverName string) ([]Tool, error) {
	resultMap, ok := resp.Result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected result type")
	}
	toolsRaw, ok := resultMap["tools"].([]any)
	if !ok {
		return []Tool{}, nil
	}

	var tools []Tool
	seen := make(map[string]bool)
	for _, t := range toolsRaw {
		toolMap, ok := t.(map[string]any)
		if !ok {
			continue
		}
		name, _ := toolMap["name"].(string)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		desc, _ := toolMap["description"].(string)
		tools = append(tools, Tool{Name: name, Desc: desc, ServerName: serverName})
	}
	return tools, nil
}
