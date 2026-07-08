# startup

``mermaid
sequenceDiagram
    participant Main as cmd/server/main.go
    participant Config as internal/common/config.go
    participant Logger as internal/common/logger.go
    participant Auth as internal/auth/auth.go
    participant Gateway as internal/mcp/gateway.go
    participant Registry as internal/mcp/registry.go
    participant Weather as servers/weather
    participant GitHub as servers/github
    participant Crypto as servers/crypto
    participant Search as servers/search
    participant News as servers/news
    participant URL as servers/url
    participant Notes as servers/notes
    participant Health as internal/mcp/health.go
    participant Manager as internal/ai/manager.go
    participant Server as internal/web/server.go

    Main->>Config: LoadConfig("config.yaml")
    Config-->>Main: *Config

    Main->>Logger: NewLogger(1000)
    Logger-->>Main: *Logger

    Main->>Auth: New(mongoURI, dbName)
    Auth-->>Main: *Auth

    Main->>Gateway: New(cfg)
    Gateway->>Registry: NewRegistry(cfg)
    Registry-->>Gateway: *Registry
    Gateway-->>Main: *Gateway

    Main->>Weather: Start("3001")
    Note over Weather: HTTP server on :3001
    Main->>GitHub: Start("3002")
    Main->>Crypto: Start("3003")
    Main->>Search: Start("3004")
    Main->>News: Start("3005")
    Main->>URL: Start("3006")
    Main->>Notes: Start("3007")

    Main->>Health: StartHealthChecker(registry, logger, 30s)
    Note over Health: Goroutine pings all servers every 30s

    Main->>Manager: New(groqKey)
    Manager-->>Main: *Manager

    Main->>Server: New(gateway, logger, manager, auth, chatStore, port)
    Server-->>Main: *Server

    Main->>Server: Start()
    Note over Server: HTTP listener on :8080

``
