package github

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/varunbanda/mcp-gateway/servers/internal/mcpcommon"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}
var token = os.Getenv("GITHUB_TOKEN")

var tools = []map[string]any{
	{"name": "get_user", "description": "Get GitHub user profile", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"username": map[string]any{"type": "string", "description": "GitHub username"}},
		"required": []string{"username"},
	}},
	{"name": "list_repos", "description": "List public repos for a GitHub user by stars", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"username": map[string]any{"type": "string", "description": "GitHub username"}},
		"required": []string{"username"},
	}},
	{"name": "get_repo", "description": "Get GitHub repo details", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{
			"owner": map[string]any{"type": "string", "description": "Repo owner"},
			"repo":  map[string]any{"type": "string", "description": "Repo name"},
		},
		"required": []string{"owner", "repo"},
	}},
}

func Start(port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mcp/message", handleMCP)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	log.Printf("GitHub MCP Server on :%s", port)
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
			"serverInfo": map[string]any{"name": "github-server", "version": "1.0.0"},
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
	case "get_user":
		u, _ := args["username"].(string)
		if u == "" { mcpcommon.SendToolResult(enc, req.ID, "Error: username required", true); return }
		body, err := githubAPI("/users/" + url.PathEscape(u))
		if err != nil { mcpcommon.SendToolResult(enc, req.ID, "Error: "+err.Error(), true); return }
		var user struct { Login, Name, Bio, Location, CreatedAt string; Followers, Following, PublicRepos int }
		json.Unmarshal(body, &user)
		mcpcommon.SendToolResult(enc, req.ID, fmt.Sprintf("GitHub User: %s (@%s)\n  Bio: %s\n  Location: %s\n  Repos: %d\n  Followers: %d | Following: %d", user.Name, user.Login, user.Bio, user.Location, user.PublicRepos, user.Followers, user.Following), false)
	case "list_repos":
		u, _ := args["username"].(string)
		if u == "" { mcpcommon.SendToolResult(enc, req.ID, "Error: username required", true); return }
		body, err := githubAPI(fmt.Sprintf("/users/%s/repos?sort=stars&per_page=10", url.PathEscape(u)))
		if err != nil { mcpcommon.SendToolResult(enc, req.ID, "Error: "+err.Error(), true); return }
		var repos []struct { Name, Description, Language string; Stars int; Fork bool }
		json.Unmarshal(body, &repos)
		var lines []string
		for _, r := range repos {
			if !r.Fork {
				d := r.Description; if len(d) > 60 { d = d[:60] + "..." }
				lines = append(lines, fmt.Sprintf("  %s — %s [%s, %d stars]", r.Name, d, r.Language, r.Stars))
			}
		}
		if len(lines) == 0 { mcpcommon.SendToolResult(enc, req.ID, "No repos found", false) } else { mcpcommon.SendToolResult(enc, req.ID, fmt.Sprintf("Repos for %s:\n%s", u, strings.Join(lines, "\n")), false) }
	case "get_repo":
		owner, _ := args["owner"].(string); repo, _ := args["repo"].(string)
		if owner == "" || repo == "" { mcpcommon.SendToolResult(enc, req.ID, "Error: owner and repo required", true); return }
		body, err := githubAPI(fmt.Sprintf("/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo)))
		if err != nil { mcpcommon.SendToolResult(enc, req.ID, "Error: "+err.Error(), true); return }
		var r struct { FullName, Description, Language string; Stars, Forks, OpenIssues int; License struct{ Name string } `json:"license"` }
		json.Unmarshal(body, &r)
		license := "None"; if r.License.Name != "" { license = r.License.Name }
		mcpcommon.SendToolResult(enc, req.ID, fmt.Sprintf("Repo: %s\n  Description: %s\n  Stars: %d | Forks: %d | Issues: %d\n  License: %s", r.FullName, r.Description, r.Stars, r.Forks, r.OpenIssues, license), false)
	default:
		mcpcommon.SendToolResult(enc, req.ID, "Unknown tool", true)
	}
}

func githubAPI(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", "https://api.github.com"+path, nil)
	if err != nil { return nil, err }
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "mcp-gateway")
	if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
	resp, err := httpClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 404 { return nil, fmt.Errorf("not found") }
	if resp.StatusCode != 200 { return nil, fmt.Errorf("GitHub API status %d", resp.StatusCode) }
	return body, nil
}
