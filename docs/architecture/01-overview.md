# Architecture Overview

## Purpose

MCP Gateway routes natural language requests to specialized backend servers
(Model Context Protocol servers) via an AI agent loop.  Users chat with an
LLM that decides which tools to call and synthesises the results.

## High-Level Diagram

```
Browser ──→ HTTP Server ──→ Chat Handler ──→ AI Manager ──→ MCP Gateway ──→ MCP Servers
                            │                              │
                            └── MongoDB (auth, history)     └── Registry (tool → server map)
```

## Directory Layout

```
cmd/server/main.go      Entry point — wires all dependencies
internal/
  ai/                   AI orchestration, LLM calls, memory, planning
  auth/                 JWT authentication, signup, login
  common/               Shared config, logger, models
  mcp/                  Tool registry, forwarding, health checks
  storage/              MongoDB + SQLite persistence
  web/                  HTTP server, middleware, route registration
  web/handlers/         Per-feature HTTP handlers
servers/                Independent MCP servers (weather, github, …)
docs/architecture/      ADRs (this directory)
```

## Key Design Decisions

1. **Single AI entry point** — `internal/ai/manager.go` orchestrates all
   LLM interactions.  No separate planner, executor, or orchestrator layers.
2. **Feature-based handler layout** —  Each handler type gets its own file
   under `internal/web/handlers/` instead of a monolithic `server.go`.
3. **MCP registry** —  A central `Registry` maps tool names to server URLs,
   keeping the gateway and forwarder decoupled from server configuration.
4. **No human-in-the-loop** —  The approval workflow was removed for
   simplicity.  All tool calls execute immediately.
5. **Context propagation** —  Request-scoped functions accept
   `context.Context` as their first parameter to enable cancellation and
   tracing.

## Dependency Direction

```
web/handlers → web → ai → mcp → servers → external APIs
                    ↘             ↘
                     auth         storage
```

Lower layers never import higher layers.
