# Explain Memory

## What it is

The Memory Store (`internal/ai/memory.go`) provides short-term conversation memory for the AI. It stores recent query-answer pairs and retrieves the most relevant ones when processing new requests.

## Implementation

```go
type MemoryEntry struct {
    Query     string
    Answer    string
    ToolsUsed []string
    Timestamp time.Time
}

type MemoryStore struct {
    mu      sync.RWMutex
    entries []MemoryEntry
    maxSize int  // default 200
}
```

It's entirely in-memory — no database, no persistence. Data is lost on restart.

## Storage

Entries are stored in a ring buffer (max 200). When full, the oldest entries are discarded.

## Retrieval

When `DecideAction` or `RunAgent` is called:
1. The user message is tokenised into words (3+ characters, alphanumeric only)
2. Each stored entry is scored by counting case-insensitive word overlaps between query + answer
3. Top 3 entries (by score) are injected into the system prompt

## Scoring algorithm

```
queryWords = tokenize(userMessage)
for each entry:
    entryWords = tokenize(entry.Query + " " + entry.Answer)
    score = countMatches(queryWords, entryWords, caseInsensitive)
```

## Why keyword scoring instead of embeddings?

- Zero dependencies (no vector DB, no embedding model)
- Fast — O(n * m) where n = entries, m = words per entry
- Good enough for a conversation window of 200 entries
- Simple to reason about and debug

## When is memory saved?

```go
h.Brain.Memory().Save(ai.MemoryEntry{
    Query: req.Message,
    Answer: answer,
    ToolsUsed: toolsUsed,
})
```

This happens in `chat.go` after every successful `RunAgent` call.
