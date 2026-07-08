package handlers

import (
	"embed"
	"net/http"
)

//go:embed dashboard.html
var dashboardFS embed.FS

//go:embed chatui.html
var chatUIFS embed.FS

var dashboardHTML string
var chatPageHTML string

func init() {
	data, _ := dashboardFS.ReadFile("dashboard.html")
	dashboardHTML = string(data)
	data, _ = chatUIFS.ReadFile("chatui.html")
	chatPageHTML = string(data)
}

type DashboardHandler struct{}

func (h *DashboardHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(dashboardHTML))
}

func (h *DashboardHandler) HandleChatPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(chatPageHTML))
}
