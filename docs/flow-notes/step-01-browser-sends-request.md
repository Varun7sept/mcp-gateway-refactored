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

## What happens next

The browser waits for the HTTP response. The request is now on its way to the Go server.
