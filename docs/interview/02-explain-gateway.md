# Explain the MCP Gateway

## What it is

The MCP gateway (`internal/mcp/`) decouples AI orchestration from backend server communication. It maintains a registry of tool → server mappings, forwards tool calls as JSON-RPC requests, and monitors server health.

## Architecture

```
AI Manager → Gateway.ForwardToolCall(ctx, req)
               → Registry.FindServerByTool(toolName)
               → forwardToServer(ctx, server, req)
                 → HTTP POST {server.URL}/mcp/message
```

## Key types

```go
type Gateway struct {
    registry *Registry
}

type Registry struct {
    servers []ConnectedServer
    toolMap map[string]string  // toolName → serverName
}

type ConnectedServer struct {
    Name string
    URL  string
    Tools []string
}

type ForwardResult struct {
    ServerName string
    Response   any
    Latency    time.Duration
}
```

## Why a registry?

Without a registry, the AI Manager would need to know which server handles which tool. The registry centralises this mapping:
- Config loaded once at startup from `config.yaml`
- Tool lookups are O(1) via `map[toolName]serverName`
- Servers/tools can be added without modifying AI code

## Why forwardToServer is separate

`forwardToServer` is a standalone function (not a method on Gateway) because it has a single responsibility: HTTP POST to a URL and parse JSON. It's independently testable and reusable.
