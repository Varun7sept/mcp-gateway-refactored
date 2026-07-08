# package_web

``mermaid
graph TB
    subgraph "internal/web package"
        S[Server<br/>server.go]
        MW[Middleware<br/>middleware.go]
        R[Routes<br/>routes.go]
    end

    subgraph "internal/web/handlers"
        DH[DashboardHandler<br/>dashboard.go]
        AH[AuthHandler<br/>auth.go]
        ADH[AdminHandler<br/>admin.go]
        MH[MCPHandler<br/>mcp.go]
        CH[ChatHandler<br/>chat.go]
        UH[UploadHandler<br/>upload.go]
        HLPR[Helpers<br/>helpers.go]
    end

    S -->|newRateLimiter| R
    MW -->|loggingMiddleware| R
    MW -->|corsMiddleware| R
    MW -->|authMiddleware| R
    R --- DH
    R --- AH
    R --- ADH
    R --- MH
    R --- CH
    R --- UH
    CH --- HLPR
    AH --- HLPR

    S ---|exported: New, Start| EXT
    R ---|exported: Server, New, Start| EXT

``
