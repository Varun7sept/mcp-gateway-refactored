# sequence_chat

``mermaid
sequenceDiagram
    participant B as Browser
    participant MW as Middleware
    participant CH as ChatHandler
    participant CS as ChatStore
    participant M as AI Manager
    participant P as Planner
    participant MEM as Memory
    participant LLM as Groq
    participant GW as Gateway
    participant SRV as MCP Server

    B->>MW: POST /api/chat
    MW->>MW: Validate JWT
    MW->>CH: HandleChat

    CH->>CH: Parse {message, session_id}
    CH->>CH: Validate message length

    CH->>CS: GetSession(ctx, id, user)
    CS-->>CH: Session

    CH->>CS: AddMessage(ctx, id, "user", msg)
    CH->>CS: UpdateSessionTitle(ctx, id, user, title)
    CH->>CS: GetRecentMessages(ctx, id, 10)
    CS-->>CH: []ChatMessage

    CH->>CH: Build llmHistory

    CH->>M: RunAgent(ctx, msg, history, callTool)
    M->>P: CreatePlan(msg)
    P-->>M: []planStep

    M->>MEM: QueryRelevant(msg, 3)
    MEM-->>M: []MemoryEntry

    M->>M: Build system prompt (plan + memories)

    loop Agent Loop (max 5)
        M->>LLM: callLLM(messages, toolDefs)
        LLM-->>M: Response

        alt Tool Call
            M->>GW: ForwardToolCall(ctx, req)
            GW->>SRV: POST /mcp/message
            SRV-->>GW: Result
            GW-->>M: ForwardResult
            M->>M: Append result to messages
        else Direct Answer
            M-->>CH: answer, steps
        end
    end

    M->>MEM: Save(entry)
    CH->>CS: AddMessage(ctx, id, "ai", answer, meta)
    CH-->>B: 200 {answer, steps, tools_used, latency}

``
