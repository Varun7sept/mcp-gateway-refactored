# MCP Gateway Layer — internal/mcp

## Why it exists

The MCP gateway decouples AI orchestration from backend server
communication.  It maintains a registry of tool → server mappings,
forwards tool calls as JSON-RPC requests, and monitors server health.

## Who calls it

- `internal/ai/manager.go` (via `Manager.RunAgent` → `Gateway.ForwardToolCall`)
- `internal/web/handlers/mcp.go` (raw MCP message endpoint)
- `internal/web/handlers/admin.go` (server/tool listing)
- `internal/web/handlers/upload.go` (document upload proxy)

## Public API

```go
func New(cfg *common.Config) *Gateway
func (g *Gateway) Registry() *Registry
func (g *Gateway) ForwardToolCall(req common.MCPRequest) (*ForwardResult, error)

func NewRegistry(cfg *common.Config) *Registry
func (r *Registry) FindServerByTool(toolName string) (ConnectedServer, error)
func (r *Registry) GetServer(name string) (ConnectedServer, bool)
func (r *Registry) ListServers() []ConnectedServer
func (r *Registry) ListTools() []string

func StartHealthChecker(registry *Registry, logger *common.Logger, interval time.Duration)
```

## File Layout

| File | Responsibility |
|------|---------------|
| gateway.go | `Gateway` struct — thin wrapper over registry |
| registry.go | Server + tool name → handler URL lookup |
| forwarder.go | JSON-RPC forwarding via HTTP POST |
| health.go | Periodic health pings to all servers |

## Registry Design

Servers are loaded from `config.yaml` at startup.  Each server declares
a list of tool names it handles.  The registry builds a reverse map:
`toolName → server`.  Lookup is O(1) via a `map[string]ConnectedServer`.

## Health Checking

A goroutine pings every server every 30 seconds via `GET /health`.
Results are reported through the admin API and logged on status
changes.
