# sequence_login

``mermaid
sequenceDiagram
    participant B as Browser
    participant S as HTTP Server
    participant AH as AuthHandler
    participant RL as RateLimiter
    participant A as Auth
    participant DB as MongoDB

    B->>S: POST /api/auth/login
    S->>AH: HandleLogin
    AH->>AH: Parse {username, password}

    AH->>RL: Allow(ip)
    RL-->>AH: true/false

    alt Rate Limited
        AH-->>B: 429 Too Many Requests
    end

    AH->>A: Login(username, password)
    A->>DB: FindOne(users, {username})
    DB-->>A: User{Username, Email, PasswordHash}

    alt User not found
        A-->>AH: error
        AH-->>B: 401 Unauthorized
    end

    A->>A: bcrypt.CompareHashAndPassword(hash, password)

    alt Password mismatch
        A-->>AH: error
        AH-->>B: 401 Unauthorized
    end

    A->>A: jwt.NewWithClaims(HS256, {sub, iat, exp: 7d})
    A->>A: token.SignedString(jwtSecret)
    A-->>AH: token string

    AH->>A: LogRequest(username, "login", ...)

    AH-->>B: 200 {token, username, email}

``
