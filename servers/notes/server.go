package notes

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/varunbanda/mcp-gateway/servers/internal/mcpcommon"
)

var tools = []map[string]any{
	{"name": "add_note", "description": "Save a note to the database", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{
			"title":   map[string]any{"type": "string", "description": "Note title"},
			"content": map[string]any{"type": "string", "description": "Note content"},
			"tags":    map[string]any{"type": "string", "description": "Optional tags"},
		},
		"required": []string{"title", "content"},
	}},
	{"name": "list_notes", "description": "List saved notes", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"limit": map[string]any{"type": "number", "description": "Max notes (1-100)"}},
	}},
	{"name": "search_notes", "description": "Search notes by keyword", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "description": "Search keyword"}},
		"required": []string{"query"},
	}},
}

type Server struct {
	db     *sql.DB
	server *http.Server
}

func New(dbPath string) (*Server, error) {
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT DEFAULT '',
			username TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}
	log.Println("Notes database initialized")
	return &Server{db: database}, nil
}

func (s *Server) Start(port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mcp/message", s.handleMCP)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		var count int
		s.db.QueryRow("SELECT COUNT(*) FROM notes").Scan(&count)
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "notes_count": count})
	})
	log.Printf("Notes MCP Server on :%s", port)
	return http.ListenAndServe(":"+port, mux)
}

func (s *Server) Close() error { return s.db.Close() }

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
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
			"serverInfo": map[string]any{"name": "notes-server", "version": "1.0.0"},
		})
	case "tools/list":
		mcpcommon.SendResult(enc, req.ID, map[string]any{"tools": tools})
	case "tools/call":
		s.handleTool(enc, req)
	default:
		mcpcommon.SendError(enc, req.ID, -32601, "Method not found")
	}
}

func (s *Server) handleTool(enc *json.Encoder, req mcpcommon.MCPRequest) {
	name, _ := req.Params["name"].(string)
	args, _ := req.Params["arguments"].(map[string]any)
	username, _ := req.Params["_user"].(string)

	switch name {
	case "add_note":
		title, _ := args["title"].(string)
		content, _ := args["content"].(string)
		tags, _ := args["tags"].(string)
		if title == "" || content == "" {
			mcpcommon.SendToolResult(enc, req.ID, "Error: title and content required", true)
			return
		}
		result, err := s.db.Exec("INSERT INTO notes (title, content, tags, username) VALUES (?, ?, ?, ?)", title, content, tags, username)
		if err != nil {
			mcpcommon.SendToolResult(enc, req.ID, "Database error: "+err.Error(), true)
			return
		}
		id, _ := result.LastInsertId()
		mcpcommon.SendToolResult(enc, req.ID, fmt.Sprintf("Note saved! ID: %d, Title: %q", id, title), false)

	case "list_notes":
		limit := 20
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limit = int(l)
			if limit > 100 { limit = 100 }
		}
		var rows *sql.Rows
		var err error
		if username != "" {
			rows, err = s.db.Query("SELECT id, title, content, tags, created_at FROM notes WHERE username=? ORDER BY created_at DESC LIMIT ?", username, limit)
		} else {
			rows, err = s.db.Query("SELECT id, title, content, tags, created_at FROM notes ORDER BY created_at DESC LIMIT ?", limit)
		}
		if err != nil {
			mcpcommon.SendToolResult(enc, req.ID, "Database error: "+err.Error(), true)
			return
		}
		defer rows.Close()
		var lines []string
		for rows.Next() {
			var id int; var title, content, tags, createdAt string
			rows.Scan(&id, &title, &content, &tags, &createdAt)
			line := fmt.Sprintf("#%d [%s] %s", id, createdAt, title)
			if tags != "" { line += fmt.Sprintf(" (tags: %s)", tags) }
			line += fmt.Sprintf("\n    %s", content)
			lines = append(lines, line)
		}
		if len(lines) == 0 {
			mcpcommon.SendToolResult(enc, req.ID, "No notes found. Use add_note to create one!", false)
		} else {
			mcpcommon.SendToolResult(enc, req.ID, fmt.Sprintf("Found %d notes:\n\n%s", len(lines), strings.Join(lines, "\n\n")), false)
		}

	case "search_notes":
		query, _ := args["query"].(string)
		if query == "" {
			mcpcommon.SendToolResult(enc, req.ID, "Error: query required", true)
			return
		}
		searchTerm := "%" + query + "%"
		var rows *sql.Rows
		var err error
		if username != "" {
			rows, err = s.db.Query("SELECT id, title, content, tags FROM notes WHERE username=? AND (title LIKE ? OR content LIKE ? OR tags LIKE ?)", username, searchTerm, searchTerm, searchTerm)
		} else {
			rows, err = s.db.Query("SELECT id, title, content, tags FROM notes WHERE title LIKE ? OR content LIKE ? OR tags LIKE ?", searchTerm, searchTerm, searchTerm)
		}
		if err != nil {
			mcpcommon.SendToolResult(enc, req.ID, "Database error: "+err.Error(), true)
			return
		}
		defer rows.Close()
		var lines []string
		for rows.Next() {
			var id int; var title, content, tags string
			rows.Scan(&id, &title, &content, &tags)
			lines = append(lines, fmt.Sprintf("#%d: %s — %s", id, title, content))
		}
		if len(lines) == 0 {
			mcpcommon.SendToolResult(enc, req.ID, fmt.Sprintf("No notes matching %q", query), false)
		} else {
			mcpcommon.SendToolResult(enc, req.ID, fmt.Sprintf("Found %d notes matching %q:\n%s", len(lines), query, strings.Join(lines, "\n")), false)
		}

	default:
		mcpcommon.SendToolResult(enc, req.ID, "Unknown tool: "+name, true)
	}
}
