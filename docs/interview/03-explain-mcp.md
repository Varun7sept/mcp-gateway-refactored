# Explain MCP (Model Context Protocol)

## What is MCP?

MCP is a lightweight JSON-RPC protocol for tool calling. Each MCP server exposes tools via HTTP endpoints. The gateway communicates with servers using the MCP message format.

## JSON-RPC Message Format

```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
        "name": "get_weather",
        "arguments": {"city": "London"}
    }
}
```

Response:

```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "result": {
        "content": [
            {"type": "text", "text": "Temperature: 20°C..."}
        ]
    }
}
```

## MCP Servers

Each server under `servers/` is an independent Go HTTP server:

| Server | Port | Tools |
|--------|------|-------|
| weather | 3001 | get_weather, get_forecast |
| github | 3002 | get_user, list_repos, get_repo |
| crypto | 3003 | get_crypto_price, get_top_cryptos |
| search | 3004 | web_search, wikipedia_summary |
| news | 3005 | get_top_news, search_news |
| url | 3006 | shorten_url, generate_qr |
| notes | 3007 | add_note, list_notes, search_notes |

## Why separate servers?

- **Independent lifecycle**: each can be deployed, scaled, or restarted individually
- **Isolation**: a crash in one server doesn't affect others
- **Technology flexibility**: servers could be written in different languages
- **Testing**: each server can be tested in isolation

## How the gateway routes

```
Registry (built from config.yaml):
  "get_weather"     → weather (localhost:3001)
  "get_user"        → github  (localhost:3002)
  "add_note"        → notes   (localhost:3007)
  ...
```
