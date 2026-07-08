# Explain JWT Authentication

## What it is

The auth package (`internal/auth/auth.go`) handles user registration (signup), login, and JWT-based request authentication using MongoDB for persistence.

## Signup Flow

```
POST /api/auth/signup {username, email, password}
1. Validate required fields
2. bcrypt.GenerateFromPassword(password, cost=10)
3. Insert into MongoDB `users` collection
4. Return 201 Created
```

## Login Flow

```
POST /api/auth/login {username, password}
1. Rate limiter check (10 attempts per minute per IP)
2. Find user by username in MongoDB
3. bcrypt.CompareHashAndPassword(storedHash, password)
4. Generate JWT:
   - Algorithm: HS256
   - Claims: {sub: username, iat: now, exp: now + 7 days}
   - Secret: from JWT_SECRET env or auto-generated
5. Log the request
6. Return 200 {token, username, email}
```

## Request Validation

```
Middleware:
1. Skip validation for /, /health, /chat, /api/auth/*
2. Extract "Bearer <token>" from Authorization header
3. jwt.Parse(token, keyFunc) → validate signature + expiry
4. Inject username into request context
5. Pass to next handler
```

## Why no refresh tokens?

The original project had refresh tokens. We removed them because:
- JWT expiry is 7 days (long enough for a chatbot)
- No sensitive data (financial, medical) is involved
- Simpler: no refresh endpoint, no token rotation logic
- If security requirements change, refresh tokens can be added back

## Security considerations

- Passwords are bcrypt-hashed (cost 10)
- JWT secret should be set via `JWT_SECRET` env var in production
- Rate limiting on login prevents brute-force attacks
- MongoDB connection string supports TLS
