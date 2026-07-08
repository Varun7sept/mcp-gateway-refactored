package url

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
	{"name": "shorten_url", "description": "Shorten a URL using is.gd", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"url": map[string]any{"type": "string", "description": "Full URL"}},
		"required": []string{"url"},
	}},
	{"name": "generate_qr", "description": "Generate QR code image URL", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string", "description": "Text or URL to encode"}},
		"required": []string{"text"},
	}},
	{"name": "expand_url", "description": "Expand a shortened URL to full destination", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"url": map[string]any{"type": "string", "description": "Shortened URL"}},
		"required": []string{"url"},
	}},
}

func Start(port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mcp/message", handleMCP)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	log.Printf("URL Tools MCP Server on :%s", port)
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
			"serverInfo": map[string]any{"name": "url-tools-server", "version": "1.0.0"},
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
	case "shorten_url":
		u, _ := args["url"].(string)
		if u == "" { mcpcommon.SendToolResult(enc, req.ID, "Error: url required", true); return }
		resp, err := httpClient.Get(fmt.Sprintf("https://is.gd/create.php?format=simple&url=%s", url.QueryEscape(u)))
		if err != nil { mcpcommon.SendToolResult(enc, req.ID, "Error: "+err.Error(), true); return }
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		s := strings.TrimSpace(string(body))
		if strings.HasPrefix(s, "http") {
			mcpcommon.SendToolResult(enc, req.ID, fmt.Sprintf("Shortened: %s → %s", u, s), false)
		} else {
			mcpcommon.SendToolResult(enc, req.ID, "Error shortening URL", true)
		}
	case "generate_qr":
		text, _ := args["text"].(string)
		if text == "" { mcpcommon.SendToolResult(enc, req.ID, "Error: text required", true); return }
		mcpcommon.SendToolResult(enc, req.ID, fmt.Sprintf("QR Code:\n  Content: %s\n  Image URL: https://api.qrserver.com/v1/create-qr-code/?size=250x250&data=%s", text, url.QueryEscape(text)), false)
	case "expand_url":
		u, _ := args["url"].(string)
		if u == "" { mcpcommon.SendToolResult(enc, req.ID, "Error: url required", true); return }
		if parsed, err := url.Parse(u); err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			mcpcommon.SendToolResult(enc, req.ID, "Error: only http/https URLs supported", true); return
		}
		client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := client.Head(u)
		if err != nil { mcpcommon.SendToolResult(enc, req.ID, fmt.Sprintf("Error: %v", err), true); return }
		defer resp.Body.Close()
		if loc := resp.Header.Get("Location"); loc != "" {
			mcpcommon.SendToolResult(enc, req.ID, fmt.Sprintf("Short: %s\n  Full: %s", u, loc), false)
		} else {
			mcpcommon.SendToolResult(enc, req.ID, fmt.Sprintf("No redirect: %s", u), false)
		}
	default:
		mcpcommon.SendToolResult(enc, req.ID, "Unknown tool", true)
	}
}
