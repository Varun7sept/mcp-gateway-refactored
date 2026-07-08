# memory

``mermaid
graph TB
    subgraph "Memory Store (internal/ai/memory.go)"
        MS[MemoryStore]
        ME[MemoryEntry]
        SAVE[Save()]
        QUERY[QueryRelevant()]
        TOKEN[tokenize()]
    end

    subgraph "Data Flow"
        USER[User Query] -->|RunAgent completes| SAVE
        SAVE -->|append| ENTRIES[(entries array<br/>max 200)]
        ENTRIES -->|ring buffer| ENTRIES

        USER2[New Query] -->|DecideAction| QUERY
        QUERY --> TOKEN
        TOKEN -->|word tokens| SCORE[score vs each entry]
        SCORE -->|sort by score| TOP[Top 3 entries]
        TOP -->|injected into| PROMPT[System Prompt]
    end

    subgraph "MemoryEntry"
        EE1[Query: string]
        EE2[Answer: string]
        EE3[ToolsUsed: []string]
        EE4[Timestamp: time.Time]
    end

    subgraph "Scoring Algorithm"
        TA[Tokenize query â†’ words A]
        TB[Tokenize entry.Query + entry.Answer â†’ words B]
        TC[Count case-insensitive matches]
    end

``
