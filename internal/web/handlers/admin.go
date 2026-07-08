package handlers

import (
	"net/http"

	"github.com/varunbanda/mcp-gateway/internal/auth"
	"github.com/varunbanda/mcp-gateway/internal/common"
	"github.com/varunbanda/mcp-gateway/internal/mcp"
)

type AdminHandler struct {
	Gateway *mcp.Gateway
	Logger  *common.Logger
	Auth    *auth.Auth
}

func (h *AdminHandler) HandleListServers(w http.ResponseWriter, r *http.Request) {
	servers := h.Gateway.Registry().ListServers()
	writeJSON(w, http.StatusOK, map[string]any{"servers": servers, "count": len(servers)})
}

func (h *AdminHandler) HandleListTools(w http.ResponseWriter, r *http.Request) {
	tools := h.Gateway.Registry().ListTools()
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools, "count": len(tools)})
}

func (h *AdminHandler) HandleLogs(w http.ResponseWriter, r *http.Request) {
	username, _ := auth.UserFromContext(r.Context())
	if h.Auth != nil {
		logs := h.Auth.RecentLogs(50, username)
		writeJSON(w, http.StatusOK, map[string]any{"logs": logs, "count": len(logs)})
		return
	}
	logs := h.Logger.Recent(50, username)
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs, "count": len(logs)})
}

func (h *AdminHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	username, _ := auth.UserFromContext(r.Context())
	if h.Auth != nil {
		stats := h.Auth.GetRequestStats(username)
		writeJSON(w, http.StatusOK, stats)
		return
	}
	stats := h.Logger.GetStats(username)
	writeJSON(w, http.StatusOK, stats)
}
