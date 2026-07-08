# JWT Token Explained with Actual Example

## Your Understanding is CORRECT

> User signs up/logs in → Gets a JWT token
> Token has 3 parts: header, payload, signature
> Signature is created by server using JWT_SECRET
> When user sends request, server checks signature against JWT_SECRET

**Yes, that is exactly right.**

---

## Real Example

Let's say the server's secret is: `my-secret-key`

### After Login, you get this token:

```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.
eyJzdWIiOiJ2YXJ1biIsImlhdCI6MTc0NTY3ODkwMCwiZXhwIjoxNzQ2MjgzNzAwfQ.
TU0F17RJ9ojYrt7yJzqfFfTb2MAEGFYXxZawbeX6nRw
```

This looks like gibberish, but it's actually 3 parts separated by dots.

---

### Part 1: HEADER (decoded)

```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9
```

When decoded from base64:

```json
{
    "alg": "HS256",
    "typ": "JWT"
}
```

| Field | Meaning |
|-------|---------|
| alg: "HS256" | The algorithm used to sign (HMAC with SHA-256) |
| typ: "JWT" | This is a JWT token |

---

### Part 2: PAYLOAD (decoded)

```
eyJzdWIiOiJ2YXJ1biIsImlhdCI6MTc0NTY3ODkwMCwiZXhwIjoxNzQ2MjgzNzAwfQ
```

When decoded from base64:

```json
{
    "sub": "varun",
    "iat": 1745678900,
    "exp": 1746283700
}
```

| Field | Meaning | Example |
|-------|---------|---------|
| sub: "varun" | **Subject** — the username this token belongs to | varun |
| iat: 1745678900 | **Issued At** — when token was created (Unix timestamp) | July 2, 2025 |
| exp: 1746283700 | **Expiration** — when token expires (Unix timestamp) | July 9, 2025 |

---

### Part 3: SIGNATURE (cannot decode — it's encrypted)

```
TU0F17RJ9ojYrt7yJzqfFfTb2MAEGFYXxZawbeX6nRw
```

This is created by the server using:

```
signature = HS256( header + "." + payload , JWT_SECRET )
```

You **cannot decode** this back into anything meaningful. You can only **verify** it.

---

## How Validation Works (Step by Step)

### When you make a request:

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.
                     eyJzdWIiOiJ2YXJ1biIsImlhdCI6MTc0NTY3ODkwMCwiZXhwIjoxNzQ2MjgzNzAwfQ.
                     TU0F17RJ9ojYrt7yJzqfFfTb2MAEGFYXxZawbeX6nRw
```

### The server does this:

```
Step 1: Split the token by "."

  Part 1 = eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9
  Part 2 = eyJzdWIiOiJ2YXJ1biIsImlhdCI6MTc0NTY3ODkwMCwiZXhwIjoxNzQ2MjgzNzAwfQ
  Part 3 = TU0F17RJ9ojYrt7yJzqfFfTb2MAEGFYXxZawbeX6nRw

Step 2: Take server's secret from memory

  secret = "my-secret-key"

Step 3: Create a fresh signature using Parts 1 and 2

  new_signature = HS256( "Part1.Part2", "my-secret-key" )
                = TU0F17RJ9ojYrt7yJzqfFfTb2MAEGFYXxZawbeX6nRw

Step 4: Compare new_signature with Part 3

  Is "TU0F17RJ9ojYrt7yJzqfFfTb2MAEGFYXxZawbeX6nRw"
  == "TU0F17RJ9ojYrt7yJzqfFfTb2MAEGFYXxZawbeX6nRw" ?

  YES → Signature matches → Token is NOT tampered

Step 5: Check expiry

  Is current_time < exp(1746283700)?

  YES → Token is NOT expired

Step 6: Extract username

  sub = "varun"

  Result: "This request is from user: varun"
```

---

## What if someone tampers with the token?

If someone changes the payload to say `"sub": "admin"` and sends:

```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.
eyJzdWIiOiJhZG1pbiIsImlhdCI6MTc0NTY3ODkwMCwiZXhwIjoxNzQ2MjgzNzAwfQ.
TU0F17RJ9ojYrt7yJzqfFfTb2MAEGFYXxZawbeX6nRw
```

Server does Step 3 again:

```
new_signature = HS256( "tampered payload", "my-secret-key" )
             = XyzAbC123...  (DIFFERENT from Part 3)
```

Step 4 comparison:

```
Is "XyzAbC123..." == "TU0F17RJ9ojYrt7yJzqfFfTb2MAEGFYXxZawbeX6nRw" ?
NO  → REJECTED (401 Unauthorized)
```

**The hacker doesn't know the JWT_SECRET** so they cannot create a valid signature for the tampered payload. The token is rejected.

---

## Summary

| Concept | Simple Explanation |
|---------|-------------------|
| Header | Says "I use HS256 algorithm" |
| Payload | Says "I am varun, created at this time, expires at this time" |
| Signature | A lock that can only be created/verified by someone who knows the JWT_SECRET |
| JWT_SECRET | The master password stored only on the server |
| Validation | Server re-creates the signature using its secret and compares |
