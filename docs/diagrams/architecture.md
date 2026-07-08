# architecture

``mermaid
graph TB
    subgraph External
        Browser[Browser]
        LLM[Groq API]
        MCP_Servers[MCP Backend Servers]
    end

    subgraph "MCP Gateway"
        subgraph "internal/web"
            HTTP[HTTP Server]
            MID[Middleware]
            ROUTES[Routes]
            H_AUTH[Auth Handler]
            H_CHAT[Chat Handler]
            H_ADMIN[Admin Handler]
            H_MCP[MCP Handler]
            H_UPLOAD[Upload Handler]
            H_DASH[Dashboard Handler]
        end

        subgraph "internal/ai"
            MANAGER[AI Manager]
            PLANNER[Planner]
            MEMORY[Memory Store]
            PROM[Prompt Builder]
            LLM_CLIENT[LLM Client]
            TOOL_EX[Tool Executor]
        end

        subgraph "internal/mcp"
            REG[Registry]
            FWD[Forwarder]
            HEALTH[Health Checker]
        end

        subgraph "internal/auth"
            AUTH[JWT Auth]
        end

        subgraph "internal/storage"
            CHAT_STORE[Chat Store]
            NOTES[Notes Store]
        end

        subgraph "internal/common"
            CONFIG[Config]
            LOGGER[Logger]
            MODELS[Shared Models]
        end
    end

    Browser --> HTTP
    HTTP --> MID
    MID --> ROUTES
    ROUTES --> H_AUTH
    ROUTES --> H_CHAT
    ROUTES --> H_ADMIN
    ROUTES --> H_MCP
    ROUTES --> H_UPLOAD
    ROUTES --> H_DASH

    H_CHAT --> MANAGER
    MANAGER --> PLANNER
    MANAGER --> MEMORY
    MANAGER --> PROM
    MANAGER --> LLM_CLIENT
    MANAGER --> TOOL_EX
    LLM_CLIENT --> LLM

    H_CHAT --> FWD
    H_MCP --> FWD
    H_UPLOAD --> FWD
    FWD --> REG
    REG --> MCP_Servers

    HEALTH --> REG

    H_AUTH --> AUTH
    MID --> AUTH

    H_CHAT --> CHAT_STORE
    H_ADMIN --> AUTH
    H_ADMIN --> LOGGER

    H_UPLOAD --> MCP_Servers

    CHAT_STORE --> MongoDB[(MongoDB)]
    NOTES --> SQLite[(SQLite)]
    AUTH --> MongoDB

``
