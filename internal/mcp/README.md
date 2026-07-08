# internal/mcp

## Why

Routes tool calls to the correct MCP server, manages server configuration,
and monitors backend health — decoupling the AI layer from server details.

## Who calls it

- `internal/ai/manager.go` — tool execution
- `internal/web/handlers/mcp.go` — raw MCP proxy
- `internal/web/handlers/admin.go` — server/tool listing

## Responsibilities

- Tool name → server URL mapping (O(1) registry lookup)
- JSON-RPC forwarding over HTTP
- Periodic server health checks

## Public entry points

```go
func New(cfg *common.Config) *Gateway
func (g *Gateway) Registry() *Registry
func (g *Gateway) ForwardToolCall(ctx context.Context, req common.MCPRequest) (*ForwardResult, error)
func NewRegistry(cfg *common.Config) *Registry
func StartHealthChecker(registry *Registry, logger *common.Logger, interval time.Duration)
```
