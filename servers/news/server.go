package news

import (
	"encoding/json"
	"encoding/xml"
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
	{"name": "get_top_news", "description": "Get top news headlines by topic", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"topic": map[string]any{"type": "string", "description": "general, technology, business, sports, science, health"}},
	}},
	{"name": "search_news", "description": "Search news articles by keyword", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "description": "Search keyword"}},
		"required": []string{"query"},
	}},
}

var feeds = map[string]string{
	"general":    "https://news.google.com/rss?hl=en-US&gl=US&ceid=US:en",
	"technology": "https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGRqTVhZU0FtVnVHZ0pWVXlnQVAB",
	"business":   "https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGx6TVdZU0FtVnVHZ0pWVXlnQVAB",
	"sports":     "https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp1ZEdvU0FtVnVHZ0pWVXlnQVAB",
	"science":    "https://news.google.com/rss/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp0Y1RjU0FtVnVHZ0pWVXlnQVAB",
	"health":     "https://news.google.com/rss/topics/CAAqIQgKIhtDQkFTRGdvSUwyMHZNR3QwTlRFU0FtVnVLQUFQAQ",
}

func Start(port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mcp/message", handleMCP)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	log.Printf("News MCP Server on :%s", port)
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
			"serverInfo": map[string]any{"name": "news-server", "version": "1.0.0"},
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
	case "get_top_news":
		topic, _ := args["topic"].(string)
		if topic == "" { topic = "general" }
		r, err := getNews(topic)
		if err != nil { mcpcommon.SendToolResult(enc, req.ID, "Error: "+err.Error(), true); return }
		mcpcommon.SendToolResult(enc, req.ID, r, false)
	case "search_news":
		q, _ := args["query"].(string)
		if q == "" { mcpcommon.SendToolResult(enc, req.ID, "Error: query required", true); return }
		r, err := search(q)
		if err != nil { mcpcommon.SendToolResult(enc, req.ID, "Error: "+err.Error(), true); return }
		mcpcommon.SendToolResult(enc, req.ID, r, false)
	default:
		mcpcommon.SendToolResult(enc, req.ID, "Unknown tool", true)
	}
}

func getNews(topic string) (string, error) {
	feed, ok := feeds[strings.ToLower(topic)]
	if !ok { feed = feeds["general"] }
	items, err := fetchRSS(feed)
	if err != nil { return "", err }
	limit := 8
	if len(items) < limit { limit = len(items) }
	var lines []string
	for i := 0; i < limit; i++ { lines = append(lines, fmt.Sprintf("  %d. %s", i+1, items[i].Title)) }
	return fmt.Sprintf("Top %s News:\n\n%s", strings.Title(topic), strings.Join(lines, "\n")), nil
}

func search(query string) (string, error) {
	items, err := fetchRSS(fmt.Sprintf("https://news.google.com/rss/search?q=%s&hl=en-US&gl=US&ceid=US:en", url.QueryEscape(query)))
	if err != nil { return "", err }
	if len(items) == 0 { return fmt.Sprintf("No news for %q", query), nil }
	limit := 8
	if len(items) < limit { limit = len(items) }
	var lines []string
	for i := 0; i < limit; i++ { lines = append(lines, fmt.Sprintf("  %d. %s", i+1, items[i].Title)) }
	return fmt.Sprintf("News about %q:\n\n%s", query, strings.Join(lines, "\n")), nil
}

type RSS struct { Channel struct { Items []struct { Title, Link, PubDate, Source string } `xml:"item"` } `xml:"channel"` }

func fetchRSS(feedURL string) ([]struct { Title, Link, PubDate, Source string }, error) {
	resp, err := httpClient.Get(feedURL)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var rss RSS
	xml.Unmarshal(body, &rss)
	return rss.Channel.Items, nil
}
