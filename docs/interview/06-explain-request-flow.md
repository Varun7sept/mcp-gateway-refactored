# Explain Request Flow

## End-to-end flow for a chat request

1. **Browser** sends `POST /api/chat` with JSON body `{"message": "...", "session_id": "..."}`
2. **Middleware stack** (logging → CORS → JWT auth):
   - `loggingMiddleware`: logs method, path, duration
   - `corsMiddleware`: validates Origin header against allowed origins
   - `authMiddleware`: extracts JWT from `Authorization: Bearer <token>`, validates, injects `username` into context
3. **ChatHandler.HandleChat**:
   - Parses request body, validates message length (max 10KB)
   - Loads session from ChatStore (MongoDB) or in-memory fallback
   - Adds user message to history
   - Updates session title if first message
   - Loads recent 10 messages for context
   - Converts to `[]map[string]string` format for LLM
   - Creates `callTool` closure that calls `Gateway.ForwardToolCall`
4. **AI Manager.RunAgent**:
   - Planner creates suggested execution plan
   - System prompt built with plan + relevant memories
   - Agent loop (max 5 iterations):
     - Call Groq API with messages + 20 tool definitions
     - Execute tool calls via gateway
     - Accumulate results
   - Final answer returned
5. **Post-processing**:
   - Save to MemoryStore
   - Save to ChatStore
   - Log request
   - Return JSON response: `{answer, steps, tools_used, latency}`

## Error paths

- Missing/invalid JWT → 401 Unauthorized
- Session not found → 403 Forbidden
- Message too long → 400 Bad Request
- AI not configured → 503 Service Unavailable
- All models failed → 500 Internal Server Error
- Tool call failed → error text fed to LLM for retry
