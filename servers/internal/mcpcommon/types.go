// Package mcpcommon provides shared JSON-RPC 2.0 helpers for MCP server implementations.
package mcpcommon

import "encoding/json"

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

func SendResult(w *json.Encoder, id any, result any) {
	w.Encode(MCPResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func SendError(w *json.Encoder, id any, code int, msg string) {
	w.Encode(MCPResponse{JSONRPC: "2.0", ID: id, Error: map[string]any{"code": code, "message": msg}})
}

func SendToolResult(w *json.Encoder, id any, text string, isError bool) {
	SendResult(w, id, map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isError,
	})
}
