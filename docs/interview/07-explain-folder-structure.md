# Explain Folder Structure

```
cmd/server/main.go         ← Entry point
internal/
  ai/                      ← AI orchestration
  auth/                    ← JWT authentication
  common/                  ← Shared types, config, logger
  mcp/                     ← MCP gateway + registry
  storage/                 ← MongoDB + SQLite persistence
  web/                     ← HTTP server + middleware
  web/handlers/            ← Per-feature HTTP handlers
servers/                   ← Independent MCP backend servers
  weather/                 ← Weather data server
  github/                  ← GitHub API server
  crypto/                  ← Cryptocurrency prices server
  search/                  ← Web search + Wikipedia server
  news/                    ← News headlines server
  url/                     ← URL shortener + QR code server
  notes/                   ← Personal notes server
  internal/mcpcommon/      ← Shared MCP types
docs/                      ← Documentation
  architecture/            ← Architecture Decision Records
  diagrams/                ← Mermaid diagrams
  interview/              ← Interview preparation notes
```

## Why this structure?

**`cmd/` pattern** — Go standard layout for executable entry points. Allows adding `cmd/docs-server/` etc. without cluttering root.

**`internal/`** — Prevents external packages from importing our internal code. Enforced by `go vet`.

**`internal/common/`** — Centralises shared types (`ChatMessage`, `ChatSession`, `AIResponse`, `MCPRequest`) so every package doesn't define its own versions.

**`internal/web/handlers/`** — Separated from routes/middleware so each handler is a focused file (auth.go, chat.go, admin.go, etc.) instead of one monolithic server.go.

**`servers/`** — Each MCP server is a standalone package with its own `Start()` function. They could be extracted into separate repositories.

**`docs/architecture/`** — ADRs capture why decisions were made, not just what was built.
