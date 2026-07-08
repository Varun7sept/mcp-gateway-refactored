package search

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/varunbanda/mcp-gateway/servers/internal/mcpcommon"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

var tools = []map[string]any{
	{"name": "web_search", "description": "Search the internet for real-time info", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "description": "Search query"}},
		"required": []string{"query"},
	}},
	{"name": "wikipedia_summary", "description": "Get Wikipedia summary for a topic", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"topic": map[string]any{"type": "string", "description": "Topic name"}},
		"required": []string{"topic"},
	}},
}

func Start(port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mcp/message", handleMCP)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	log.Printf("Search MCP Server on :%s", port)
	return http.ListenAndServe(":"+port, mux)
}

func handleMCP(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	var req mcpcommon.MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mcpcommon.SendError(enc, nil, -32700, "Parse error")
		return
	}
	switch req.Method {
	case "initialize":
		mcpcommon.SendResult(enc, req.ID, map[string]any{
			"protocolVersion": "2024-11-05", "capabilities": map[string]any{"tools": map[string]any{}},
			"serverInfo": map[string]any{"name": "search-server", "version": "1.0.0"},
		})
	case "tools/list":
		mcpcommon.SendResult(enc, req.ID, map[string]any{"tools": tools})
	case "tools/call":
		handleTool(enc, req)
	default:
		mcpcommon.SendError(enc, req.ID, -32601, "Method not found")
	}
}

func handleTool(enc *json.Encoder, req mcpcommon.MCPRequest) {
	name, _ := req.Params["name"].(string)
	args, _ := req.Params["arguments"].(map[string]any)
	switch name {
	case "web_search":
		q, _ := args["query"].(string)
		if q == "" { mcpcommon.SendToolResult(enc, req.ID, "Error: query required", true); return }
		r, err := duckDuckGo(q)
		if err != nil { mcpcommon.SendToolResult(enc, req.ID, "Error: "+err.Error(), true); return }
		mcpcommon.SendToolResult(enc, req.ID, r, false)
	case "wikipedia_summary":
		t, _ := args["topic"].(string)
		if t == "" { mcpcommon.SendToolResult(enc, req.ID, "Error: topic required", true); return }
		r, err := wikipedia(t)
		if err != nil { mcpcommon.SendToolResult(enc, req.ID, "Error: "+err.Error(), true); return }
		mcpcommon.SendToolResult(enc, req.ID, r, false)
	default:
		mcpcommon.SendToolResult(enc, req.ID, "Unknown tool", true)
	}
}

func duckDuckGo(query string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1", url.QueryEscape(query)), nil)
	if err != nil { return "", err }
	req.Header.Set("User-Agent", "MCP-Gateway/1.0")
	resp, err := httpClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var data struct {
		Abstract, AbstractSource, Answer, Heading string
		RelatedTopics []struct { Text, FirstURL string } `json:"RelatedTopics"`
	}
	json.Unmarshal(body, &data)

	var result strings.Builder
	if data.Answer != "" { result.WriteString("Answer: " + data.Answer + "\n\n") }
	if data.Abstract != "" {
		result.WriteString("Summary: " + data.Abstract + "\n")
		if data.AbstractSource != "" { result.WriteString("Source: " + data.AbstractSource + "\n") }
	}
	if data.Abstract == "" && data.Answer == "" && len(data.RelatedTopics) > 0 {
		result.WriteString("Related results:\n")
		limit := 5; if len(data.RelatedTopics) < limit { limit = len(data.RelatedTopics) }
		for i := 0; i < limit; i++ { result.WriteString(fmt.Sprintf("  %d. %s\n", i+1, data.RelatedTopics[i].Text)) }
	}
	if result.Len() == 0 { return fmt.Sprintf("No results for %q", query), nil }
	return result.String(), nil
}

func wikipedia(topic string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://en.wikipedia.org/api/rest_v1/page/summary/%s", url.PathEscape(strings.ReplaceAll(topic, " ", "_"))), nil)
	if err != nil { return "", err }
	req.Header.Set("User-Agent", "MCP-Gateway/1.0")
	resp, err := httpClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode == 404 { return fmt.Sprintf("No Wikipedia article for %q", topic), nil }
	body, _ := io.ReadAll(resp.Body)
	var data struct { Title, Extract, Description string }
	json.Unmarshal(body, &data)
	result := fmt.Sprintf("Wikipedia: %s\n", data.Title)
	if data.Description != "" { result += fmt.Sprintf("Description: %s\n\n", data.Description) }
	if data.Extract != "" {
		if len(data.Extract) > 500 { data.Extract = data.Extract[:500] + "..." }
		result += data.Extract
	}
	return result, nil
}
