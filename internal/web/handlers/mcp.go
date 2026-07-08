package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/varunbanda/mcp-gateway/internal/auth"
	"github.com/varunbanda/mcp-gateway/internal/common"
	"github.com/varunbanda/mcp-gateway/internal/mcp"
)

type MCPHandler struct {
	Gateway *mcp.Gateway
	Logger  *common.Logger
	Auth    *auth.Auth
}

func (h *MCPHandler) HandleMCPMessage(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)

	var req common.MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	username, _ := auth.UserFromContext(r.Context())

	if req.Method == "tools/list" {
		tools := h.Gateway.Registry().ListTools()
		h.Logger.Log("tools/list", "", "", username, "success", "", time.Since(start))
		if h.Auth != nil {
			h.Auth.LogRequest(username, "tools/list", "", "", "success", "", time.Since(start))
		}
		writeJSON(w, http.StatusOK, common.MCPResponse{
			JSONRPC: "2.0", ID: req.ID,
			Result: map[string]any{"tools": tools},
		})
		return
	}

	if req.Method == "tools/call" {
		if username != "" {
			req.Params["_user"] = username
		}
		toolName, _ := req.Params["name"].(string)

		result, err := h.Gateway.ForwardToolCall(r.Context(), req)
		latency := time.Since(start)

		if err != nil {
			h.Logger.Log("tools/call", toolName, "", username, "error", err.Error(), latency)
			if h.Auth != nil {
				h.Auth.LogRequest(username, "tools/call", toolName, "", "error", err.Error(), latency)
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}

		h.Logger.Log("tools/call", toolName, result.ServerName, username, "success", "", latency)
		if h.Auth != nil {
			h.Auth.LogRequest(username, "tools/call", toolName, result.ServerName, "success", "", latency)
		}
		writeJSON(w, http.StatusOK, result.Response)
		return
	}

	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported method: " + req.Method})
}
