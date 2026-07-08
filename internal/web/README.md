# internal/web

## Why

HTTP transport layer: receives requests, applies middleware, routes to
feature-specific handlers, and returns responses.

## Who calls it

- `cmd/server/main.go` — creates `web.Server` and calls `Start()`
- `internal/web/handlers/` — registered in `routes.go`

## Responsibilities

- HTTP server lifecycle (start/stop)
- Middleware stack: logging, CORS, JWT auth, rate limiting
- Route registration for all API endpoints
- Delegate request handling to `internal/web/handlers/`

## Public entry points

```go
func New(gw *mcp.Gateway, logger *common.Logger, brain *ai.Manager, authenticator *auth.Auth, chatStore *storage.ChatStore, port int) *Server
func (s *Server) Start() error
```
