# MCP Gateway

A clean, refactored MCP (Model Context Protocol) Gateway that routes AI chat requests to specialized MCP backend servers with tool-calling capabilities.

## Architecture

```
Browser → HTTP Server → Chat Handler → AI Manager → Gateway → MCP Server
                                         ↓
                                    Memory Store
```

### Directory Layout

```
cmd/server/main.go       # Entry point: wires everything together
internal/
  ai/                    # AI manager: single entry point for LLM calls
  auth/                  # JWT authentication (signup, login, validation)
  common/                # Shared types, config, logger
  mcp/                   # MCP gateway (registry, forwarder, health checks)
  storage/               # MongoDB chat store, SQLite notes store
  web/                   # HTTP server, middleware, routes, handlers
servers/                 # Independent MCP backend servers
  weather/               # Weather data
  github/                # GitHub API
  crypto/                # Cryptocurrency prices
  search/                # Web search
  news/                  # News headlines
  url/                   # URL content fetching
  notes/                 # Personal notes (SQLite-backed)
  internal/mcpcommon/    # Shared MCP types
```

## Quick Start

### Prerequisites

- Go 1.21+
- MongoDB (for auth and chat history)
- A Spoonacular API key (for recipe search server)
- An OpenAI-compatible API key (for AI chat)

### Configuration

Copy `config.yaml` to the working directory and edit:

```yaml
server:
  port: ":8080"
mongodb:
  uri: "mongodb://localhost:27017"
  database: "mcp-gateway"
llm:
  api_key: "your-openai-api-key"
  models: ["gpt-4o", "gpt-4"]
```

### Run

```bash
go run ./cmd/server/
```

The server starts at `http://localhost:8080`. Open the dashboard at `/`, or the chat UI at `/chat`.

## API Endpoints

| Method | Path            | Description           |
|--------|-----------------|-----------------------|
| POST   | /api/auth/signup| Register a new user   |
| POST   | /api/auth/login | Login, get JWT token  |
| POST   | /api/chat       | Send chat message     |
| POST   | /api/mcp        | Raw MCP JSON-RPC call |
| POST   | /api/upload     | Upload file to docs   |
| GET    | /api/admin/servers | List MCP servers   |
| GET    | /api/admin/tools   | List available tools|
| GET    | /api/admin/logs    | Request logs       |
| GET    | /api/admin/stats   | Usage statistics   |
| GET    | /health         | Health check          |
| GET    | /               | Dashboard UI          |
| GET    | /chat           | Chat UI               |

## Dependency Rules

Architecture enforces a strict one-way dependency chain:

```
web/handlers → web → ai → mcp → servers → external APIs
                    ↘             ↘
                     auth         storage
```

**Rules:**
- Lower layers **never** import higher layers
- `internal/ai/` cannot import `internal/web/`
- `internal/mcp/` cannot import `internal/ai/`
- `servers/` cannot import any `internal/` package
- Shared types live in `internal/common/` and may be imported by any layer

Violations are caught by `go vet` (import cycles) and enforced during
code review.

## Public API Guidelines

- Export only what other packages need
- Keep helper functions unexported
- Accept `context.Context` as the first parameter of every request-scoped function
- Never create background contexts inside business logic

## Environment Variables

- `PORT` - override server port
- `MONGO_URI` - override MongoDB URI
- `MONGO_DATABASE` - override database name
- `JWT_SECRET` - override JWT signing key
- `LLM_API_KEY` - override LLM API key
- `ALLOWED_ORIGINS` - CORS origins (comma-separated)
