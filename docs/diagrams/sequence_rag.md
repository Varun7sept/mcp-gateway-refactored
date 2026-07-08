# sequence_rag

``mermaid
sequenceDiagram
    participant B as Browser
    participant CH as ChatHandler
    participant M as AI Manager
    participant LLM as Groq
    participant SRV as Documents Server

    B->>CH: POST /api/chat {message: "what does doc.pdf say?"}

    CH->>M: RunAgent(ctx, message, history, callTool)

    M->>M: Regex match: doc.pdf
    M->>M: Force ask_document tool call

    Note over M: Document routing bypasses LLM decision

    M->>SRV: POST /mcp/message
    Note over M,SRV: tools/call {name: "ask_document", arguments: {question, document_name}}

    SRV-->>M: Text content from document
    M->>M: Append to conversation
    M->>M: Add system prompt: "Answer using only the retrieved passages above"

    M->>LLM: callLLM(messages + document text)
    LLM-->>M: Answer based on document

    M-->>CH: answer synthesised from document
    CH-->>B: 200 {answer: "The document says..."}

``
