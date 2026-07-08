package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/varunbanda/mcp-gateway/internal/auth"
	"github.com/varunbanda/mcp-gateway/internal/ai"
	"github.com/varunbanda/mcp-gateway/internal/common"
	"github.com/varunbanda/mcp-gateway/internal/mcp"
	"github.com/varunbanda/mcp-gateway/internal/storage"
)

type ChatHandler struct {
	Gateway   *mcp.Gateway
	Logger    *common.Logger
	Brain     *ai.Manager
	Auth      *auth.Auth
	ChatStore *storage.ChatStore
	MemHistory map[string][]common.ChatMessage
	mu        sync.RWMutex
}

func (h *ChatHandler) HandleChat(w http.ResponseWriter, r *http.Request) {
	if h.Brain == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI not configured (set GROQ_API_KEY)"})
		return
	}

	var req struct {
		Message   string   `json:"message"`
		SessionID string   `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" || req.SessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message and session_id required"})
		return
	}
	if len(req.Message) > 10000 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message too long"})
		return
	}

	start := time.Now()
	username, _ := auth.UserFromContext(r.Context())

	ctx := r.Context()
	var history []common.ChatMessage
	if h.ChatStore != nil {
		if _, err := h.ChatStore.GetSession(ctx, req.SessionID, username); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "session not found"})
			return
		}
		h.ChatStore.AddMessage(ctx, req.SessionID, "user", req.Message, nil)

		if sess, _ := h.ChatStore.GetSession(ctx, req.SessionID, username); sess != nil && sess.Title == "New Chat" {
			title := req.Message
			if len(title) > 50 {
				title = title[:50] + "..."
			}
			h.ChatStore.UpdateSessionTitle(ctx, req.SessionID, username, title)
		}

		msgs, _ := h.ChatStore.GetRecentMessages(ctx, req.SessionID, 10)
		history = msgs
	} else {
		h.mu.RLock()
		msgs := h.MemHistory[req.SessionID]
		h.mu.RUnlock()

		for _, m := range msgs {
			history = append(history, m)
		}

		h.mu.Lock()
		h.MemHistory[req.SessionID] = append(msgs, common.ChatMessage{Role: "user", Content: req.Message, CreatedAt: time.Now()})
		h.mu.Unlock()
	}

	var llmHistory []map[string]string
	for _, m := range history {
		llmHistory = append(llmHistory, map[string]string{"role": m.Role, "content": m.Content})
	}

	callToolFn := func(ctx context.Context, toolName string, args map[string]any) (string, error) {
		fwdReq := common.MCPRequest{
			JSONRPC: "2.0", ID: 1, Method: "tools/call",
			Params: map[string]any{"name": toolName, "arguments": args, "_user": username},
		}
		result, err := h.Gateway.ForwardToolCall(ctx, fwdReq)
		if err != nil {
			return "", err
		}
		return extractToolText(result.Response), nil
	}

	answer, steps, err := h.Brain.RunAgent(ctx, req.Message, llmHistory, callToolFn)
	chatLatency := time.Since(start)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "AI error: " + err.Error()})
		return
	}

	var toolsUsed []string
	for _, step := range steps {
		toolsUsed = append(toolsUsed, step.ToolName)
		h.Logger.Log("agent", step.ToolName, "", username, "success", "", 0)
		if h.Auth != nil {
			h.Auth.LogRequest(username, "agent", step.ToolName, "", "success", "", 0)
		}
	}

	h.Brain.Memory().Save(ai.MemoryEntry{
		Query: req.Message, Answer: answer, ToolsUsed: toolsUsed,
	})

	if h.ChatStore != nil {
		h.ChatStore.AddMessage(ctx, req.SessionID, "ai", answer, map[string]any{
			"tools": toolsUsed, "latency": chatLatency.Milliseconds(), "steps": steps,
		})
	} else {
		h.mu.Lock()
		h.MemHistory[req.SessionID] = append(h.MemHistory[req.SessionID], common.ChatMessage{Role: "assistant", Content: answer, CreatedAt: time.Now()})
		if len(h.MemHistory[req.SessionID]) > 20 {
			h.MemHistory[req.SessionID] = h.MemHistory[req.SessionID][len(h.MemHistory[req.SessionID])-20:]
		}
		h.mu.Unlock()
	}

	h.Logger.Log("chat", "", "", username, "success", "", chatLatency)
	if h.Auth != nil {
		h.Auth.LogRequest(username, "chat", "", "", "success", "", chatLatency)
	}

	writeJSON(w, http.StatusOK, common.AIResponse{
		Answer: answer, Steps: steps, ToolsUsed: toolsUsed,
		Latency: chatLatency.Milliseconds(),
	})
}

func (h *ChatHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	if h.ChatStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"sessions": []any{}, "count": 0})
		return
	}
	username, _ := auth.UserFromContext(r.Context())
	sessions, err := h.ChatStore.ListSessions(r.Context(), username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions, "count": len(sessions)})
}

func (h *ChatHandler) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	if h.ChatStore == nil {
		writeJSON(w, http.StatusCreated, map[string]any{"id": fmt.Sprintf("local-%d", time.Now().UnixNano()), "title": "New Chat"})
		return
	}
	username, _ := auth.UserFromContext(r.Context())
	var req struct{ Title string `json:"title"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Title == "" {
		req.Title = "New Chat"
	}
	session, err := h.ChatStore.CreateSession(r.Context(), username, req.Title)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (h *ChatHandler) HandleDeleteSession(w http.ResponseWriter, r *http.Request) {
	username, _ := auth.UserFromContext(r.Context())
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing session id"})
		return
	}
	if h.ChatStore != nil {
		if err := h.ChatStore.DeleteSession(r.Context(), id, username); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *ChatHandler) HandleGetMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing session id"})
		return
	}
	if h.ChatStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"messages": []any{}, "count": 0})
		return
	}
	username, _ := auth.UserFromContext(r.Context())
	if _, err := h.ChatStore.GetSession(r.Context(), id, username); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	messages, err := h.ChatStore.GetMessages(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": messages, "count": len(messages)})
}

func extractToolText(response any) string {
	respMap, ok := response.(map[string]any)
	if !ok {
		return ""
	}
	result, ok := respMap["result"].(map[string]any)
	if !ok {
		return ""
	}
	content, ok := result["content"].([]any)
	if !ok {
		return ""
	}
	var text string
	for _, c := range content {
		if cm, ok := c.(map[string]any); ok {
			if t, ok := cm["text"].(string); ok {
				text += t
			}
		}
	}
	return text
}
