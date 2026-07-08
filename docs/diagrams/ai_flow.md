# ai_flow

``mermaid
graph TB
    CHAT[Chat Handler] -->|RunAgent| MAN[Manager]

    subgraph "AI Manager (manager.go)"
        MAN -->|1. CreatePlan| PL[Planner<br/>planner.go]
        PL -->|plan steps| MAN

        MAN -->|2. Build Prompt| PB[Prompt Builder<br/>prompt.go]
        PB -->|system prompt| MAN

        MAN -->|3. Query Memory| MEM[Memory Store<br/>memory.go]
        MEM -->|relevant entries| MAN

        MAN -->|4. Agent Loop| LLM_CLIENT[LLM Client<br/>llm.go]
        LLM_CLIENT -->|HTTP POST| GROQ[Groq API]

        MAN -->|5. Tool Execution| TE[Tool Executor<br/>tool_executor.go]
        TE -->|tool definitions| LLM_CLIENT
    end

    MAN -->|tool call| GATEWAY[MCP Gateway<br/>internal/mcp/]

    subgraph "Agent Loop Logic"
        direction LR
        S1[LLM decides] --> S2{Tool call?}
        S2 -->|Yes| S3[Execute via Gateway]
        S2 -->|No| S4[Return answer]
        S3 --> S1
    end

    GATEWAY --> SRV[MCP Servers]

    MAN -->|save| MEM

``
