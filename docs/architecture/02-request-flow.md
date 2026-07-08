# Request Flow

## Chat Request Lifecycle

```
  1. Browser sends POST /api/chat { message, session_id }
  2. authMiddleware validates JWT, injects username into context
  3. ChatHandler.HandleChat reads request body
  4. ChatStore loads recent session history (or in-memory fallback)
  5. AI Manager.RunAgent is called with message + history
     a. Planner.CreatePlan analyses message → ordered task list
     b. System prompt is built with plan guidance + memories
     c. Agent loop (max 5 steps):
        i.  callLLM → model decides tool or direct answer
        ii. If tool: Gateway.ForwardToolCall → MCP server → result
        iii. Result appended to message history
     d. Final answer synthesised (or tool results combined on error)
  6. MemoryStore.Save records query + answer + tools used
  7. ChatStore.AddMessage persists AI response
  8. JSON response { answer, steps, tools_used, latency } returned
```

## Authentication Flow

```
  POST /api/auth/signup { username, email, password }
    → bcrypt hash → MongoDB insert → 201

  POST /api/auth/login { username, password }
    → bcrypt compare → JWT (HS256, 7-day expiry) → 200 + token

  Subsequent requests:
    Header: Authorization: Bearer <token>
    authMiddleware validates → injects username into context
```

## MCP Tool Call Flow

```
  Gateway.ForwardToolCall(req)
    → Registry.FindServerByTool(toolName)
    → forwardToServer(server, req)
      → HTTP POST {server.URL}/mcp/message
      → Parse JSON-RPC response
      → Return { server_name, response, latency_ms }
```

## Error Handling

- LLM model fallback: tries 3 models in sequence, returns first success
- Tool call failure: error message is returned to LLM, which may retry
- Session not found: 403 Forbidden
- Missing/invalid JWT: 401 Unauthorized
- AI not configured: 503 Service Unavailable
