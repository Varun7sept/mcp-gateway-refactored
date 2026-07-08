# Step 2: The server says "Who are you?"

## What is middleware?

Imagine a **water pipe**. Water (the request) flows through the pipe. Before the water reaches the tap, it passes through **filters** (middleware). Each filter does one thing — one removes dirt, another adds minerals. The water keeps flowing through all of them.

Middleware is code that sits **between** the incoming request and the actual handler. Each piece does one small job, then passes the request to the next.

```
Request → [Logging Middleware] → [CORS Middleware] → [Auth Middleware] → [Chat Handler]
```

## The 3 middleware layers

For our "What's the weather in London?" request, the server does 3 quick checks:

### 1. Logging Middleware — "Write it in the log book"

```go
start := time.Now()
log.Printf("→ POST /api/chat")     // "Someone arrived"
next.ServeHTTP(w, r)               // let them through
log.Printf("← POST /api/chat (2.3s)")  // "They left after 2.3s"
```

**Simple explanation:** Writes in a notebook: "Someone asked about weather at 2:30pm. They left at 2:32pm." This helps you debug later.

### 2. CORS Middleware — "Where are you coming from?"

**Simple explanation:** Checks if the request is coming from an allowed website. If a bad website tries to use our API, the browser blocks it.

**Why it exists:** Browsers have a security rule — a website from `evil.com` should not be able to send requests to your bank's API. CORS is the browser's way of enforcing this.

### 3. Auth/JWT Middleware — "Show me your ID card"

This is the main check. It looks for a `Authorization: Bearer <token>` header in the request.

**What is JWT?** A JWT (JSON Web Token) is like a **digital ID card**. You get it when you log in. It's a long string like:
```
eyJhbGciOiJIUzI1NiIs...
```

**What the middleware does:**
- If the URL is public (like `/`, `/chat`, `/health`) → let everyone through
- If the URL is private (like `/api/chat`) → check for the ID card
  - Has valid ID card? → Let them through, remember who they are
  - No ID card? → Return 401 "Unauthorized"

**Why JWT?** Because HTTP has no memory. Every request is like a stranger walking in. JWT is the ID card that says "I already logged in yesterday, remember me?"

## What happens if auth fails?

```javascript
// Browser receives:
{"error": "missing or invalid token"}
// Shows: "Connection error" (from chatui.html:90)
```

## Difference between Auth/JWT and CORS

| Concept | Checks | Why |
|---------|--------|-----|
| Auth/JWT | WHO are you? | So only registered users can chat |
| CORS | WHERE are you coming from? | So only allowed websites can use our API |
| Middleware | Not a check — it's the PIPE | Keeps each check in its own box |

## Files involved

| File | What it does |
|------|-------------|
| `internal/web/middleware.go:63` | Logging — writes timestamps |
| `internal/web/middleware.go:86` | CORS — checks origin |
| `internal/web/middleware.go:66` | Auth — validates JWT token |
| `internal/web/routes.go:77-82` | Wraps the 3 middleware in order |
