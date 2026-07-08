# sequence_upload

``mermaid
sequenceDiagram
    participant B as Browser
    participant MW as Middleware
    participant UH as UploadHandler
    participant GW as Gateway
    participant REG as Registry
    participant SRV as Documents Server

    B->>MW: POST /api/upload (multipart file)
    MW->>MW: Validate JWT
    MW->>UH: HandleFileUpload

    UH->>UH: Parse multipart form
    UH->>UH: Read file content (max 10MB)

    UH->>GW: Registry()
    GW-->>UH: *Registry

    UH->>REG: GetServer("documents")
    REG-->>UH: ConnectedServer{URL}

    UH->>SRV: POST {URL}/mcp/message (upload)
    Note over UH,SRV: Reads existing documents, forwards content

    SRV-->>UH: JSON-RPC Response
    UH-->>B: 200 {success, document_name, text}

``
