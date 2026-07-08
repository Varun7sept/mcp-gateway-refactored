package web

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/varunbanda/mcp-gateway/internal/ai"
	"github.com/varunbanda/mcp-gateway/internal/auth"
	"github.com/varunbanda/mcp-gateway/internal/common"
	"github.com/varunbanda/mcp-gateway/internal/mcp"
	"github.com/varunbanda/mcp-gateway/internal/storage"
	"github.com/varunbanda/mcp-gateway/internal/web/handlers"
)

type Server struct {
	gateway         *mcp.Gateway
	logger          *common.Logger
	brain           *ai.Manager
	auth            *auth.Auth
	chatStore       *storage.ChatStore
	authLimiter     *rateLimiter
	memHistory      map[string][]common.ChatMessage
	port            int
}

func New(gw *mcp.Gateway, logger *common.Logger, brain *ai.Manager, authenticator *auth.Auth, chatStore *storage.ChatStore, port int) *Server {
	return &Server{
		gateway: gw, logger: logger, brain: brain, auth: authenticator,
		chatStore: chatStore, port: port,
		authLimiter: newRateLimiter(time.Minute, 10),
		memHistory:  make(map[string][]common.ChatMessage),
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	dh := &handlers.DashboardHandler{}
	mux.HandleFunc("GET /", dh.HandleDashboard)
	mux.HandleFunc("GET /chat", dh.HandleChatPage)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "healthy", "servers": len(s.gateway.Registry().ListServers()),
			"tools": len(s.gateway.Registry().ListTools()),
		})
	})

	ah := &handlers.AuthHandler{Auth: s.auth, Limiter: s.authLimiter}
	mux.HandleFunc("POST /api/auth/signup", ah.HandleSignup)
	mux.HandleFunc("POST /api/auth/login", ah.HandleLogin)

	adminH := &handlers.AdminHandler{Gateway: s.gateway, Logger: s.logger, Auth: s.auth}
	mux.HandleFunc("GET /api/servers", adminH.HandleListServers)
	mux.HandleFunc("GET /api/tools", adminH.HandleListTools)
	mux.HandleFunc("GET /api/logs", adminH.HandleLogs)
	mux.HandleFunc("GET /api/stats", adminH.HandleStats)

	mcpH := &handlers.MCPHandler{Gateway: s.gateway, Logger: s.logger, Auth: s.auth}
	mux.HandleFunc("POST /mcp/message", mcpH.HandleMCPMessage)

	ch := &handlers.ChatHandler{
		Gateway: s.gateway, Logger: s.logger, Brain: s.brain,
		Auth: s.auth, ChatStore: s.chatStore, MemHistory: s.memHistory,
	}
	mux.HandleFunc("POST /api/chat", ch.HandleChat)
	mux.HandleFunc("GET /api/chat/sessions", ch.HandleListSessions)
	mux.HandleFunc("POST /api/chat/sessions", ch.HandleCreateSession)
	mux.HandleFunc("DELETE /api/chat/sessions/{id}", ch.HandleDeleteSession)
	mux.HandleFunc("GET /api/chat/sessions/{id}/messages", ch.HandleGetMessages)

	uh := &handlers.UploadHandler{Gateway: s.gateway}
	mux.HandleFunc("POST /api/upload", uh.HandleFileUpload)

	handler := http.Handler(mux)
	handler = loggingMiddleware(handler)
	handler = corsMiddleware(handler)
	if s.auth != nil {
		handler = authMiddleware(s.auth)(handler)
	}

	port := s.port
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	addr := ":" + strconv.Itoa(port)
	return http.ListenAndServe(addr, handler)
}
