package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/varunbanda/mcp-gateway/internal/ai"
	"github.com/varunbanda/mcp-gateway/internal/auth"
	"github.com/varunbanda/mcp-gateway/internal/common"
	"github.com/varunbanda/mcp-gateway/internal/mcp"
	"github.com/varunbanda/mcp-gateway/internal/storage"
	"github.com/varunbanda/mcp-gateway/internal/web"
	"github.com/varunbanda/mcp-gateway/servers/notes"
	"github.com/varunbanda/mcp-gateway/servers/weather"
	"github.com/varunbanda/mcp-gateway/servers/github"
	"github.com/varunbanda/mcp-gateway/servers/crypto"
	"github.com/varunbanda/mcp-gateway/servers/news"
	"github.com/varunbanda/mcp-gateway/servers/search"
	"github.com/varunbanda/mcp-gateway/servers/url"
)

func main() {
	log.Println("Starting MCP Gateway...")

	// 1. Load Config
	cfg, err := common.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Config: %v", err)
	}
	log.Printf("Loaded config: %d servers", len(cfg.Servers))

	// 2. Initialize Logger
	reqLogger := common.NewLogger(1000)

	// 3. Connect Database
	var chatStore *storage.ChatStore
	var authenticator *auth.Auth
	if cfg.MongoDB.URI != "" {
		var authErr error
		authenticator, authErr = auth.New(cfg.MongoDB.URI, cfg.MongoDB.Database)
		if authErr != nil {
			log.Printf("WARNING: Auth unavailable: %v (proceeding without)", authErr)
		} else {
			chatStore = storage.NewChatStore(authenticator.DB())
			log.Println("MongoDB connected — auth enabled")
		}
	} else {
		log.Println("MongoDB not configured — auth disabled")
	}

	// 4. Initialize Gateway
	gw := mcp.New(cfg)
	log.Printf("Gateway initialized with %d servers", len(gw.Registry().ListServers()))

	// 5. Register & Start MCP Servers
	startMCP := func(name string, fn func() error) {
		go func() {
			log.Printf("Starting %s server...", name)
			if err := fn(); err != nil {
				log.Printf("%s server exited: %v", name, err)
			}
		}()
	}

	if s, err := notes.New("./notes.db"); err == nil {
		startMCP("notes", func() error { return s.Start(":3002") })
		defer s.Close()
	}
	startMCP("weather", func() error { return weather.Start("3001") })
	startMCP("github", func() error { return github.Start("3003") })
	startMCP("crypto", func() error { return crypto.Start("3004") })
	startMCP("news", func() error { return news.Start("3005") })
	startMCP("url-tools", func() error { return url.Start("3006") })
	startMCP("search", func() error { return search.Start("3007") })

	// Python docs server
	startMCP("documents", func() error {
		pythonCmd := "python3"
		if _, err := exec.LookPath("python3"); err != nil {
			pythonCmd = "python"
		}
		cmd := exec.Command(pythonCmd, "examples/docs-server/server.py")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})

	// 6. Start Health Checker
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	gw.StartHealthChecker(ctx, 10*time.Second)

	// 7. Initialize AI Manager
	var brain *ai.Manager
	groqKey := os.Getenv("GROQ_API_KEY")
	if groqKey != "" {
		brain = ai.New(groqKey)
		log.Println("AI Chat enabled (Groq API)")
	} else {
		log.Println("AI Chat disabled (set GROQ_API_KEY)")
	}

	// 8. Start HTTP Server
	httpServer := web.New(gw, reqLogger, brain, authenticator, chatStore, cfg.Gateway.Port)
	if err := httpServer.Start(); err != nil {
		log.Fatalf("Server: %v", err)
	}
}
