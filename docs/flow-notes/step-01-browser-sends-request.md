# Step 1: Browser sends the HTTP request

**Query:** "What's the weather in London?"

## What happens

1. User types query in the chat input box (`chatui.html:65`)
2. Presses Enter or clicks the → button
3. JavaScript function `sendMessage()` fires (`chatui.html:71`)

## The JavaScript code

```javascript
async function sendMessage() {
    const input = document.getElementById('user-input');
    const msg = input.value.trim();
    if (!msg) return;
    addMessage(msg, 'user');                    // show user bubble
    input.value = '';
    document.getElementById('send-btn').disabled = true;
    document.getElementById('typing').style.display = 'block';  // typing dots

    const resp = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: msg, session_id: sessionId })
    });

    const data = await resp.json();
    const answer = data.error || data.answer || 'No response';
    addMessage(answer, 'ai', data);             // show AI bubble
    // ...
}
```

## The HTTP request on the wire

```
POST http://localhost:8080/api/chat
Content-Type: application/json

{"message":"What's the weather in London?","session_id":"local-1745678901234"}
```

## What the user sees

- The message appears as a user bubble (purple, right-aligned)
- Three purple bouncing dots appear (typing indicator)
- The send button is disabled

---

## Explanation

### Why `session_id` is sent
- The frontend generates a unique `session_id` on page load: `sessionId = 'local-' + Date.now()`
- This tells the server which conversation thread this message belongs to
- If MongoDB is available, the server stores messages per session; otherwise it uses an in-memory fallback
- Without `session_id`, the AI would have no context of previous messages in the same conversation

### Why `POST` and not `GET`
- The request contains a message body — by HTTP convention, `POST` is used for sending data that creates/modifies state
- The chat message creates a record (stored in MongoDB or in-memory) and triggers AI computation
- `GET` has URL length limits and should be idempotent (no side effects)

### Why the typing indicator matters
- The AI agent loop can take 2-10 seconds (multiple LLM calls + tool executions)
- The typing dots give immediate feedback that the request was received
- The send button is disabled to prevent duplicate submissions

### How we know the frontend file
- The HTML is served via `//go:embed chatui.html` in `dashboard.go`
- The route is registered in `routes.go:42`: `mux.HandleFunc("GET /chat", dh.HandleChatPage)`
- This means the Go server compiles the HTML into the binary — no separate frontend serving needed

---

## What happens next

The browser waits for the HTTP response. The request is now on its way to the Go server. The next step is **Step 2: Middleware processes the request** — logging, CORS, and JWT validation.

## Files involved

| File | Role |
|------|------|
| `internal/web/handlers/chatui.html` | Frontend HTML + JavaScript |
| `internal/web/handlers/dashboard.go` | Embeds and serves the HTML |
| `internal/web/routes.go` | Registers the `/chat` route |
