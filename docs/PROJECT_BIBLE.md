# MCP Gateway — Complete Repository Bible

> **Version:** 1.0.0
> **Language:** Go 1.25.5
> **Module:** `github.com/varunbanda/mcp-gateway`
> **Last Updated:** July 2026

---

## Table of Contents

1. [Repository Overview](#1-repository-overview)
2. [Startup Flow](#2-startup-flow)
3. [Request Flow](#3-request-flow)
4. [AI Layer Deep Dive](#4-ai-layer-deep-dive)
5. [MCP Gateway Layer](#5-mcp-gateway-layer)
6. [Authentication Layer](#6-authentication-layer)
7. [Storage Layer](#7-storage-layer)
8. [Web Layer](#8-web-layer)
9. [Handlers Deep Dive](#9-handlers-deep-dive)
10. [MCP Servers](#10-mcp-servers)
11. [Frontend](#11-frontend)
12. [Configuration](#12-configuration)
13. [Dependencies and Build](#13-dependencies-and-build)
14. [Testing Strategy](#14-testing-strategy)
15. [Deployment](#15-deployment)
16. [Common Interview Questions and Answers](#16-common-interview-questions-and-answers)
17. [Architecture Decision Records Summary](#17-architecture-decision-records-summary)
18. [Troubleshooting Guide](#18-troubleshooting-guide)

---

## 1. Repository Overview

### 1.1 Purpose of the Project

The **MCP Gateway** is a Go-based HTTP server that routes natural language chat requests to specialized MCP (Model Context Protocol) backend servers. Users interact with an AI-powered chat interface. The user types a question or command (e.g., "What's the weather in Tokyo?" or "Show me the latest crypto prices"), and the system:

1. Passes the message to an LLM (Large Language Model) via the Groq API.
2. The LLM decides which tools to call — weather, GitHub, crypto, news, search, notes, URL tools, or document Q&A.
3. The gateway forwards the tool call to the appropriate MCP backend server.
4. The LLM synthesizes the tool results into a natural-language answer.
5. The answer is returned to the user, stored in a chat session, and logged for analytics.

The project demonstrates a clean, layered architecture in Go with strong separation of concerns, a single AI entry point, a registry-based MCP layer, JWT authentication, MongoDB persistence, and a fully functional web UI.

### 1.2 Key Architectural Decisions

#### Single AI Entry Point

All LLM interactions flow through `internal/ai/manager.go`. There is no separate orchestrator, planner, or executor layer. The `Manager` struct owns the LLM client, memory store, prompt builder, and planner. This keeps the AI layer cohesive and easy to reason about.

**Rationale:** A previous version of this project had separate planner, executor, and orchestrator components. This caused tight coupling, complex data flow, and difficulty debugging. The refactored version merges all AI logic into one package with one public entry point (`RunAgent`).

#### Feature-Based Handlers

Each HTTP handler type gets its own file under `internal/web/handlers/`:
- `auth.go` — signup/login
- `chat.go` — chat CRUD
- `admin.go` — admin endpoints
- `mcp.go` — raw MCP passthrough
- `upload.go` — file upload proxy
- `dashboard.go` — static page serving

**Rationale:** A single monolithic `server.go` with all handlers becomes unmaintainable. Feature-based files make it easy to find, modify, and test specific endpoints.

#### MCP Registry Pattern

The `Registry` in `internal/mcp/registry.go` maps server names to `ConnectedServer` structs and supports O(1) lookup from tool name to server. The `Gateway` is a thin wrapper around the registry.

**Rationale:** Decouples tool routing from server configuration. Adding a new server means adding a config entry; the registry auto-discovers tools via the health check.

#### No Human-in-the-Loop

Tool calls execute immediately without an approval step.

**Rationale:** The approval workflow added complexity without proportional value for this use case. For a production enterprise deployment, an approval layer could be re-added as middleware.

#### Context Propagation

Every request-scoped function accepts `context.Context` as its first parameter. This enables:
- Request cancellation propagation through the entire call chain.
- Deadline propagation for timeouts.
- Potential future tracing/monitoring integration.

#### In-Memory Memory Store

The AI layer uses an in-memory ring buffer (max 200 entries) with keyword scoring for relevance. No vector database or embeddings are used.

**Rationale:** Simplicity. For a production system with millions of conversations, a vector database (Pinecone, Qdrant, pgvector) would be more appropriate.

### 1.3 Technology Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| Language | Go 1.25.5 | High-performance, concurrent backend |
| HTTP Framework | `net/http` (stdlib) | No external framework; Go 1.22+ routing patterns |
| LLM API | Groq API (OpenAI-compatible) | Fast inference via open-source models |
| Authentication | JWT (HS256) via `golang-jwt/jwt/v5` | Stateless auth tokens |
| Password Hashing | bcrypt via `golang.org/x/crypto` | Secure password storage |
| Primary DB | MongoDB via `mongo-driver` | Chat sessions, messages, user accounts |
| Secondary DB | SQLite via `modernc.org/sqlite` | Personal notes (embedded in each notes server) |
| Configuration | `gopkg.in/yaml.v3` | `config.yaml` parsing |
| Frontend | Vanilla HTML/CSS/JS | Dashboard and chat UI (no framework) |
| MCP Protocol | Custom JSON-RPC 2.0 | Communication between gateway and servers |

### 1.4 Who Uses It and Why

**Developers** use this project as:
- A reference architecture for building Go monoliths with clean layering.
- A learning resource for MCP (Model Context Protocol) — the emerging standard for AI tool integration.
- A starting point for building AI-powered chat applications with multi-tool orchestration.

**End users** interact through:
- The `/chat` web UI — a ChatGPT-like interface with capability suggestion buttons.
- The `/` dashboard — an admin panel showing server health, tool availability, and request statistics.

---

## 2. Startup Flow

### 2.1 Entry Point: `cmd/server/main.go`

The entire application starts from a single `main()` function.

```go
func main() {
    log.Println("Starting MCP Gateway...")

    // 1. Load Config
    cfg, err := common.LoadConfig("config.yaml")
    if err != nil { log.Fatalf("Config: %v", err) }

    // 2. Initialize Logger
    reqLogger := common.NewLogger(1000)

    // 3. Connect Database
    var chatStore *storage.ChatStore
    var authenticator *auth.Auth
    if cfg.MongoDB.URI != "" {
        authenticator, authErr = auth.New(cfg.MongoDB.URI, cfg.MongoDB.Database)
        if authErr != nil {
            log.Printf("WARNING: Auth unavailable: %v (proceeding without)", authErr)
        } else {
            chatStore = storage.NewChatStore(authenticator.DB())
        }
    }

    // 4. Initialize Gateway
    gw := mcp.New(cfg)

    // 5. Register & Start MCP Servers (goroutines)
    startMCP("weather", func() error { return weather.Start("3001") })
    startMCP("github", func() error { return github.Start("3003") })
    startMCP("crypto", func() error { return crypto.Start("3004") })
    startMCP("news", func() error { return news.Start("3005") })
    startMCP("url-tools", func() error { return url.Start("3006") })
    startMCP("search", func() error { return search.Start("3007") })
    // notes and documents have special handling

    // 6. Start Health Checker
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()
    gw.StartHealthChecker(ctx, 10*time.Second)

    // 7. Initialize AI Manager
    var brain *ai.Manager
    if groqKey := os.Getenv("GROQ_API_KEY"); groqKey != "" {
        brain = ai.New(groqKey)
    }

    // 8. Start HTTP Server
    httpServer := web.New(gw, reqLogger, brain, authenticator, chatStore, cfg.Gateway.Port)
    if err := httpServer.Start(); err != nil {
        log.Fatalf("Server: %v", err)
    }
}
```

### 2.2 Step-by-Step Initialization

#### Step 1: Load Configuration

`common.LoadConfig` reads `config.yaml`, unmarshals it into a `Config` struct, applies environment variable overrides (`MONGO_URI`, `MONGO_DATABASE`), validates server entries (no empty names, no duplicates, valid URLs), and defaults the gateway port to 8080.

**Config validation rules:**
- All servers must have non-empty names
- No duplicate server names
- Enabled servers must have a non-empty URL
- All URLs must be parseable by `url.ParseRequestURI`

#### Step 2: Initialize Logger

Creates a thread-safe in-memory ring buffer logger with capacity for 1000 entries.

```go
func NewLogger(maxLogs int) *Logger {
    return &Logger{
        logs:    make([]RequestLog, 0, maxLogs),
        maxLogs: maxLogs,
        nextID:  1,
    }
}
```

Each entry records: `ID`, `Timestamp`, `Method`, `ToolName`, `ServerName`, `Username`, `Latency`, `Status`, `Error`.

#### Step 3: Connect to MongoDB (Optional)

If MongoDB is configured, `auth.New` connects, pings the database, creates the `users` and `request_logs` collections with indexes, and returns an `Auth` instance. The database handle is extracted via `authenticator.DB()` and passed to `storage.NewChatStore`.

If MongoDB is unavailable, the server starts with reduced functionality:
- No authentication (public access)
- In-memory chat history (lost on restart)

#### Step 4: Initialize Gateway + Registry

`mcp.New` creates a `Gateway` struct whose `NewRegistry` call iterates over all enabled servers in config and populates the registry with `ConnectedServer` entries (initial status: `unknown`, empty tool list).

#### Step 5: Launch All MCP Servers

Eight MCP backend servers are started concurrently in goroutines:

| Server | Port | Package | Backend API |
|--------|------|---------|-------------|
| Weather | 3001 | `weather` | wttr.in |
| Notes | 3002 | `notes` | SQLite |
| GitHub | 3003 | `github` | GitHub REST API |
| Crypto | 3004 | `crypto` | CoinGecko |
| News | 3005 | `news` | Google News RSS |
| URL Tools | 3006 | `url` | is.gd / goqr.me |
| Search | 3007 | `search` | DuckDuckGo / Wikipedia |
| Documents | 3008 | Python script | External docs server |

The notes server uses a `*Server` struct with `New()` and `Close()` methods. The documents server is started as a Python subprocess.

#### Step 6: Start Health Checker Goroutine

```go
gw.StartHealthChecker(ctx, 10*time.Second)
```

- Runs immediately once, then every 10 seconds
- Sends MCP `initialize` and `tools/list` requests to every registered server
- Updates server status (online/offline) and discovers tools
- Uses goroutines for concurrent server checking
- Stops on context cancellation (SIGINT/SIGTERM)

#### Step 7: Initialize AI Manager

```go
var brain *ai.Manager
groqKey := os.Getenv("GROQ_API_KEY")
if groqKey != "" {
    brain = ai.New(groqKey)
    log.Println("AI Chat enabled (Groq API)")
} else {
    log.Println("AI Chat disabled (set GROQ_API_KEY)")
}
```

Without the API key, the chat endpoint returns `503 Service Unavailable`.

#### Step 8: Start HTTP Server

```go
httpServer := web.New(gw, reqLogger, brain, authenticator, chatStore, cfg.Gateway.Port)
if err := httpServer.Start(); err != nil {
    log.Fatalf("Server: %v", err)
}
```

Binds to `:8080` (or `$PORT` override). This is a blocking call.

### 2.3 Graceful Shutdown

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()
```

When Ctrl+C or SIGTERM is received:
1. The health checker goroutine stops (checks `ctx.Done()`).
2. The notes server's `defer s.Close()` closes the SQLite database.
3. The HTTP server is killed by the OS (no explicit `Shutdown` call in this version).

---

## 3. Request Flow

### 3.1 Full Trace: Browser Chat Request

```
Browser
  ¦ POST /api/chat { message: "Weather in Tokyo", session_id: "abc123" }
  ¦ Authorization: Bearer <jwt>
  ?
+-------------------------------------+
¦ loggingMiddleware                   ¦
¦  • Logs "? POST /api/chat"         ¦
¦  • Timing starts                    ¦
+-------------------------------------+
  ¦
  ?
+-------------------------------------+
¦ corsMiddleware                      ¦
¦  • Checks Origin whitelist          ¦
¦  • Sets CORS headers if matched     ¦
¦  • Handles OPTIONS preflight        ¦
+-------------------------------------+
  ¦
  ?
+-------------------------------------+
¦ authMiddleware                      ¦
¦  • Skips public paths               ¦
¦  • Extracts Bearer token            ¦
¦  • Validates JWT signature + expiry ¦
¦  • Injects username into context    ¦
+-------------------------------------+
  ¦
  ?
+-------------------------------------+
¦ ChatHandler.HandleChat              ¦
¦  • Checks AI configured             ¦
¦  • Decodes JSON body                ¦
¦  • Validates message (=10000 chars) ¦
¦  • Validates session ownership      ¦
¦  • Saves user message to ChatStore  ¦
¦  • Auto-generates session title     ¦
¦  • Loads last 10 messages           ¦
+-------------------------------------+
  ¦
  ?
+-------------------------------------+
¦ AI Manager.RunAgent                 ¦
¦  • Planner.CreatePlan (keywords)    ¦
¦  • Build system prompt              ¦
¦  • Agent loop (max 5 steps):        ¦
¦    1. callLLM ? tool or answer      ¦
¦    2. If tool: ForwardToolCall      ¦
¦    3. Append tool result            ¦
¦  • Synthesize final answer          ¦
+-------------------------------------+
  ¦
  ?
+-------------------------------------+
¦ Gateway.ForwardToolCall             ¦
¦  • Registry.FindServerByTool        ¦
¦  • Construct MCP JSON-RPC request   ¦
¦  • HTTP POST to MCP server          ¦
¦  • Parse response                   ¦
+-------------------------------------+
  ¦
  ?
+-------------------------------------+
¦ MCP Server (e.g. weather :3001)     ¦
¦  • Decode MCPRequest                ¦
¦  • Route to tool handler            ¦
¦  • Fetch external API (wttr.in)     ¦
¦  • Return MCPResponse with text     ¦
+-------------------------------------+
  ¦ Response bubbles back up
  ¦
  ChatHandler:
  ¦  • Saves AI response to store
  ¦  • Saves to MemoryStore
  ¦  • Logs request (dual: in-memory + MongoDB)
  ¦  • Returns JSON: { answer, steps, tools_used, latency_ms }
  ?
Browser renders answer in chat bubble
```

### 3.2 Error Handling at Each Layer

#### HTTP Middleware Errors

| Layer | Error | Response |
|-------|-------|----------|
| CORS | Origin not allowed | No CORS headers set (browser blocks) |
| Auth | Missing/invalid token | `401 { "error": "missing or invalid token" }` |
| Auth | Expired token | `401 { "error": "invalid or expired token" }` |
| Body | Payload >10MB | `413 Request Entity Too Large` |

#### Handler Errors

| Handler | Error | Response |
|---------|-------|----------|
| Chat | AI not configured | `503 { "error": "AI not configured (set GROQ_API_KEY)" }` |
| Chat | Empty message | `400 { "error": "message and session_id required" }` |
| Chat | Session not found | `403 { "error": "session not found" }` |
| Chat | LLM call failure | `500 { "error": "AI error: ..." }` |
| Auth | Missing fields | `400 { "error": "username, email, password required" }` |
| Auth | Duplicate user | `409 { "error": "username or email already exists" }` |
| Auth | Invalid credentials | `401 { "error": "invalid credentials" }` |
| Auth | Rate limited | `429 { "error": "too many requests" }` |
| MCP | Invalid JSON | `400 { "error": "invalid JSON: ..." }` |
| MCP | Tool not found | `502 { "error": "route to tool: ..." }` |

### 3.3 Rate Limiting

The auth handler uses a sliding window rate limiter:

```go
type rateLimiter struct {
    mu       sync.Mutex
    attempts map[string][]time.Time
    window   time.Duration
    max      int
}

func newRateLimiter(window time.Duration, max int) *rateLimiter {
    return &rateLimiter{
        attempts: make(map[string][]time.Time),
        window:   window,
        max:      max,
    }
}

func (rl *rateLimiter) Allow(ip string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    now := time.Now()
    cutoff := now.Add(-rl.window)
    var valid []time.Time
    for _, t := range rl.attempts[ip] {
        if t.After(cutoff) {
            valid = append(valid, t)
        }
    }
    if len(valid) >= rl.max {
        rl.attempts[ip] = valid
        return false
    }
    if len(valid) == 0 {
        delete(rl.attempts, ip)
    }
    rl.attempts[ip] = append(valid, now)
    return true
}
```

Configuration: **10 attempts per IP per 1-minute window** for both signup and login endpoints.

The algorithm:
1. Get current time.
2. Remove timestamps older than `now - window`.
3. If remaining timestamps >= `max`, deny.
4. Otherwise, append current timestamp and allow.

Cleanup: Entries with no valid timestamps are deleted from the map.

### 3.4 CORS Handling

```go
func corsMiddleware(next http.Handler) http.Handler {
    allowed := os.Getenv("ALLOWED_ORIGINS")
    if allowed == "" {
        allowed = "https://mcp-gateway-tvaa.onrender.com"
    }
    origins := strings.Split(allowed, ",")
    originSet := make(map[string]bool, len(origins))
    for _, o := range origins {
        originSet[strings.TrimSpace(o)] = true
    }
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")
        if origin != "" && originSet[origin] {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Vary", "Origin")
        }
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

Key behaviors:
- Whitelist-based origin matching.
- `Vary: Origin` header for proper caching.
- OPTIONS preflight handled immediately.
- Default origin is the Render.com deployment URL.

### 3.5 Logging Middleware

```go
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        log.Printf("? %s %s", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
        log.Printf("? %s %s (%s)", r.Method, r.URL.Path, time.Since(start))
    })
}
```

Every request is logged with start/end markers and duration to stdout.

## 4. AI Layer Deep Dive

The AI layer is the brain of the system. It lives entirely in `internal/ai/` and consists of seven files:

| File | Responsibility |
|------|---------------|
| `manager.go` | Public API: `New()`, `DecideAction()`, `SynthesizeAnswer()`, `RunAgent()` |
| `llm.go` | HTTP client for Groq API, model fallback logic |
| `tool_executor.go` | 20 tool definitions with schemas |
| `planner.go` | Keyword-based multi-step plan generation |
| `memory.go` | In-memory ring buffer with keyword relevance scoring |
| `prompt.go` | System prompt, synthesis prompt, retry prompt |
| `helpers.go` | `stripThink()`, `truncate()` |

### 4.1 Manager (`manager.go`)

#### Struct Definition

```go
type Manager struct {
    apiKey     string
    models     []string
    httpClient *http.Client
    memory     *MemoryStore
    prompt     *promptBuilder
    planner    *planner
    thinkRegex *regexp.Regexp
}
```

**Field Details:**

| Field | Type | Purpose |
|-------|------|---------|
| `apiKey` | `string` | Groq API key from `GROQ_API_KEY` env var |
| `models` | `[]string` | Ordered list of model names for fallback |
| `httpClient` | `*http.Client` | HTTP client with 30s timeout |
| `memory` | `*MemoryStore` | In-memory ring buffer (max 200 entries) |
| `prompt` | `*promptBuilder` | Builds system/synthesis/retry prompts |
| `planner` | `*planner` | Keyword-based pattern matching for multi-step plans |
| `thinkRegex` | `*regexp.Regexp` | Strips `<think>` tags from LLM output |

#### Constructor: `New()`

```go
func New(apiKey string) *Manager {
    models := []string{"llama-3.3-70b-versatile", "qwen/qwen3-32b", "qwen/qwen3.6-27b"}
    if configured := strings.TrimSpace(os.Getenv("GROQ_MODELS")); configured != "" {
        models = nil
        for _, m := range strings.Split(configured, ",") {
            if m = strings.TrimSpace(m); m != "" {
                models = append(models, m)
            }
        }
    }
    return &Manager{
        apiKey:     apiKey,
        models:     models,
        httpClient: &http.Client{Timeout: 30 * time.Second},
        memory:     NewMemoryStore(200),
        prompt:     newPromptBuilder(),
        planner:    newPlanner(),
        thinkRegex: regexp.MustCompile(`(?s)<think>.*?</think>\s*`),
    }
}
```

Default model fallback chain:
1. `llama-3.3-70b-versatile` (primary)
2. `qwen/qwen3-32b` (first fallback)
3. `qwen/qwen3.6-27b` (second fallback)

Environment variable `GROQ_MODELS` can override the entire list (comma-separated).

#### `Memory()` Accessor

```go
func (m *Manager) Memory() *MemoryStore {
    return m.memory
}
```

Exposes the memory store to the `ChatHandler` so it can save completed interactions (query, answer, tools used).

#### `DecideAction()` — Single-Step Decision

```go
func (m *Manager) DecideAction(ctx context.Context, userMessage string, history []map[string]string) (*ToolCallResult, error) {
    memories := ""
    entries := m.memory.QueryRelevant(userMessage, 3)
    if len(entries) > 0 {
        var parts []string
        for i, e := range entries {
            parts = append(parts, fmt.Sprintf("Past interaction %d:\n  User asked: %s\n  I answered: %s\n  Tools used: %s",
                i+1, e.Query, truncate(e.Answer, 200), strings.Join(e.ToolsUsed, ", ")))
        }
        memories = "Here are relevant past conversations for context:\n\n" + strings.Join(parts, "\n\n")
    }

    messages := []llmMessage{
        {Role: "system", Content: m.prompt.SystemPrompt(memories)},
    }
    for _, h := range history {
        role := h["role"]
        if role == "ai" { role = "assistant" }
        messages = append(messages, llmMessage{Role: role, Content: h["content"]})
    }
    messages = append(messages, llmMessage{Role: "user", Content: userMessage})

    resp, err := m.callLLM(ctx, messages, m.toolDefs())
    if err != nil { return nil, err }
    choice := resp.Choices[0]

    if len(choice.Message.ToolCalls) > 0 {
        tc := choice.Message.ToolCalls[0]
        var args map[string]any
        if tc.Function.Arguments != "" {
            json.Unmarshal([]byte(tc.Function.Arguments), &args)
        }
        if args == nil { args = map[string]any{} }
        return &ToolCallResult{
            NeedsTool: true, ToolName: tc.Function.Name,
            Arguments: args, ToolCallID: tc.ID,
        }, nil
    }

    return &ToolCallResult{
        NeedsTool: false, DirectAnswer: m.stripThink(choice.Message.Content),
    }, nil
}
```

This is the **single-step** decision function. It queries memory, builds messages, calls the LLM with tool definitions, and returns either a tool call or a direct answer.

#### `SynthesizeAnswer()` — Tool Result to Natural Language

```go
func (m *Manager) SynthesizeAnswer(ctx context.Context, userMessage, toolName, toolCallID, toolResult string) (string, error) {
    messages := []llmMessage{
        {Role: "system", Content: m.prompt.SystemPrompt("")},
        {Role: "user", Content: userMessage},
        {
            Role: "assistant",
            ToolCalls: []toolCall{
                {ID: toolCallID, Type: "function", Function: functionCall{Name: toolName, Arguments: "{}"}},
            },
        },
        {Role: "tool", Content: toolResult, ToolCallID: toolCallID},
    }
    resp, err := m.callLLM(ctx, messages, nil)
    if err != nil { return toolResult, err }
    return m.stripThink(resp.Choices[0].Message.Content), nil
}
```

Builds a four-message conversation (system, user, assistant tool call, tool result) and asks the LLM to synthesize. Falls back to raw tool result on error.

#### `RunAgent()` — The Main Agent Loop

```go
func (m *Manager) RunAgent(ctx context.Context, userMessage string, history []map[string]string, callTool func(context.Context, string, map[string]any) (string, error)) (string, []common.ToolStep, error) {
    const maxSteps = 5
    // Phase 1: Planning
    plan := m.planner.CreatePlan(userMessage)
    planGuide := ""
    if len(plan) > 0 {
        var steps []string
        for i, p := range plan {
            steps = append(steps, fmt.Sprintf("  %d. %s (use tool: %s)", i+1, p.Description, p.ToolName))
        }
        planGuide = "Suggested execution plan:\n" + strings.Join(steps, "\n") + "\nFollow this plan unless the user requests something different.\n"
    }

    messages := []llmMessage{
        {Role: "system", Content: m.prompt.SystemPrompt(planGuide)},
    }
    for _, h := range history {
        role := h["role"]
        if role == "ai" { role = "assistant" }
        messages = append(messages, llmMessage{Role: role, Content: h["content"]})
    }
    messages = append(messages, llmMessage{Role: "user", Content: userMessage})

    var steps []common.ToolStep

    // Phase 2: Document Detection
    docPattern := regexp.MustCompile(`(?i)([a-z0-9_().-]+\.(?:pdf|txt|md|csv|json|docx))`)
    if docMatch := docPattern.FindStringSubmatch(userMessage); len(docMatch) > 1 {
        args := map[string]any{"question": userMessage, "document_name": docMatch[1]}
        result, err := callTool(ctx, "ask_document", args)
        if err != nil { result = "Error: " + err.Error() }
        steps = append(steps, common.ToolStep{ToolName: "ask_document", Arguments: args, Result: result})
        messages = append(messages,
            llmMessage{Role: "assistant", ToolCalls: []toolCall{{ID: "forced_doc", Type: "function", Function: functionCall{Name: "ask_document", Arguments: "{}"}}}},
            llmMessage{Role: "tool", Content: result, ToolCallID: "forced_doc"},
            llmMessage{Role: "system", Content: "Answer using only the retrieved passages above."},
        )
    }

    // Phase 3: Agent Loop (max 5 iterations)
    for i := 0; i < maxSteps; i++ {
        resp, err := m.callLLM(ctx, messages, m.toolDefs())
        if err != nil { return "", steps, fmt.Errorf("step %d: %w", i+1, err) }
        choice := resp.Choices[0]

        if len(choice.Message.ToolCalls) == 0 {
            answer := m.stripThink(choice.Message.Content)
            if len(steps) > 0 && strings.TrimSpace(answer) == "" {
                answer = steps[len(steps)-1].Result
            }
            return answer, steps, nil
        }

        for _, tc := range choice.Message.ToolCalls {
            var args map[string]any
            json.Unmarshal([]byte(tc.Function.Arguments), &args)
            result, err := callTool(ctx, tc.Function.Name, args)
            if err != nil { result = "Error calling tool: " + err.Error() }
            steps = append(steps, common.ToolStep{ToolName: tc.Function.Name, Arguments: args, Result: result})
            messages = append(messages, llmMessage{Role: "assistant", ToolCalls: []toolCall{tc}})
            messages = append(messages, llmMessage{Role: "tool", Content: result, ToolCallID: tc.ID})
        }
    }

    // Phase 4: Force summary if max steps exhausted
    messages = append(messages, llmMessage{Role: "user", Content: "Summarize all gathered information into a final answer."})
    resp, err := m.callLLM(ctx, messages, nil)
    if err != nil {
        var combined string
        for _, s := range steps { combined += s.Result + "\n" }
        return combined, steps, nil
    }
    return m.stripThink(resp.Choices[0].Message.Content), steps, nil
}
```

**Phase 1 — Planning:** The planner suggests an ordered tool execution plan based on keyword matching. This is injected into the system prompt as guidance (not enforcement).

**Phase 2 — Document Detection:** If the user message contains a filename with a document extension (pdf, txt, md, csv, json, docx), the agent forces an `ask_document` call before entering the main loop.

**Phase 3 — Agent Loop:**
- Max 5 iterations prevents infinite loops
- Multi-tool per step — the LLM can call multiple tools in one response
- Tool errors are fed back to the LLM for implicit retry
- Empty final answer falls back to last tool result

**Phase 4 — Final Synthesis:** If the agent loop exhausts 5 steps without a final answer, it forces a summary. If even that LLM call fails, all tool results are concatenated as a raw fallback.

### 4.2 LLM Client (`llm.go`)

#### HTTP Client

```go
var forwardClient = &http.Client{Timeout: 30 * time.Second}
```

#### Message Types

```go
type llmMessage struct {
    Role       string     `json:"role"`
    Content    string     `json:"content,omitempty"`
    ToolCalls  []toolCall `json:"tool_calls,omitempty"`
    ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
    ID       string       `json:"id"`
    Type     string       `json:"type"`
    Function functionCall `json:"function"`
}

type functionCall struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"`
}
```

These implement the OpenAI chat completion message format (Groq mirrors the OpenAI API).

#### Request/Response Types

```go
type chatRequest struct {
    Model    string       `json:"model"`
    Messages []llmMessage `json:"messages"`
    Tools    []toolDef    `json:"tools,omitempty"`
}

type chatResponse struct {
    Choices []struct {
        Message llmMessage `json:"message"`
    } `json:"choices"`
    Error *struct {
        Message string `json:"message"`
    } `json:"error,omitempty"`
}
```

#### Model Fallback Chain

```go
func (m *Manager) callLLM(ctx context.Context, messages []llmMessage, tools []toolDef) (*chatResponse, error) {
    var failures []string
    for _, model := range m.models {
        req := chatRequest{Model: model, Messages: messages, Tools: tools}
        body, _ := json.Marshal(req)

        httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost,
            "https://api.groq.com/openai/v1/chat/completions",
            bytes.NewReader(body))
        httpReq.Header.Set("Content-Type", "application/json")
        httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)

        resp, err := m.httpClient.Do(httpReq)
        if err != nil {
            failures = append(failures, fmt.Sprintf("%s: %v", model, err))
            continue
        }

        respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
        resp.Body.Close()

        var chatResp chatResponse
        json.Unmarshal(respBody, &chatResp)

        if resp.StatusCode == http.StatusOK && chatResp.Error == nil && len(chatResp.Choices) > 0 {
            return &chatResp, nil
        }

        msg := http.StatusText(resp.StatusCode)
        if chatResp.Error != nil && chatResp.Error.Message != "" {
            msg = chatResp.Error.Message
        }
        failures = append(failures, fmt.Sprintf("%s: %s", model, msg))

        // Continue on retryable errors
        if resp.StatusCode == http.StatusTooManyRequests ||
            resp.StatusCode == http.StatusForbidden ||
            resp.StatusCode == http.StatusNotFound ||
            resp.StatusCode >= 500 {
            continue
        }
        return nil, fmt.Errorf("model %s: %s", model, msg)
    }
    return nil, fmt.Errorf("all models failed: %s", strings.Join(failures, "; "))
}
```

The fallback logic iterates through models in order, trying each until one succeeds:

| HTTP Status | Behavior |
|-------------|----------|
| 200 (success) | Parse response, return if valid |
| 429 (rate limit) | Continue to next model |
| 403 (forbidden) | Continue to next model |
| 404 (not found) | Continue to next model |
| 500+ (server error) | Continue to next model |
| Other 4xx | Return error immediately |

Response is limited to 4MB via `io.LimitReader`.

### 4.3 Tool Executor (`tool_executor.go`)

#### Tool Definition Types

```go
type toolDef struct {
    Type     string      `json:"type"`
    Function functionDef `json:"function"`
}

type functionDef struct {
    Name        string         `json:"name"`
    Description string         `json:"description"`
    Parameters  map[string]any `json:"parameters"`
}
```

These conform to the OpenAI tool calling format. Each `toolDef` represents a function the LLM can call.

#### ToolCallResult Type

```go
type ToolCallResult struct {
    NeedsTool    bool
    ToolName     string
    Arguments    map[string]any
    ToolCallID   string
    DirectAnswer string
}
```

Returned by `DecideAction` to indicate whether a tool should be called or a direct answer is available.

#### All 20 Tool Definitions

```go
func (m *Manager) toolDefs() []toolDef {
    return []toolDef{
        makeToolDef("get_weather", "Get real-time weather for any city", map[string]any{
            "city": map[string]any{"type": "string", "description": "City name"},
        }, []string{"city"}),
        makeToolDef("get_forecast", "Get 3-day weather forecast", map[string]any{
            "city": map[string]any{"type": "string", "description": "City name"},
        }, []string{"city"}),
        makeToolDef("get_user", "Get GitHub user profile", map[string]any{
            "username": map[string]any{"type": "string", "description": "GitHub username"},
        }, []string{"username"}),
        makeToolDef("list_repos", "List GitHub user repos by stars", map[string]any{
            "username": map[string]any{"type": "string", "description": "GitHub username"},
        }, []string{"username"}),
        makeToolDef("get_repo", "Get GitHub repo details", map[string]any{
            "owner": map[string]any{"type": "string", "description": "Repo owner"},
            "repo":  map[string]any{"type": "string", "description": "Repo name"},
        }, []string{"owner", "repo"}),
        makeToolDef("add_note", "Save a note to the database", map[string]any{
            "title":   map[string]any{"type": "string", "description": "Note title"},
            "content": map[string]any{"type": "string", "description": "Note content"},
        }, []string{"title", "content"}),
        makeToolDef("list_notes", "List all saved notes", map[string]any{}, nil),
        makeToolDef("search_notes", "Search notes by keyword", map[string]any{
            "query": map[string]any{"type": "string", "description": "Search keyword"},
        }, []string{"query"}),
        makeToolDef("get_crypto_price", "Get live crypto price", map[string]any{
            "coin": map[string]any{"type": "string", "description": "Coin ID e.g. bitcoin"},
        }, []string{"coin"}),
        makeToolDef("get_top_cryptos", "Get top 10 cryptocurrencies", map[string]any{}, nil),
        makeToolDef("get_top_news", "Get top news headlines by topic", map[string]any{
            "topic": map[string]any{"type": "string", "description": "general, technology, business, sports, science, health"},
        }, nil),
        makeToolDef("search_news", "Search news articles", map[string]any{
            "query": map[string]any{"type": "string", "description": "Search keyword"},
        }, []string{"query"}),
        makeToolDef("shorten_url", "Shorten a URL", map[string]any{
            "url": map[string]any{"type": "string", "description": "Full URL to shorten"},
        }, []string{"url"}),
        makeToolDef("generate_qr", "Generate QR code image URL", map[string]any{
            "text": map[string]any{"type": "string", "description": "Text or URL to encode"},
        }, []string{"text"}),
        makeToolDef("web_search", "Search the internet for real-time info", map[string]any{
            "query": map[string]any{"type": "string", "description": "Search query"},
        }, []string{"query"}),
        makeToolDef("wikipedia_summary", "Get Wikipedia summary", map[string]any{
            "topic": map[string]any{"type": "string", "description": "Topic name"},
        }, []string{"topic"}),
        makeToolDef("upload_document", "Upload document to knowledge base", map[string]any{
            "name":    map[string]any{"type": "string", "description": "Document name"},
            "content": map[string]any{"type": "string", "description": "Document text"},
        }, []string{"name", "content"}),
        makeToolDef("ask_document", "Ask questions about uploaded documents", map[string]any{
            "question":      map[string]any{"type": "string", "description": "Your question"},
            "document_name": map[string]any{"type": "string", "description": "Optional specific document"},
        }, []string{"question"}),
        makeToolDef("list_documents", "List all uploaded documents", map[string]any{}, nil),
    }
}
```

#### `makeToolDef` Helper

```go
func makeToolDef(name, desc string, properties map[string]any, required []string) toolDef {
    params := map[string]any{
        "type": "object", "properties": properties, "additionalProperties": false,
    }
    if required != nil {
        params["required"] = required
    }
    return toolDef{
        Type: "function",
        Function: functionDef{
            Name: name, Description: desc, Parameters: params,
        },
    }
}
```

Key behaviors:
- `additionalProperties: false` prevents the LLM from inventing parameters.
- `required: nil` = optional parameter.
- `required: []string{"city"}` = required parameter.
- Empty `properties` with `nil required` = no parameters needed.

#### Tool Categories

| Category | Tools | MCP Server | Port |
|----------|-------|------------|------|
| Weather | `get_weather`, `get_forecast` | weather | 3001 |
| Notes | `add_note`, `list_notes`, `search_notes` | notes | 3002 |
| GitHub | `get_user`, `list_repos`, `get_repo` | github | 3003 |
| Crypto | `get_crypto_price`, `get_top_cryptos` | crypto | 3004 |
| News | `get_top_news`, `search_news` | news | 3005 |
| URL | `shorten_url`, `generate_qr` | url-tools | 3006 |
| Search | `web_search`, `wikipedia_summary` | search | 3007 |
| Documents | `upload_document`, `ask_document`, `list_documents` | documents | 3008 |

### 4.4 Planner (`planner.go`)

The planner attempts to predict what sequence of tool calls would satisfy the user's request. This is a keyword-based heuristic — not an LLM-based planner. The plan is injected into the system prompt as a suggestion, not a command.

#### PlanStep and Planner Types

```go
type planStep struct {
    Description string
    ToolName    string
    Priority    int
}

type planner struct {
    patterns map[string][]planStep
}
```

#### Constructor

```go
func newPlanner() *planner {
    return &planner{
        patterns: map[string][]planStep{
            "weather+news": {
                {Description: "Fetch weather data", ToolName: "get_weather", Priority: 1},
                {Description: "Fetch news headlines", ToolName: "get_top_news", Priority: 2},
            },
            "github+weather": {
                {Description: "Fetch GitHub profile", ToolName: "get_user", Priority: 1},
                {Description: "Fetch weather data", ToolName: "get_weather", Priority: 2},
            },
        },
    }
}
```

#### CreatePlan() — Pattern Matching

```go
func (p *planner) CreatePlan(userMessage string) []planStep {
    lower := strings.ToLower(userMessage)

    hasWeather := strings.Contains(lower, "weather") || strings.Contains(lower, "temperature") || strings.Contains(lower, "forecast")
    hasNews := strings.Contains(lower, "news") || strings.Contains(lower, "headline") || strings.Contains(lower, "happening")
    hasGitHub := strings.Contains(lower, "github") || strings.Contains(lower, "git") || strings.Contains(lower, "repo")
    hasCrypto := strings.Contains(lower, "bitcoin") || strings.Contains(lower, "crypto") || strings.Contains(lower, "ethereum")
    hasSearch := strings.Contains(lower, "search") || strings.Contains(lower, "find") || strings.Contains(lower, "look up")
    hasWiki := strings.Contains(lower, "wikipedia") || strings.Contains(lower, "who is") || strings.Contains(lower, "what is")

    if hasWeather && hasNews {
        return p.patterns["weather+news"]
    }
    if hasGitHub && hasWeather {
        return p.patterns["github+weather"]
    }
    if hasCrypto && hasSearch {
        return []planStep{
            {Description: "Fetch cryptocurrency price", ToolName: "get_crypto_price", Priority: 1},
            {Description: "Search for additional context", ToolName: "web_search", Priority: 2},
        }
    }
    if hasWiki && hasNews {
        return []planStep{
            {Description: "Get Wikipedia summary", ToolName: "wikipedia_summary", Priority: 1},
            {Description: "Fetch related news", ToolName: "search_news", Priority: 2},
        }
    }
    return nil
}
```

Four patterns are matched (in order):
1. **weather+news** ? `get_weather` ? `get_top_news`
2. **github+weather** ? `get_user` ? `get_weather`
3. **crypto+search** ? `get_crypto_price` ? `web_search`
4. **wikipedia+news** ? `wikipedia_summary` ? `search_news`

**Fallback:** Returns `nil` for unrecognized patterns. The LLM decides entirely on its own.

### 4.5 Memory Store (`memory.go`)

#### MemoryEntry Struct

```go
type MemoryEntry struct {
    Query     string    `json:"query"`
    Answer    string    `json:"answer"`
    ToolsUsed []string  `json:"tools_used"`
    Timestamp time.Time `json:"timestamp"`
}
```

Records what the user asked, what the AI answered, which tools were used, and when it happened.

#### MemoryStore Struct

```go
type MemoryStore struct {
    mu      sync.RWMutex
    entries []MemoryEntry
    maxSize int
}
```

A thread-safe ring buffer with configurable maximum size (default 200).

#### Constructor

```go
func NewMemoryStore(maxSize int) *MemoryStore {
    return &MemoryStore{
        entries: make([]MemoryEntry, 0, maxSize),
        maxSize: maxSize,
    }
}
```

#### Save()

```go
func (m *MemoryStore) Save(entry MemoryEntry) {
    m.mu.Lock()
    defer m.mu.Unlock()
    entry.Timestamp = time.Now()
    m.entries = append(m.entries, entry)
    if len(m.entries) > m.maxSize {
        m.entries = m.entries[len(m.entries)-m.maxSize:]
    }
}
```

Appends the entry with current timestamp. If the ring buffer exceeds `maxSize`, the oldest entries are discarded.

#### QueryRelevant() — Keyword Scoring Algorithm

```go
func (m *MemoryStore) QueryRelevant(query string, limit int) []MemoryEntry {
    m.mu.RLock()
    defer m.mu.RUnlock()

    type scored struct {
        entry MemoryEntry
        score int
    }
    queryWords := tokenize(query)
    var scoredEntries []scored
    for _, entry := range m.entries {
        score := 0
        entryWords := tokenize(entry.Query + " " + entry.Answer)
        for _, qw := range queryWords {
            for _, ew := range entryWords {
                if strings.EqualFold(qw, ew) {
                    score++
                }
            }
        }
        if score > 0 {
            scoredEntries = append(scoredEntries, scored{entry, score})
        }
    }

    // Sort by score descending
    for i := 0; i < len(scoredEntries); i++ {
        for j := i + 1; j < len(scoredEntries); j++ {
            if scoredEntries[j].score > scoredEntries[i].score {
                scoredEntries[i], scoredEntries[j] = scoredEntries[j], scoredEntries[i]
            }
        }
    }

    if limit > len(scoredEntries) { limit = len(scoredEntries) }
    result := make([]MemoryEntry, limit)
    for i := 0; i < limit; i++ {
        result[i] = scoredEntries[i].entry
    }
    return result
}
```

The scoring algorithm:
1. Tokenizes the query into words (=3 characters, alphanumeric).
2. For each memory entry, tokenizes the combined query+answer text.
3. Counts case-insensitive word matches between query tokens and entry tokens.
4. Only entries with score > 0 are returned.
5. Entries are sorted by score descending.
6. Returns at most `limit` entries (typically 3).

#### tokenize() Helper

```go
func tokenize(s string) []string {
    var words []string
    var current []rune
    for _, r := range s {
        if unicode.IsLetter(r) || unicode.IsDigit(r) {
            current = append(current, unicode.ToLower(r))
        } else if len(current) > 0 {
            if len(current) > 2 {
                words = append(words, string(current))
            }
            current = nil
        }
    }
    if len(current) > 2 {
        words = append(words, string(current))
    }
    return words
}
```

Behavior:
- Splits on non-alphanumeric characters
- Converts to lowercase
- Filters out words with =2 characters (filters "is", "in", "at", "to", etc.)

### 4.6 Prompt Builder (`prompt.go`)

#### SystemPrompt()

```go
func (p *promptBuilder) SystemPrompt(memories string) string {
    base := `You are a helpful AI assistant with access to real-time tools.

Capabilities: weather forecasts, GitHub data, crypto prices, news, web search, Wikipedia, notes, URL shortener/QR, document Q&A.

TOOL SELECTION RULES:
- Weather questions -> get_weather or get_forecast
- GitHub profiles/repos -> get_user, list_repos, get_repo
- Crypto prices -> get_crypto_price or get_top_cryptos
- Breaking news, current events -> search_news
- Facts about person/place/event -> wikipedia_summary
- Niche queries, real-time stats -> web_search
- Notes -> add_note, list_notes, search_notes
- URLs/QR -> shorten_url, generate_qr
- Documents -> upload_document, ask_document, list_documents

GOLDEN RULES:
1. NEVER answer stats, records, or numbers from memory - always use a tool.
2. NEVER use both search_news AND web_search for the same question - pick one.
3. Resolve pronouns from conversation history before calling a tool.
4. Be concise, factual, and conversational.
5. Strip any <think> tags from responses.`

    if memories != "" {
        base += "\n\n" + memories
    }
    return base
}
```

**Key rules:**
- **Golden Rule 1:** Prevents hallucination of real-time data.
- **Golden Rule 2:** Prevents redundant tool calls.
- **Golden Rule 3:** Enables pronoun resolution across turns.
- Memories (relevant past interactions) are appended at the bottom.
- The LLM is told to strip `<think>` tags, and the code also removes them server-side.

#### SynthesisPrompt()

```go
func (p *promptBuilder) SynthesisPrompt(userMessage string, results []string) string {
    joined := ""
    for _, r := range results {
        joined += r + "\n\n"
    }
    return fmt.Sprintf(`You are an AI assistant that synthesizes tool results into a helpful natural answer.
Answer the user's question directly using the data from the tool results.
If results include lists, present them as clean bullet points.
Combine multiple tool results into ONE coherent answer.
Be concise but complete. Aim for 2-6 sentences or a short bulleted list.

User asked: %s

Tool results:
%s

Now write a helpful answer:`, userMessage, joined)
}
```

Used when multiple tool results need to be combined. Note: This is defined but not currently called from the main agent loop — synthesis is done inline in `RunAgent`.

#### RetryPrompt()

```go
func (p *promptBuilder) RetryPrompt(taskDesc, toolName string, args map[string]any, errMsg string) string {
    return fmt.Sprintf(`A tool call failed. Suggest an alternative approach to accomplish the same goal.
If no alternative exists, respond with: {"alternative":false}
Otherwise respond with JSON: {"alternative":true,"tool":"tool_name","arguments":{...},"description":"..."}

Task: %s
Tool called: %s
Arguments: %v
Error: %s`, taskDesc, toolName, args, errMsg)
}
```

This is a recovery prompt asking the LLM to suggest an alternative approach when a tool fails. Note: Defined but not currently used — tool errors are returned directly to the LLM in `RunAgent`.

### 4.7 Helpers (`helpers.go`)

```go
func (m *Manager) stripThink(s string) string {
    return strings.TrimSpace(m.thinkRegex.ReplaceAllString(s, ""))
}

func truncate(s string, max int) string {
    runes := []rune(s)
    if len(runes) <= max { return s }
    return string(runes[:max]) + "..."
}
```

- `stripThink`: Removes `<think>...</think>` blocks from LLM responses.
- `truncate`: Truncates strings at `max` runes (not bytes, for Unicode safety). Used for memory display (200 chars).

## 5. MCP Gateway Layer

The MCP Gateway layer lives in `internal/mcp/` and provides tool discovery, request routing, forwarding, and health monitoring. It consists of four files:

| File | Responsibility |
|------|---------------|
| `gateway.go` | Thin wrapper over Registry |
| `registry.go` | Server + tool registry from config |
| `forwarder.go` | HTTP forwarding of MCP JSON-RPC requests |
| `health.go` | Periodic health checks for all servers |

### 5.1 Gateway (`gateway.go`)

The `Gateway` is intentionally thin — it is a wrapper that holds the registry and provides a single `ForwardToolCall` method.

```go
type Gateway struct {
    registry *Registry
}

func New(cfg *common.Config) *Gateway {
    return &Gateway{
        registry: NewRegistry(cfg),
    }
}

func (g *Gateway) Registry() *Registry {
    return g.registry
}
```

**Design rationale:** The Gateway exists as a separate type so that:
1. It can be extended with additional functionality (caching, rate limiting, circuit breakers).
2. It presents a clean public API: `mcp.New(cfg)` rather than `&mcp.Registry{...}`.
3. The ForwardToolCall method logically belongs on the gateway (it uses the registry).

### 5.2 Registry (`registry.go`)

#### Server Status Types

```go
type ServerStatus string

const (
    StatusOnline  ServerStatus = "online"
    StatusOffline ServerStatus = "offline"
    StatusUnknown ServerStatus = "unknown"
)
```

#### Tool and ConnectedServer Types

```go
type Tool struct {
    Name       string `json:"name"`
    Desc       string `json:"description"`
    ServerName string `json:"server_name"`
}

type ConnectedServer struct {
    Name      string
    URL       string
    Status    ServerStatus
    Tools     []Tool
    LastCheck time.Time
    Latency   time.Duration
}
```

`Tool` represents a single tool discovered on an MCP server. `ConnectedServer` holds the runtime state of a registered server.

#### Registry Struct

```go
type Registry struct {
    mu      sync.RWMutex
    servers map[string]*ConnectedServer
}
```

The servers map is keyed by server name (from config). Pointers are used so status updates modify in place without map reassignment.

#### Loading from Config

```go
func NewRegistry(cfg *common.Config) *Registry {
    r := &Registry{servers: make(map[string]*ConnectedServer)}
    for _, sc := range cfg.Servers {
        if !sc.Enabled { continue }
        r.servers[sc.Name] = &ConnectedServer{
            Name: sc.Name, URL: sc.URL,
            Status: StatusUnknown, Tools: []Tool{},
        }
    }
    return r
}
```

Only enabled servers are registered. Initial status is `unknown`. Tools are populated by the health checker.

#### FindServerByTool — O(1) Lookup

```go
func (r *Registry) FindServerByTool(toolName string) (ConnectedServer, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    for _, s := range r.servers {
        for _, t := range s.Tools {
            if t.Name == toolName {
                return *s, nil
            }
        }
    }
    return ConnectedServer{}, fmt.Errorf("no server for tool %q", toolName)
}
```

Scans all servers and their tools. While O(n*m) in theory, in practice n = 10 servers and m = 5 tools per server, making it effectively O(1).

#### ListServers() / ListTools() / GetServer()

```go
func (r *Registry) ListServers() []ConnectedServer {
    r.mu.RLock()
    defer r.mu.RUnlock()
    result := make([]ConnectedServer, 0, len(r.servers))
    for _, s := range r.servers { result = append(result, *s) }
    return result
}

func (r *Registry) ListTools() []Tool {
    r.mu.RLock()
    defer r.mu.RUnlock()
    var all []Tool
    for _, s := range r.servers {
        if s.Status == StatusOnline {
            all = append(all, s.Tools...)
        }
    }
    return all
}

func (r *Registry) GetServer(name string) (ConnectedServer, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    s, ok := r.servers[name]
    if !ok { return ConnectedServer{}, fmt.Errorf("server %q not found", name) }
    return *s, nil
}
```

- `ListTools` only returns tools from **online** servers.
- `GetServer` is used by the upload handler to find the documents server.

#### UpdateStatus

```go
func (r *Registry) UpdateStatus(name string, status ServerStatus, tools []Tool, latency time.Duration) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if s, ok := r.servers[name]; ok {
        s.Status = status
        s.LastCheck = time.Now()
        s.Latency = latency
        if tools != nil { s.Tools = tools }
    }
}
```

Called by the health checker to update server runtime state. Tools are only overwritten when `tools != nil` (on health check success).

### 5.3 Forwarder (`forwarder.go`)

#### ForwardResult Type

```go
type ForwardResult struct {
    ServerName string        `json:"server_name"`
    Response   any           `json:"response"`
    Latency    time.Duration `json:"latency_ms"`
}
```

#### ForwardToolCall

```go
func (g *Gateway) ForwardToolCall(ctx context.Context, req common.MCPRequest) (*ForwardResult, error) {
    toolName, _ := req.Params["name"].(string)
    if toolName == "" { return nil, fmt.Errorf("missing tool name") }

    server, err := g.registry.FindServerByTool(toolName)
    if err != nil { return nil, fmt.Errorf("route to tool %q: %w", toolName, err) }

    start := time.Now()
    response, err := forwardToServer(ctx, server, req)
    latency := time.Since(start)
    if err != nil { return nil, fmt.Errorf("forward to %s: %w", server.Name, err) }

    return &ForwardResult{ServerName: server.Name, Response: response, Latency: latency}, nil
}
```

Steps:
1. Extract tool name from the MCP request parameters.
2. Find the server that hosts this tool via the registry.
3. Forward the request to the server via HTTP.
4. Return the result with server name and latency timing.

#### forwardToServer

```go
func forwardToServer(ctx context.Context, server ConnectedServer, req common.MCPRequest) (any, error) {
    body, err := json.Marshal(req)
    if err != nil { return nil, fmt.Errorf("marshal: %w", err) }

    url := server.URL + "/mcp/message"
    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
    if err != nil { return nil, fmt.Errorf("create request: %w", err) }
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := forwardClient.Do(httpReq)
    if err != nil { return nil, fmt.Errorf("http: %w", err) }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
    if err != nil { return nil, fmt.Errorf("read: %w", err) }
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
    }

    var response any
    if err := json.Unmarshal(respBody, &response); err != nil {
        return nil, fmt.Errorf("parse: %w", err)
    }
    return response, nil
}
```

Key behaviors:
- Context propagation: `http.NewRequestWithContext` enables cancellation.
- 10MB response limit (`io.LimitReader`).
- Response parsed as `any` (flexible JSON structure).
- Non-200 status codes return full body in error message.
- 30-second HTTP client timeout (`forwardClient`).

### 5.4 Health Checker (`health.go`)

#### StartHealthChecker

```go
func (g *Gateway) StartHealthChecker(ctx context.Context, interval time.Duration) {
    g.checkAll()
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                log.Println("Health checker stopped")
                return
            case <-ticker.C:
                g.checkAll()
            }
        }
    }()
    log.Printf("Health checker started (%s interval)", interval)
}
```

Runs immediately once, then at the configured interval (default: 10 seconds). Stops when context is cancelled (SIGINT/SIGTERM).

#### checkAll

```go
func (g *Gateway) checkAll() {
    servers := g.registry.ListServers()
    var wg sync.WaitGroup
    for _, s := range servers {
        wg.Add(1)
        go func(s ConnectedServer) {
            defer wg.Done()
            tools, latency, err := checkServer(s)
            if err != nil {
                log.Printf("  [%s] OFFLINE — %v", s.Name, err)
                g.registry.UpdateStatus(s.Name, StatusOffline, nil, 0)
            } else {
                log.Printf("  [%s] ONLINE — %d tools, %s", s.Name, len(tools), latency)
                g.registry.UpdateStatus(s.Name, StatusOnline, tools, latency)
            }
        }(s)
    }
    wg.Wait()
}
```

All servers are checked concurrently via goroutines. Status is updated atomically via `UpdateStatus`.

#### Health Probe Protocol

```go
func checkServer(server ConnectedServer) ([]Tool, time.Duration, error) {
    start := time.Now()
    url := server.URL + "/mcp/message"

    // Step 1: Initialize
    initReq := mcpRequest{
        JSONRPC: "2.0", ID: 1, Method: "initialize",
        Params: map[string]any{
            "protocolVersion": "2024-11-05",
            "capabilities":    map[string]any{},
            "clientInfo":      map[string]any{"name": "mcp-gateway", "version": "1.0.0"},
        },
    }
    if _, err := sendMCP(healthClient, url, initReq); err != nil {
        return nil, 0, fmt.Errorf("initialize: %w", err)
    }

    // Step 2: List tools
    toolsReq := mcpRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"}
    resp, err := sendMCP(healthClient, url, toolsReq)
    if err != nil { return nil, 0, fmt.Errorf("tools/list: %w", err) }

    latency := time.Since(start)
    tools, err := parseTools(resp, server.Name)
    if err != nil { return nil, latency, fmt.Errorf("parse tools: %w", err) }
    return tools, latency, nil
}
```

The health check uses the standard MCP handshake:
1. Send `initialize` — verifies the server is alive and supports MCP.
2. Send `tools/list` — discovers available tools.
3. Measure total round-trip latency.

#### sendMCP and parseTools

```go
func sendMCP(client *http.Client, url string, req mcpRequest) (*mcpResponse, error) {
    body, _ := json.Marshal(req)
    resp, err := client.Post(url, "application/json", bytes.NewReader(body))
    if err != nil { return nil, fmt.Errorf("http: %w", err) }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK { return nil, fmt.Errorf("status %d", resp.StatusCode) }
    var mcpResp mcpResponse
    json.NewDecoder(resp.Body).Decode(&mcpResp)
    if mcpResp.Error != nil { return nil, fmt.Errorf("mcp error: %v", mcpResp.Error) }
    return &mcpResp, nil
}

func parseTools(resp *mcpResponse, serverName string) ([]Tool, error) {
    resultMap, ok := resp.Result.(map[string]any)
    if !ok { return nil, fmt.Errorf("unexpected result type") }
    toolsRaw, ok := resultMap["tools"].([]any)
    if !ok { return []Tool{}, nil }

    var tools []Tool
    seen := make(map[string]bool)
    for _, t := range toolsRaw {
        toolMap, ok := t.(map[string]any)
        if !ok { continue }
        name, _ := toolMap["name"].(string)
        if name == "" || seen[name] { continue }
        seen[name] = true
        desc, _ := toolMap["description"].(string)
        tools = append(tools, Tool{Name: name, Desc: desc, ServerName: serverName})
    }
    return tools, nil
}
```

- Health checks use a separate client with 5-second timeout (`healthClient`).
- `parseTools` handles unexpected response shapes gracefully and deduplicates tools by name.

---

## 6. Authentication Layer

The authentication layer lives in `internal/auth/auth.go` and handles user management, JWT tokens, and request logging.

### 6.1 User Struct

```go
type User struct {
    Username  string    `bson:"username" json:"username"`
    Email     string    `bson:"email" json:"email"`
    Password  string    `bson:"password" json:"-"`
    CreatedAt time.Time `bson:"created_at" json:"created_at"`
}
```

The `Password` field has `json:"-"` to prevent it from being serialized in API responses.

### 6.2 Auth Struct

```go
type Auth struct {
    users       *mongo.Collection
    requestLogs *mongo.Collection
    jwtSecret   []byte
    db          *mongo.Database
}

func (a *Auth) DB() *mongo.Database { return a.db }
```

`DB()` exposes the raw MongoDB database handle for the `ChatStore` to use (same DB, different collections).

### 6.3 Context Key

```go
type ctxKey string
const UserKey ctxKey = "username"

func UserFromContext(ctx context.Context) (string, bool) {
    v := ctx.Value(UserKey)
    if v == nil { return "", false }
    s, ok := v.(string)
    return s, ok
}
```

Standard Go pattern for context value injection. The middleware sets `UserKey` after JWT validation, and handlers read it via `UserFromContext`.

### 6.4 Connection and Indexes

```go
func New(uri, database string) (*Auth, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
    if err != nil { return nil, fmt.Errorf("mongo connect: %w", err) }
    if err := client.Ping(ctx, nil); err != nil { return nil, fmt.Errorf("mongo ping: %w", err) }

    secret := os.Getenv("JWT_SECRET")
    if secret == "" { return nil, fmt.Errorf("JWT_SECRET environment variable required") }

    db := client.Database(database)
    a := &Auth{
        users:       db.Collection("users"),
        requestLogs: db.Collection("request_logs"),
        jwtSecret:   []byte(secret),
    }
    if err := a.ensureIndexes(ctx); err != nil { return nil, fmt.Errorf("indexes: %w", err) }
    return a, nil
}
```

Indexes created:
- `users`: unique index on `username`, unique index on `email`
- `request_logs`: compound index on `username` + `created_at` (descending)

### 6.5 Signup Flow

```go
func (a *Auth) Signup(username, email, password string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if len(password) < 6 { return "", fmt.Errorf("password must be at least 6 characters") }

    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil { return "", fmt.Errorf("hash password: %w", err) }

    user := User{Username: username, Email: email, Password: string(hash), CreatedAt: time.Now()}
    if _, err := a.users.InsertOne(ctx, user); err != nil {
        if mongo.IsDuplicateKeyError(err) {
            return "", fmt.Errorf("username or email already exists")
        }
        return "", fmt.Errorf("insert user: %w", err)
    }
    return a.generateToken(username)
}
```

1. Validates password length (=6 characters).
2. Hashes password with bcrypt (default cost = 10).
3. Inserts user into MongoDB.
4. On duplicate key error, returns a user-friendly message.
5. Returns a JWT token (user is logged in immediately after signup).

### 6.6 Login Flow

```go
func (a *Auth) Login(username, password string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    var user User
    if err := a.users.FindOne(ctx, bson.M{"username": username}).Decode(&user); err != nil {
        return "", fmt.Errorf("invalid credentials")
    }
    if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
        return "", fmt.Errorf("invalid credentials")
    }
    return a.generateToken(username)
}
```

The error message is intentionally generic (`"invalid credentials"`) to prevent username enumeration.

### 6.7 JWT Token Generation

```go
func (a *Auth) generateToken(username string) (string, error) {
    claims := jwt.MapClaims{
        "sub": username,
        "iat": time.Now().Unix(),
        "exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(a.jwtSecret)
}
```

- Algorithm: HS256 (HMAC-SHA256)
- Claims: `sub` (username), `iat` (issued at), `exp` (expiration — 7 days)

### 6.8 Token Validation

```go
func (a *Auth) ValidateToken(tokenStr string) (string, error) {
    token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return a.jwtSecret, nil
    })
    if err != nil { return "", fmt.Errorf("invalid token") }
    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok || !token.Valid { return "", fmt.Errorf("invalid token claims") }
    username, ok := claims["sub"].(string)
    if !ok { return "", fmt.Errorf("invalid token subject") }
    return username, nil
}
```

1. Parses the JWT, extracting the signing key.
2. Validates the signing method is HMAC (prevents algorithm confusion attacks).
3. Verifies the signature and expiry.
4. Extracts the `sub` claim as the username.

### 6.9 Request Logging

```go
func (a *Auth) LogRequest(username, method, toolName, serverName, status, errMsg string, latency time.Duration) {
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        a.requestLogs.InsertOne(ctx, bson.M{
            "username": username, "method": method, "tool_name": toolName,
            "server_name": serverName, "status": status, "error": errMsg,
            "latency_ms": latency.Milliseconds(), "created_at": time.Now(),
        })
    }()
}
```

Runs in a goroutine with 5-second timeout. Failures are silently dropped (fire-and-forget).

### 6.10 GetStats Aggregation Pipeline

```go
func (a *Auth) GetRequestStats(username string) map[string]any {
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    match := bson.M{}
    if username != "" { match = bson.M{"username": username} }

    pipeline := mongo.Pipeline{
        {{Key: "$match", Value: match}},
        {{Key: "$group", Value: bson.M{
            "_id": nil, "total_requests": bson.M{"$sum": 1},
            "success_count": bson.M{"$sum": bson.M{"$cond": []any{bson.M{"$eq": []string{"$status", "success"}}, 1, 0}}},
            "error_count":   bson.M{"$sum": bson.M{"$cond": []any{bson.M{"$eq": []string{"$status", "error"}}, 1, 0}}},
            "avg_latency":   bson.M{"$avg": "$latency_ms"},
        }}},
    }

    stats := map[string]any{
        "total_requests": 0, "success_count": 0, "error_count": 0, "avg_latency_ms": 0,
        "requests_by_tool": map[string]int{}, "requests_by_server": map[string]int{},
    }

    cursor, err := a.requestLogs.Aggregate(ctx, pipeline)
    if err != nil { return stats }
    defer cursor.Close(ctx)
    var results []bson.M
    cursor.All(ctx, &results)
    if len(results) > 0 {
        r := results[0]
        if v, ok := r["total_requests"]; ok { stats["total_requests"] = v }
        if v, ok := r["success_count"]; ok { stats["success_count"] = v }
        if v, ok := r["error_count"]; ok { stats["error_count"] = v }
        if v, ok := r["avg_latency"]; ok { stats["avg_latency_ms"] = v }
    }
    return stats
}
```

The MongoDB aggregation pipeline computes: total request count, success count, error count, and average latency. Note: `requests_by_tool` and `requests_by_server` are **not** populated by the MongoDB aggregation — they remain empty maps.

### 6.11 RecentLogs Query

```go
func (a *Auth) RecentLogs(n int, username string) []bson.M {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    filter := bson.M{}
    if username != "" { filter = bson.M{"username": username} }
    cursor, err := a.requestLogs.Find(ctx, filter,
        options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(int64(n)))
    if err != nil { return nil }
    defer cursor.Close(ctx)
    var results []bson.M
    cursor.All(ctx, &results)
    return results
}
```

Returns the most recent N logs, optionally filtered by username.

### 6.12 No Refresh Token Support

The system uses a single long-lived JWT (7 days). There is no refresh token mechanism.

**Rationale:** Refresh tokens add significant complexity (storage, rotation, revocation) without proportional benefit for this use case. For a production system with strict security requirements, refresh tokens should be added.

---

## 7. Storage Layer

The storage layer lives in `internal/storage/` and consists of two stores:

| File | Responsibility |
|------|---------------|
| `mongo.go` | Chat session and message persistence in MongoDB |
| `notes.go` | Notes store interface (SQLite) |

### 7.1 Chat Store (`mongo.go`)

#### Collections

```go
type ChatStore struct {
    sessions *mongo.Collection
    messages *mongo.Collection
}
```

- **`chat_sessions`** — stores session metadata (id, username, title, timestamps)
- **`chat_messages`** — stores individual messages within sessions

#### Indexes

```go
s.sessions.Indexes().CreateOne(ctx, mongo.IndexModel{
    Keys: bson.D{{Key: "username", Value: 1}, {Key: "updated_at", Value: -1}},
})
s.messages.Indexes().CreateOne(ctx, mongo.IndexModel{
    Keys: bson.D{{Key: "session_id", Value: 1}, {Key: "created_at", Value: 1}},
})
```

- `chat_sessions`: Compound index on `(username, -updated_at)` for efficient session listing.
- `chat_messages`: Compound index on `(session_id, created_at)` for efficient message retrieval.

#### Session CRUD

**CreateSession:** Uses MongoDB ObjectID for session IDs.
**ListSessions:** Returns sessions sorted by `updated_at` descending.
**GetSession:** Enforces ownership — only the session owner can access it.
**DeleteSession:** Deletes both the session and all its messages.
**UpdateSessionTitle:** Updates title and bumps `updated_at`. Auto-titling uses the first user message (truncated to 50 chars).

#### Message Operations

**AddMessage:** Inserts a message and updates the session's `updated_at`.
**GetMessages:** Returns all messages sorted chronologically (ascending).
**GetRecentMessages:** Returns the most recent N messages in chronological order (fetches descending from DB, then reverses).

#### BSON Conversion Helpers

```go
func bsonToSession(r bson.M) common.ChatSession {
    s := common.ChatSession{
        Username: getStr(r, "username"), Title: getStr(r, "title"),
        CreatedAt: getTime(r, "created_at"), UpdatedAt: getTime(r, "updated_at"),
    }
    if id, ok := r["_id"].(primitive.ObjectID); ok { s.ID = id.Hex() }
    return s
}

func bsonToMessage(r bson.M) common.ChatMessage {
    m := common.ChatMessage{
        Role: getStr(r, "role"), Content: getStr(r, "content"),
        CreatedAt: getTime(r, "created_at"),
    }
    if meta, ok := r["meta"]; ok {
        if mm, ok := meta.(map[string]any); ok { m.Meta = mm }
        else if mm, ok := meta.(bson.M); ok { m.Meta = mm }
    }
    return m
}
```

### 7.2 Notes Store (`notes.go`)

```go
type NotesStore struct {
    db *sql.DB
}

func NewNotesStore(path string) (*NotesStore, error) {
    db, err := sql.Open("sqlite", path)
    if err != nil { return nil, err }
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS notes (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            title TEXT NOT NULL,
            content TEXT NOT NULL,
            tags TEXT DEFAULT '',
            username TEXT DEFAULT '',
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`)
    if err != nil { return nil, fmt.Errorf("create table: %w", err) }
    return &NotesStore{db: db}, nil
}
```

**Important note:** This `NotesStore` in `internal/storage/` is a standalone store intended for use by the gateway layer. However, the MCP notes server (in `servers/notes/server.go`) has its own SQLite database managed independently. The `internal/storage/notes.go` is **not currently wired into the main application**.

---

## 8. Web Layer

The web layer lives in `internal/web/` and provides the HTTP server, middleware, and route registration.

### 8.1 Server (`server.go`)

#### Server Struct

```go
type Server struct {
    gateway     *mcp.Gateway
    logger      *common.Logger
    brain       *ai.Manager
    auth        *auth.Auth
    chatStore   *storage.ChatStore
    authLimiter *rateLimiter
    memHistory  map[string][]common.ChatMessage
    port        int
}

func New(gw *mcp.Gateway, logger *common.Logger, brain *ai.Manager,
    authenticator *auth.Auth, chatStore *storage.ChatStore, port int) *Server {
    return &Server{
        gateway: gw, logger: logger, brain: brain, auth: authenticator,
        chatStore: chatStore, port: port,
        authLimiter: newRateLimiter(time.Minute, 10),
        memHistory:  make(map[string][]common.ChatMessage),
    }
}
```

The `Server` struct wires all dependencies together. `memHistory` is an in-memory fallback for chat sessions when MongoDB is unavailable.

### 8.2 Middleware (`middleware.go`)

#### clientIP

```go
func clientIP(r *http.Request) string {
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        if i := strings.Index(xff, ","); i != -1 {
            return strings.TrimSpace(xff[:i])
        }
        return strings.TrimSpace(xff)
    }
    ip, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil { return r.RemoteAddr }
    return ip
}
```

Respects `X-Forwarded-For` for reverse proxy deployments. Takes the first IP in the chain.

#### loggingMiddleware

```go
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        log.Printf("? %s %s", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
        log.Printf("? %s %s (%s)", r.Method, r.URL.Path, time.Since(start))
    })
}
```

#### corsMiddleware

Whitelist-based CORS. Default origin is `https://mcp-gateway-tvaa.onrender.com`.

#### authMiddleware

```go
func authMiddleware(a *auth.Auth) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if r.Method == http.MethodOptions {
                next.ServeHTTP(w, r); return
            }
            switch r.URL.Path {
            case "/", "/health", "/chat", "/api/auth/signup", "/api/auth/login":
                next.ServeHTTP(w, r); return
            }
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" || len(authHeader) <= 7 || authHeader[:7] != "Bearer " {
                writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid token"})
                return
            }
            username, err := a.ValidateToken(authHeader[7:])
            if err != nil {
                writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
                return
            }
            ctx := context.WithValue(r.Context(), auth.UserKey, username)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

- OPTIONS requests are always allowed (CORS preflight).
- Public paths bypassed: `/`, `/health`, `/chat`, `/api/auth/signup`, `/api/auth/login`.
- Token format: `Authorization: Bearer <token>`.
- On success, username is injected into request context.
- Not applied when `a` is nil (auth disabled).

### 8.3 Routes (`routes.go`)

#### Route Registration

The `Start()` method registers all routes and applies middleware in order (outermost ? innermost): `loggingMiddleware` ? `corsMiddleware` ? `authMiddleware`.

**Complete route table:**

| Method | Path | Handler | Auth Required |
|--------|------|---------|---------------|
| GET | `/` | DashboardHandler.HandleDashboard | No |
| GET | `/chat` | DashboardHandler.HandleChatPage | No |
| GET | `/health` | Inline (health check) | No |
| POST | `/api/auth/signup` | AuthHandler.HandleSignup | No |
| POST | `/api/auth/login` | AuthHandler.HandleLogin | No |
| GET | `/api/servers` | AdminHandler.HandleListServers | Yes |
| GET | `/api/tools` | AdminHandler.HandleListTools | Yes |
| GET | `/api/logs` | AdminHandler.HandleLogs | Yes |
| GET | `/api/stats` | AdminHandler.HandleStats | Yes |
| POST | `/mcp/message` | MCPHandler.HandleMCPMessage | Yes |
| POST | `/api/chat` | ChatHandler.HandleChat | Yes |
| GET | `/api/chat/sessions` | ChatHandler.HandleListSessions | Yes |
| POST | `/api/chat/sessions` | ChatHandler.HandleCreateSession | Yes |
| DELETE | `/api/chat/sessions/{id}` | ChatHandler.HandleDeleteSession | Yes |
| GET | `/api/chat/sessions/{id}/messages` | ChatHandler.HandleGetMessages | Yes |
| POST | `/api/upload` | UploadHandler.HandleFileUpload | Yes |

## 9. Handlers Deep Dive

Each handler type lives in `internal/web/handlers/` and follows the same pattern: a struct holding dependencies, and methods that implement `http.HandlerFunc`.

### 9.1 AuthHandler (`auth.go`)

```go
type AuthHandler struct {
    Auth    *auth.Auth
    Limiter interface{ Allow(string) bool }
}
```

The `Limiter` is an interface type for testability — any rate limiter implementing `Allow(ip string) bool` can be injected.

#### HandleSignup

```go
func (h *AuthHandler) HandleSignup(w http.ResponseWriter, r *http.Request) {
    if h.Auth == nil {
        writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth not configured"})
        return
    }
    if !h.Limiter.Allow(clientIP(r)) {
        writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests"})
        return
    }
    var req struct {
        Username string `json:"username"`
        Email    string `json:"email"`
        Password string `json:"password"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Email == "" || req.Password == "" {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username, email, password required"})
        return
    }
    token, err := h.Auth.Signup(req.Username, req.Email, req.Password)
    if err != nil {
        writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
        return
    }
    writeJSON(w, http.StatusCreated, map[string]any{"token": token, "username": req.Username})
}
```

Response (201): `{ "token": "eyJ...", "username": "johndoe" }`

#### HandleLogin

```go
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
    // Same pattern: check auth ? rate limit ? parse ? login ? respond
    var req struct { Username string; Password string }
    // ...validation...
    token, err := h.Auth.Login(req.Username, req.Password)
    if err != nil {
        writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"token": token, "username": req.Username})
}
```

### 9.2 ChatHandler (`chat.go`)

The ChatHandler is the most complex handler. It coordinates the full chat lifecycle.

```go
type ChatHandler struct {
    Gateway    *mcp.Gateway
    Logger     *common.Logger
    Brain      *ai.Manager
    Auth       *auth.Auth
    ChatStore  *storage.ChatStore
    MemHistory map[string][]common.ChatMessage
    mu         sync.RWMutex
}
```

#### HandleChat — Full Lifecycle

1. **Check AI configured** — returns 503 if `Brain == nil`.
2. **Parse request** — validates message (=10000 chars) and session_id.
3. **Load history** — from MongoDB or in-memory fallback.
4. **Validate session ownership** — ensures user owns the session.
5. **Save user message** — persists to ChatStore.
6. **Auto-title** — if session title is "New Chat", uses first 50 chars of message.
7. **Build LLM history** — converts to `[]map[string]string` format.
8. **Define callTool function** — wraps Gateway.ForwardToolCall with MCP request construction and text extraction.
9. **Run AI agent** — calls `Brain.RunAgent` with the callback.
10. **Track tools used** — logs each tool call to both in-memory logger and MongoDB.
11. **Save to memory** — stores in AI MemoryStore for future relevance.
12. **Save AI response** — persists to ChatStore (or in-memory).
13. **Log request** — logs to both in-memory logger and MongoDB.
14. **Return JSON response** — `{ answer, steps, tools_used, latency_ms }`.

**In-memory fallback behavior:**
- When MongoDB is unavailable, sessions are stored in `map[string][]ChatMessage`.
- Max 20 messages per session in memory.
- Session IDs are generated as `local-<timestamp>`.
- Sessions are not persisted across restarts.

#### extractToolText Helper

```go
func extractToolText(response any) string {
    respMap, ok := response.(map[string]any)
    if !ok { return "" }
    result, ok := respMap["result"].(map[string]any)
    if !ok { return "" }
    content, ok := result["content"].([]any)
    if !ok { return "" }
    var text string
    for _, c := range content {
        if cm, ok := c.(map[string]any); ok {
            if t, ok := cm["text"].(string); ok {
                text += t
            }
        }
    }
    return text
}
```

Navigates the nested JSON-RPC response: `response ? result ? content[] ? { type: "text", text: "..." }`.

### 9.3 AdminHandler (`admin.go`)

```go
type AdminHandler struct {
    Gateway *mcp.Gateway
    Logger  *common.Logger
    Auth    *auth.Auth
}
```

Simple handlers that delegate to Gateway/Logger/Auth:
- `HandleListServers` — `ListServers()` from registry.
- `HandleListTools` — `ListTools()` from registry.
- `HandleLogs` — recent logs from MongoDB or in-memory.
- `HandleStats` — aggregated stats from MongoDB or in-memory.

### 9.4 MCPHandler (`mcp.go`)

```go
type MCPHandler struct {
    Gateway *mcp.Gateway
    Logger  *common.Logger
    Auth    *auth.Auth
}

func (h *MCPHandler) HandleMCPMessage(w http.ResponseWriter, r *http.Request) {
    // Parse MCP JSON-RPC request
    // Route based on method:
    //   "tools/list" ? return all tools from registry
    //   "tools/call" ? forward to appropriate server via Gateway.ForwardToolCall
    //   other ? 400 "unsupported method"
}
```

Provides raw MCP passthrough — clients can call `POST /mcp/message` with any valid MCP JSON-RPC request.

### 9.5 UploadHandler (`upload.go`)

```go
type UploadHandler struct {
    Gateway *mcp.Gateway
}

func (h *UploadHandler) HandleFileUpload(w http.ResponseWriter, r *http.Request) {
    server, err := h.Gateway.Registry().GetServer("documents")
    if err != nil { /* 502 */ }
    // Reverse proxy to documents server /upload endpoint
    proxyReq, err := http.NewRequest("POST", server.URL+"/upload", r.Body)
    // Copy Content-Type, Content-Length, Accept headers
    // 60-second timeout
    // Stream response back
}
```

### 9.6 DashboardHandler (`dashboard.go`)

```go
//go:embed dashboard.html
var dashboardFS embed.FS

//go:embed chatui.html
var chatUIFS embed.FS

var dashboardHTML string
var chatPageHTML string

func init() {
    data, _ := dashboardFS.ReadFile("dashboard.html")
    dashboardHTML = string(data)
    data, _ = chatUIFS.ReadFile("chatui.html")
    chatPageHTML = string(data)
}
```

Uses Go's `//go:embed` to embed HTML files into the binary at compile time.

---

## 10. MCP Servers

Each MCP server is an independent HTTP server implementing the MCP JSON-RPC protocol. They all follow the same pattern:
1. Listen on a unique port.
2. Serve `POST /mcp/message` for MCP requests.
3. Serve `GET /health` for health checks.
4. Implement `initialize`, `tools/list`, and `tools/call` methods.
5. Return results in MCP JSON-RPC 2.0 format.

### 10.1 Common MCP Types (`servers/internal/mcpcommon/types.go`)

```go
type MCPRequest struct {
    JSONRPC string         `json:"jsonrpc"`
    ID      any            `json:"id"`
    Method  string         `json:"method"`
    Params  map[string]any `json:"params,omitempty"`
}

type MCPResponse struct {
    JSONRPC string `json:"jsonrpc"`
    ID      any    `json:"id"`
    Result  any    `json:"result,omitempty"`
    Error   any    `json:"error,omitempty"`
}

func SendResult(w *json.Encoder, id any, result any) {
    w.Encode(MCPResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func SendError(w *json.Encoder, id any, code int, msg string) {
    w.Encode(MCPResponse{JSONRPC: "2.0", ID: id, Error: map[string]any{"code": code, "message": msg}})
}

func SendToolResult(w *json.Encoder, id any, text string, isError bool) {
    SendResult(w, id, map[string]any{
        "content": []map[string]any{{"type": "text", "text": text}},
        "isError": isError,
    })
}
```

The MCP JSON-RPC format wraps tool results in:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [{ "type": "text", "text": "Weather in Tokyo: 22°C..." }],
    "isError": false
  }
}
```

### 10.2 Weather Server (`servers/weather/server.go`)

**Port:** 3001
**API:** [wttr.in](https://wttr.in) (free, no API key)

**Tools:** `get_weather` (current conditions), `get_forecast` (3-day)

**Data structure:**
```go
type wttrData struct {
    CurrentCondition []struct {
        TempC, TempF, Humidity, WindspeedK, FeelsLikeC string
        Desc []struct{ Value string } `json:"weatherDesc"`
    } `json:"current_condition"`
    Weather []struct {
        Date string `json:"date"`
        MaxTempC, MinTempC string
        Hourly []struct {
            TempC string
            Desc  []struct{ Value string } `json:"weatherDesc"`
        } `json:"hourly"`
    } `json:"weather"`
}
```

**Output format:**
```
Weather in Tokyo:
  Temp: 22°C
  Condition: Partly cloudy
  Humidity: 65%
  Wind: 12 km/h
```

### 10.3 GitHub Server (`servers/github/server.go`)

**Port:** 3003
**API:** GitHub REST API (https://api.github.com)
**Auth:** Optional `GITHUB_TOKEN` env var

**Tools:** `get_user` (profile), `list_repos` (by stars), `get_repo` (details)

**GitHub API helper:**
```go
func githubAPI(path string) ([]byte, error) {
    req, _ := http.NewRequest("GET", "https://api.github.com"+path, nil)
    req.Header.Set("Accept", "application/vnd.github.v3+json")
    req.Header.Set("User-Agent", "mcp-gateway")
    if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
    resp, err := httpClient.Do(req)
    if resp.StatusCode == 404 { return nil, fmt.Errorf("not found") }
    if resp.StatusCode != 200 { return nil, fmt.Errorf("GitHub API status %d", resp.StatusCode) }
    return body, nil
}
```

**get_user output:** GitHub user profile with bio, location, repos, followers.
**list_repos output:** Top 10 repos sorted by stars, with description and language.

### 10.4 Crypto Server (`servers/crypto/server.go`)

**Port:** 3004
**API:** CoinGecko API (https://api.coingecko.com)

**Tools:** `get_crypto_price` (live price + 24h change), `get_top_cryptos` (top 10 by market cap)

**getPrice output:**
```
Bitcoin Price:
  USD: $67,234.00
  INR: Rs.5,612,345.00
  24h: +2.34% (up)
  Market Cap: $1,320,000,000,000
```

### 10.5 Search Server (`servers/search/server.go`)

**Port:** 3007
**APIs:** DuckDuckGo Instant Answer + Wikipedia REST API

**Tools:** `web_search` (DuckDuckGo), `wikipedia_summary` (Wikipedia)

**DuckDuckGo:** Uses the DuckDuckGo Instant Answer API. Prioritizes: `Answer` > `Abstract` > `RelatedTopics`.
**Wikipedia:** Uses `/api/rest_v1/page/summary/`. Truncates extract to 500 characters.

### 10.6 News Server (`servers/news/server.go`)

**Port:** 3005
**API:** Google News RSS feeds (no API key required)

**Tools:** `get_top_news` (by topic), `search_news` (by keyword)

**RSS Feeds:** Predefined feeds for general, technology, business, sports, science, health.

**RSS Parsing:**
```go
type RSS struct {
    Channel struct {
        Items []struct { Title, Link, PubDate, Source string } `xml:"item"`
    } `xml:"channel"`
}
```

Returns top 8 headlines as a numbered list.

### 10.7 URL Server (`servers/url/server.go`)

**Port:** 3006
**APIs:** is.gd URL shortener, goqr.me QR code generator

**Tools:** `shorten_url` (is.gd), `generate_qr` (QR code image URL), `expand_url` (follow redirect)

**shorten_url:** Calls `https://is.gd/create.php?format=simple&url=...`
**generate_qr:** Returns an image URL from `https://api.qrserver.com/v1/create-qr-code/`
**expand_url:** Follows redirects using HTTP HEAD with `CheckRedirect` set to `http.ErrUseLastResponse`.

### 10.8 Notes Server (`servers/notes/server.go`)

**Port:** 3002
**Database:** SQLite (via `modernc.org/sqlite`)

**Tools:** `add_note`, `list_notes`, `search_notes`

**Schema:**
```sql
CREATE TABLE IF NOT EXISTS notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    tags TEXT DEFAULT '',
    username TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**Key behaviors:**
- Notes are scoped by `username` (from `_user` parameter).
- `list_notes` returns in reverse chronological order.
- `search_notes` uses `LIKE '%query%'` across title, content, and tags.
- Proper `Close()` for graceful SQLite shutdown.

---

## 11. Frontend

The frontend consists of two HTML files embedded via `//go:embed`.

### 11.1 Dashboard (`dashboard.html`)

**Route:** `GET /`

The admin dashboard shows real-time system status with four metric cards and three tables (servers, tools, logs). It auto-refreshes every 5 seconds.

**JS Logic:**
```javascript
async function refresh() {
    const [servers, tools, stats] = await Promise.all([
        fetch('/api/servers').then(r=>r.json()),
        fetch('/api/tools').then(r=>r.json()),
        fetch('/api/stats').then(r=>r.json())
    ]);
    // Update count cards
    // Render server table with status badges
    // Render tool table
    // Fetch and render logs
}
refresh();
setInterval(refresh, 5000);
```

### 11.2 Chat UI (`chatui.html`)

**Route:** `GET /chat`

A ChatGPT-like chat interface with capability suggestion buttons.

**Key features:**
- Session IDs generated client-side (`local-<timestamp>`).
- Typing indicator with bouncing dots animation.
- Tool badges showing which tools were used.
- Latency display in milliseconds.
- Capability buttons for quick prompts ("Weather in Tokyo", "Bitcoin price now", etc.).

**JS Logic:**
```javascript
let sessionId = 'local-' + Date.now();
async function sendMessage() {
    const msg = document.getElementById('user-input').value.trim();
    addMessage(msg, 'user');
    // Show typing indicator
    const resp = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: msg, session_id: sessionId })
    });
    const data = await resp.json();
    addMessage(data.answer || data.error, 'ai', { tools: data.tools_used, latency: data.latency });
    // Hide typing indicator
}
```

### 11.3 Communication

- Dashboard fetches from Admin API endpoints (no auth token since dashboard page is public).
- Chat UI calls `POST /api/chat` without auth header (chat page is public).
- Auth tokens are not handled in the frontend HTML pages — these are designed for development/demo use.

## 12. Configuration

### 12.1 `config.yaml` Format

```yaml
gateway:
  port: 8080
  name: "MCP Gateway"

mongodb:
  uri: ""
  database: "mcp_gateway"

servers:
  - name: "weather"
    url: "http://localhost:3001"
    enabled: true

  - name: "notes"
    url: "http://localhost:3002"
    enabled: true

  - name: "github"
    url: "http://localhost:3003"
    enabled: true

  - name: "crypto"
    url: "http://localhost:3004"
    enabled: true

  - name: "news"
    url: "http://localhost:3005"
    enabled: true

  - name: "url-tools"
    url: "http://localhost:3006"
    enabled: true

  - name: "search"
    url: "http://localhost:3007"
    enabled: true

  - name: "documents"
    url: "http://localhost:3008"
    enabled: true
```

### 12.2 Config Struct (Go)

```go
type ServerConfig struct {
    Name    string `yaml:"name"`
    URL     string `yaml:"url"`
    Enabled bool   `yaml:"enabled"`
}

type GatewayConfig struct {
    Port int    `yaml:"port"`
    Name string `yaml:"name"`
}

type MongoConfig struct {
    URI      string `yaml:"uri"`
    Database string `yaml:"database"`
}

type Config struct {
    Gateway GatewayConfig   `yaml:"gateway"`
    MongoDB MongoConfig     `yaml:"mongodb"`
    Servers []ServerConfig  `yaml:"servers"`
}
```

### 12.3 Environment Variables

| Variable | Purpose | Default | Required |
|----------|---------|---------|----------|
| `PORT` | HTTP server port | 8080 | No |
| `MONGO_URI` | MongoDB connection string | (from config.yaml) | No |
| `MONGO_DATABASE` | MongoDB database name | (from config.yaml) | No |
| `JWT_SECRET` | HMAC key for JWT signing | None | Yes (if MongoDB enabled) |
| `GROQ_API_KEY` | Groq API key for LLM | None | Yes (if AI enabled) |
| `GROQ_MODELS` | Comma-separated model override | `llama-3.3-70b-versatile,qwen/qwen3-32b,qwen/qwen3.6-27b` | No |
| `ALLOWED_ORIGINS` | Comma-separated CORS origins | `https://mcp-gateway-tvaa.onrender.com` | No |
| `GITHUB_TOKEN` | GitHub API token | None | No |

### 12.4 Config Validation

`common.LoadConfig` performs:
1. File must exist and be valid YAML.
2. `MONGO_URI` / `MONGO_DATABASE` env vars override config file values.
3. Gateway port defaults to 8080 if not specified.
4. All servers must have non-empty names.
5. No duplicate server names.
6. Enabled servers must have a non-empty URL.
7. All URLs must be parseable by `url.ParseRequestURI`.

---

## 13. Dependencies and Build

### 13.1 Go Module (`go.mod`)

```
module github.com/varunbanda/mcp-gateway
go 1.25.5

require (
    github.com/golang-jwt/jwt/v5 v5.3.1
    go.mongodb.org/mongo-driver v1.17.9
    golang.org/x/crypto v0.53.0
    gopkg.in/yaml.v3 v3.0.1
    modernc.org/sqlite v1.37.1
)
```

### 13.2 Key Dependencies

| Dependency | Purpose | Version |
|------------|---------|---------|
| `github.com/golang-jwt/jwt/v5` | JWT creation and validation | v5.3.1 |
| `go.mongodb.org/mongo-driver` | MongoDB client | v1.17.9 |
| `golang.org/x/crypto` | bcrypt password hashing | v0.53.0 |
| `gopkg.in/yaml.v3` | YAML configuration parsing | v3.0.1 |
| `modernc.org/sqlite` | Pure Go SQLite driver (no CGO) | v1.37.1 |

### 13.3 Build and Run

```bash
# Build the server binary
go build -o mcp-gateway ./cmd/server/

# Run directly
go run ./cmd/server/

# Run with environment variables
PORT=9090 MONGO_URI="mongodb://localhost:27017" JWT_SECRET="my-secret" GROQ_API_KEY="gsk_..." go run ./cmd/server/
```

### 13.4 Build Output

The build produces a single binary containing:
- The HTTP server
- All embedded HTML files (dashboard.html, chatui.html)
- All MCP server logic (started in-process as goroutines)
- The SQLite driver (pure Go, no CGO dependency)

---

## 14. Testing Strategy

### 14.1 Current Status

The project currently has **no dedicated test files**. The codebase was built with an architecture-first approach, focusing on clean separation of concerns that will make testing straightforward.

### 14.2 Testing Architecture

Each layer should be tested in isolation:

#### AI Layer (`internal/ai/`)

The `Manager` depends on the Groq API. Tests should mock the HTTP endpoint using `httptest.NewServer`:

```go
func TestManager_DecideAction_NoTool(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(chatResponse{
            Choices: []struct{ Message llmMessage }{
                {Message: llmMessage{Role: "assistant", Content: "Hello!"}},
            },
        })
    }))
    defer server.Close()

    m := New("test-key")
    m.httpClient = server.Client()
    result, err := m.DecideAction(context.Background(), "Hi", nil)
    assert.NoError(t, err)
    assert.False(t, result.NeedsTool)
    assert.Equal(t, "Hello!", result.DirectAnswer)
}
```

#### Gateway Layer (`internal/mcp/`)

Test the registry with a synthetic config, then test forwarding with an httptest server simulating an MCP backend:

```go
func TestRegistry_FindServerByTool(t *testing.T) {
    cfg := &common.Config{
        Servers: []common.ServerConfig{
            {Name: "weather", URL: "http://localhost:3001", Enabled: true},
        },
    }
    r := NewRegistry(cfg)
    r.UpdateStatus("weather", StatusOnline, []Tool{
        {Name: "get_weather", ServerName: "weather"},
    }, 0)
    server, err := r.FindServerByTool("get_weather")
    assert.NoError(t, err)
    assert.Equal(t, "weather", server.Name)
}
```

#### Auth Layer (`internal/auth/`)

Use a test MongoDB instance (or mock the collection interface) to test signup/login/token validation flow.

#### Handlers (`internal/web/handlers/`)

Use `httptest.NewRecorder` and `httptest.NewRequest` with mocked dependencies:

```go
func TestChatHandler_HandleChat_NoAI(t *testing.T) {
    h := &ChatHandler{Brain: nil}
    w := httptest.NewRecorder()
    r := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"message":"hi","session_id":"abc"}`))
    h.HandleChat(w, r)
    assert.Equal(t, 503, w.Code)
}
```

### 14.3 Mocking Strategies

| External Dependency | Mocking Approach |
|---------------------|------------------|
| Groq API | `httptest.NewServer` with mock responses |
| MongoDB | Use `mongodb-memory-server` or mock `*mongo.Collection` |
| GitHub API | `httptest.NewServer` returning fixture data |
| wttr.in | `httptest.NewServer` returning fixture weather data |
| CoinGecko | `httptest.NewServer` returning fixture crypto data |
| DuckDuckGo/Wikipedia | `httptest.NewServer` returning fixture search data |
| Google News RSS | `httptest.NewServer` returning fixture RSS XML |
| is.gd / goqr.me | `httptest.NewServer` returning fixture responses |
| SQLite | In-memory SQLite (`:memory:`) |

---

## 15. Deployment

### 15.1 Production Environment Variables

```bash
# Required
export JWT_SECRET="your-256-bit-secret"
export GROQ_API_KEY="gsk_your-groq-api-key"

# MongoDB
export MONGO_URI="mongodb+srv://user:pass@cluster.mongodb.net/?retryWrites=true&w=majority"
export MONGO_DATABASE="mcp_gateway_prod"

# Optional
export PORT=8080
export ALLOWED_ORIGINS="https://your-domain.com,https://app.your-domain.com"
export GITHUB_TOKEN="ghp_your-github-token"
```

### 15.2 MongoDB Atlas Connection

For production MongoDB Atlas:
1. Create a free tier cluster on atlas.mongodb.com.
2. Set up a database user with read/write permissions.
3. Whitelist the deployment IP (or `0.0.0.0/0` for Render.com).
4. Use the SRV connection string format.

### 15.3 Docker Considerations

The project can be containerized with a multi-stage Docker build:

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /mcp-gateway ./cmd/server/

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /mcp-gateway .
COPY config.yaml .
EXPOSE 8080
CMD ["./mcp-gateway"]
```

Notes for Docker:
- `CGO_ENABLED=0` ensures a fully static binary.
- Alpine base keeps the image small (~20MB).
- `ca-certificates` needed for HTTPS calls to external APIs.
- The config.yaml must be mounted or baked into the image.

### 15.4 Render.com Deployment (Original)

This project was originally deployed on Render.com:
- Web service running the Go binary.
- MongoDB Atlas for the database.
- Environment variables set via Render dashboard.
- Health check endpoint at `/health`.

---

## 16. Common Interview Questions and Answers

### Q1: Why did you refactor the project?

**A:** The original codebase had three separate components — an orchestrator, a planner, and an executor — that created tight coupling, complex data flow, and made debugging difficult. The refactored version merges all AI logic into a single `Manager` struct in `internal/ai/`. All three responsibilities (planning, executing, orchestrating) are now handled within one coherent agent loop (`RunAgent`). This reduces the cognitive load for new developers, eliminates cross-package dependencies, and makes the AI layer a single import point.

**Key changes:**
- Merged orchestrator + planner + executor ? single `ai.Manager`
- Removed `WorkflowStep` and `WorkflowResult` types now using `common.ToolStep`
- Simplified the agent loop from 9 files to 7 files

### Q2: Why did you remove the orchestrator/planner/executor?

**A:** The separation of planner, executor, and orchestrator created an over-engineered abstraction for what is fundamentally a loop: `LLM decides ? tool runs ? LLM synthesizes`. The LLM itself handles planning implicitly through its system prompt and tool definitions. Having explicit Go-side planning logic (the `planner.go` keyword patterns) and execution logic (`tool_executor.go` definitions) within the same package is sufficient. The orchestrator was just a loop that called the planner then the executor — now that loop is the `RunAgent` method.

**Benefits of removal:**
- Reduced code from ~400 lines of orchestration to ~200 lines in `RunAgent`
- Eliminated cross-package type dependencies
- Single entry point for all AI interactions
- Easier to trace and debug agent behavior

### Q3: Why separate MCP servers instead of one monolith?

**A:** Each MCP server is independently deployable, testable, and scalable. By separating:
- **Weather** calls wttr.in (free)
- **GitHub** calls GitHub API (with optional token)
- **Crypto** calls CoinGecko (free tier)
- **News** scrapes Google News RSS
- **Search** calls DuckDuckGo + Wikipedia
- **URL** calls is.gd + goqr.me
- **Notes** runs SQLite

Each server has different rate limits, error handling, and data formats. If one server fails (e.g., CoinGecko rate limit), it doesn't affect others. In production, each server could run in its own container with its own scaling policy.

**Trade-offs:** Increased network latency (HTTP call vs in-process function call) and deployment complexity. For this project, the modularity benefits outweigh the overhead since all servers run in-process as goroutines.

### Q4: How does the agent loop work?

**A:** The agent loop in `RunAgent`:
1. **Plan:** The `planner.CreatePlan()` uses keyword matching to suggest an ordered tool execution plan (e.g., "weather in Tokyo and latest news" ? fetch weather first, then news). This is injected into the system prompt as guidance.
2. **Loop (max 5 steps):** Each iteration sends the conversation history + tool definitions to the LLM. If the LLM returns tool calls, each is executed via the `callTool` callback (which forwards to the gateway). The result is appended to the message history and fed back to the LLM.
3. **Termination:** When the LLM returns a text response (no tool calls), the loop exits with that answer.
4. **Fallback:** If the LLM returns an empty answer after tool calls, the last tool result is used. If max steps are exhausted, a forced summary prompt is sent. If even that fails, all tool results are concatenated.

### Q5: How does model fallback work?

**A:** The `callLLM` method iterates through `m.models` in order:
1. Primary: `llama-3.3-70b-versatile`
2. First fallback: `qwen/qwen3-32b`
3. Second fallback: `qwen/qwen3.6-27b`

For each model, it sends a POST to the Groq API. On success (HTTP 200 + valid response), it returns immediately. On failure, it appends the error and tries the next model. Retryable errors (429, 403, 404, 500+) continue the loop. Non-retryable errors (other 4xx) return immediately. If all models fail, a combined error message is returned.

The model list can be overridden via the `GROQ_MODELS` environment variable (comma-separated).

### Q6: How does memory work?

**A:** The `MemoryStore` is an in-memory ring buffer with a maximum of 200 entries. Each `MemoryEntry` stores the user's query, the AI's answer, the tools used, and a timestamp. When the AI needs to respond to a new query, `QueryRelevant()` tokenizes both the query and all stored entries into words (=3 characters, alphanumeric only), then scores each entry by the number of matching words. The top 3 results are injected into the system prompt prefixed with "Past interaction N: ...".

**Limitations:** No persistence across restarts. No vector search. Keyword matching is simple but effective for this use case. For production scale, a vector database (Pinecone, pgvector) would be appropriate.

### Q7: How does authentication work?

**A:** The `auth` package handles:
1. **Signup:** Validates password (=6 chars), hashes with bcrypt, stores in MongoDB `users` collection with unique indexes on `username` and `email`.
2. **Login:** Finds user by username, compares bcrypt hash, returns JWT (HS256, 7-day expiry) with `sub` claim = username.
3. **Validation:** Middleware parses the `Authorization: Bearer <token>` header, validates the JWT signature and expiry, and injects the username into the request context.

The error message for failed login is intentionally generic (`"invalid credentials"`) to prevent username enumeration.

### Q8: Why no refresh tokens?

**A:** Refresh tokens add significant complexity:
- Need a secure token store (database table)
- Rotation logic (replace refresh token on each use)
- Revocation mechanism (token blacklist)
- Two-step auth flow (access + refresh)

For this project's use case (personal productivity tool, not enterprise SSO), a single long-lived JWT (7 days) is sufficient. Users who need to log out can simply discard their token. For production with compliance requirements (SOC2, HIPAA), refresh tokens should be added.

### Q9: Why no human-in-the-loop approval?

**A:** The approval workflow was removed because:
- Every tool call adds latency (user must click "approve" each time)
- The LLM already validates arguments before calling tools
- For a personal assistant use case, approval interrupts flow
- Tools are read-only or low-risk (weather, news, search, crypto prices)

If this were deployed in a high-risk environment (banking, healthcare), an approval layer could be added as middleware between the ChatHandler and the Gateway without changing the AI layer.

### Q10: How would you add a new tool?

**A:** Adding a new tool requires changes in three layers:

1. **MCP Server** (e.g., `servers/weather/server.go`):
   - Add a new tool definition to the `tools` slice (name, description, inputSchema).
   - Add a new case in the `handleTool` switch for `tools/call`.
   - Implement the business logic function.

2. **AI Layer** (`internal/ai/tool_executor.go`):
   - Add a new `makeToolDef(...)` call in `toolDefs()` with the same name, description, and parameters.
   - Update the `SystemPrompt` in `prompt.go` to include the new capability in TOOL SELECTION RULES.

3. **Configuration** (`config.yaml`):
   - Add the new server entry (if it's a standalone server) or the tool will be auto-discovered by the health checker.

### Q11: How would you scale this?

**A:** Several scaling strategies:
- **Horizontal scaling:** Run multiple gateway instances behind a load balancer. Use MongoDB as the shared session store.
- **Vertical scaling:** Increase instance size for CPU/memory-bound LLM processing.
- **MCP server isolation:** Run each MCP server in its own container with independent scaling policies (e.g., 10 weather server replicas, 2 GitHub replicas).
- **Caching:** Add Redis caching for frequently called tools (weather, crypto prices).
- **Async processing:** Use a message queue (RabbitMQ, Kafka) for non-blocking tool execution.
- **Database:** Add MongoDB replicas for high availability.

### Q12: How would you add tracing/monitoring?

**A:** Use OpenTelemetry for distributed tracing:
1. Add `otel.SetTracerProvider(...)` in `main.go`.
2. Create spans in each layer: handler ? AI agent ? gateway ? MCP server.
3. Export traces to Jaeger, Grafana Tempo, or Datadog.
4. Key spans to instrument: `RunAgent` (entire chat), `callLLM` (each LLM request), `ForwardToolCall` (each tool call), `checkAll` (health check).
5. Add metrics: request count, latency percentiles, error rate, tool usage frequency.
6. Add structured logging (zerolog, zap) with trace IDs.

### Q13: What would you improve next?

**A:** Priority improvements:
1. **Test coverage:** Unit tests for all layers, integration tests for the agent loop.
2. **Graceful HTTP shutdown:** Add `http.Server.Shutdown()` for clean connection draining.
3. **Rate limiting:** Move rate limiting to a middleware for all endpoints, not just auth.
4. **LLM streaming:** Stream responses to the frontend via Server-Sent Events for better UX.
5. **Vector memory:** Replace keyword scoring with embeddings (pgvector, Qdrant) for semantic search.
6. **Configuration validation:** Add JSON Schema validation for config.yaml.
7. **Admin UI:** Polish the dashboard with auth token management and user settings.
8. **Error classification:** Structured error types instead of string messages.

---

## 17. Architecture Decision Records Summary

### ADR 1: Context Propagation as First Parameter

**Decision:** Every request-scoped function accepts `context.Context` as its first parameter.

**Rationale:** Enables request cancellation, deadline propagation, and future tracing integration. Go convention.

### ADR 2: Single AI Entry Point (No Orchestrator)

**Decision:** All AI logic lives in `internal/ai/manager.go` with one public entry point (`RunAgent`).

**Rationale:** Previous separate orchestrator/planner/executor created tight coupling and complex data flow. Merging into one package simplifies the architecture.

### ADR 3: Feature-Based Handler Layout

**Decision:** Each handler type gets its own file under `internal/web/handlers/`.

**Rationale:** Monolithic `server.go` becomes unmaintainable. Feature-based files make it easy to find, modify, and test specific endpoints.

### ADR 4: MCP Registry Pattern

**Decision:** A central `Registry` maps tool names to server URLs, keeping the gateway and forwarder decoupled from server configuration.

**Rationale:** Adding a new server means adding a config entry; the registry auto-discovers tools via health check. No code changes needed in routing logic.

### ADR 5: In-Memory Memory Store

**Decision:** Use an in-memory ring buffer with keyword scoring instead of a vector database.

**Rationale:** Simplicity. For this use case (personal assistant, <200 conversations), a vector database is over-engineering. Can be upgraded later.

### ADR 6: Keyword Scoring vs Embeddings

**Decision:** Simple word-overlap scoring for memory relevance instead of semantic embeddings.

**Rationale:** Fast, no external dependencies, sufficient for keyword-based matching. Embeddings would add latency, cost, and complexity for marginal improvement at this scale.

### ADR 7: No Refresh Tokens

**Decision:** Single long-lived JWT (7 days) without refresh tokens.

**Rationale:** Refresh tokens add complexity (storage, rotation, revocation) without proportional benefit for this project's use case.

### ADR 8: No Human Approval Workflow

**Decision:** Tool calls execute immediately without approval.

**Rationale:** Every approval step adds latency and friction. Tools are read-only or low-risk. Can be added as middleware if needed.

### ADR 9: In-Memory Fallback for Chat

**Decision:** When MongoDB is unavailable, chat history falls back to an in-memory map.

**Rationale:** Graceful degradation. Users can still chat during database outages, though history is lost on restart.

### ADR 10: Embedded HTML via //go:embed

**Decision:** HTML files are compiled into the binary using Go's `//go:embed` directive.

**Rationale:** Single binary deployment — no static file server needed. Simplifies deployment to platforms like Render.com.

---

## 18. Troubleshooting Guide

### "AI not configured" Error

**Symptom:** `POST /api/chat` returns `503 { "error": "AI not configured (set GROQ_API_KEY)" }`

**Cause:** `GROQ_API_KEY` environment variable is not set.

**Solution:** Set the environment variable:
```bash
export GROQ_API_KEY="gsk_your-groq-api-key"
```
Or pass it inline:
```bash
GROQ_API_KEY="gsk_..." go run ./cmd/server/
```

### MongoDB Connection Failures

**Symptom:** Server starts with "WARNING: Auth unavailable" or fails to start if MongoDB is required.

**Cause:** MongoDB URI is empty, unreachable, or authentication credentials are invalid.

**Solutions:**
1. Check `MONGO_URI` environment variable or `mongodb.uri` in config.yaml.
2. Verify MongoDB is running: `mongosh mongodb://localhost:27017`.
3. Check network connectivity and firewall rules.
4. For MongoDB Atlas, ensure the IP whitelist includes your deployment.

### MCP Server Not Responding

**Symptom:** Health checker logs `[weather] OFFLINE` or tool calls return errors.

**Causes:**
1. The MCP server process crashed.
2. Port is already in use.
3. MCP server started on a different port than configured.

**Solutions:**
1. Check server logs for crash messages.
2. Verify port availability: `netstat -ano | findstr :3001`.
3. Check config.yaml for port mismatch.
4. The health checker automatically retries every 10 seconds.

### Rate Limiting Issues

**Symptom:** `POST /api/auth/login` returns `429 { "error": "too many requests" }`

**Cause:** More than 10 requests per minute from the same IP.

**Solution:** Wait 60 seconds before retrying. The rate limiter uses a sliding window, so requests older than 1 minute are automatically deallocated.

### JWT Expiry

**Symptom:** API requests return `401 { "error": "invalid or expired token" }`

**Cause:** JWT token has expired (7-day lifetime).

**Solution:** The client must re-authenticate via `POST /api/auth/login` to obtain a new token. There is no refresh token mechanism.

### Cross-Origin Errors

**Symptom:** Browser console shows CORS errors when accessing from a custom domain.

**Cause:** The request origin is not in the `ALLOWED_ORIGINS` whitelist.

**Solution:** Set the `ALLOWED_ORIGINS` environment variable:
```bash
export ALLOWED_ORIGINS="https://your-domain.com,https://app.your-domain.com"
```

### Model Quota Exceeded

**Symptom:** LLM calls fail with rate limit or quota errors.

**Cause:** Groq API rate limits or usage quotas exceeded.

**Solutions:**
1. The model fallback chain automatically handles rate limits (429 ? next model).
2. Reduce request frequency.
3. Upgrade Groq API tier for higher limits.
4. Override model list with `GROQ_MODELS` to prioritize cheaper/faster models.

### General Debugging Tips

1. Check stdout logs for middleware start/end markers.
2. Enable verbose logging: the health checker logs each server's status on every cycle.
3. Use `/health` endpoint to verify server status.
4. Use `/api/servers` to see all registered servers and their status.
5. Use `/api/tools` to verify tool discovery.
6. Check that all MCP servers are running on their expected ports.
7. Verify config.yaml is in the working directory of the process.

---

> **End of MCP Gateway Repository Bible**
> 
> This document covers ~60 pages when rendered in markdown.
> For questions or contributions, open an issue or PR on the repository.