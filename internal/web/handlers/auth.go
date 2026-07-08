package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/varunbanda/mcp-gateway/internal/auth"
)

type AuthHandler struct {
	Auth    *auth.Auth
	Limiter interface{ Allow(string) bool }
}

func (h *AuthHandler) HandleSignup(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth not configured"})
		return
	}
	if !h.Limiter.Allow(clientIP(r)) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username, email, password required"})
		return
	}
	token, err := h.Auth.Signup(req.Username, req.Email, req.Password)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "username": req.Username})
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth not configured"})
		return
	}
	if !h.Limiter.Allow(clientIP(r)) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password required"})
		return
	}
	token, err := h.Auth.Login(req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "username": req.Username})
}
