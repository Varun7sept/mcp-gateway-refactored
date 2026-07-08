package common

import "time"

type ChatMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	Meta      map[string]any `json:"meta,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type ChatSession struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AIRequest struct {
	Message   string   `json:"message"`
	SessionID string   `json:"session_id"`
}

type AIResponse struct {
	Answer    string            `json:"answer"`
	Steps     []ToolStep        `json:"steps"`
	ToolsUsed []string          `json:"tools_used"`
	Latency   int64             `json:"latency_ms"`
}

type ToolStep struct {
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments"`
	Result    string         `json:"result"`
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type MCPRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}
