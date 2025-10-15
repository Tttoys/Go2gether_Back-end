# ADR-002: Authentication Approach

## Context
The **Go2gether** platform requires secure user authentication to allow access to
personalized travel features such as trip creation, availability marking, and budget planning.

The authentication system must:
- Support **Google Sign-In** for convenience and security.
- Provide **JWT-based sessions** for stateless API communication.
- Protect sensitive endpoints such as `/api/auth/profile` and `/api/trips`.
- Integrate with the existing **Supabase PostgreSQL** database for user data.

---

## Decision

### 🔐 Authentication Type
We adopted **Google OAuth2** for authentication.
- Frontend triggers the Google login flow via `/api/auth/google`.
- Backend handles OAuth callback from Google and exchanges the `code` for tokens.
- A **JWT token** is then generated and sent back to the frontend.
- Subsequent API calls require this token in the `Authorization: Bearer <token>` header.

### ⚙️ Session Strategy
- **Type:** JWT (JSON Web Token)
- **Storage:** LocalStorage (frontend)
- **Lifetime:** 24 hours
- **Verification:** Every protected route uses `AuthMiddleware` to decode and verify the JWT.
- Stateless design — no session is stored server-side.

### 🧾 User Storage
- User records are stored in **Supabase PostgreSQL** table `users`.
- When a user logs in via Google for the first time, a new record is inserted.
- If the user already exists, their data (name/email/picture) is updated.

### 🌐 Redirect URIs
| Environment | URI |
|--------------|-----|
| **Frontend (local)** | `http://localhost:5173/auth/callback` |
| **Backend (local)** | `http://localhost:8080/api/auth/google/callback` |
| **Production (planned)** | `https://go2gether.app/api/auth/google/callback` |

### 🧰 Frameworks & Libraries
- **Backend:**  
  - `golang.org/x/oauth2` + `golang.org/x/oauth2/google` (Google OAuth client)  
  - `github.com/golang-jwt/jwt/v5` (JWT generation and verification)  
  - `github.com/joho/godotenv` (environment config loader)  
- **Frontend:**  
  - React + Vite + Fetch API  
  - `useEffect` + `useNavigate` (handle OAuth redirects)

---

## Flow Diagram (Simplified)

1️⃣ User clicks **“Sign in with Google”** on frontend  
2️⃣ Redirects to **backend `/api/auth/google`**  
3️⃣ Backend → Google consent page  
4️⃣ Google → sends callback to `/api/auth/google/callback`  
5️⃣ Backend verifies token, creates/updates user in DB  
6️⃣ Backend issues **JWT** → returns to frontend  
7️⃣ Frontend stores token → all future API calls include it in headers  
8️⃣ Protected routes validate token via middleware before granting access

---

## Alternatives Considered
| Option | Reason Not Chosen |
|--------|-------------------|
| Cookie-based sessions | Stateful and harder to scale across instances. |
| Firebase Authentication | Limited control over token claims and DB structure. |
| Custom email/password only | We prefer Google OAuth for ease of onboarding and security. |

---

## Consequences
✅ Stateless, scalable authentication via JWT  
✅ Simplified login with Google OAuth2  
✅ Seamless frontend–backend integration  
⚠️ Requires secure handling of JWT in frontend (avoid XSS)  
⚠️ OAuth credentials must be protected in `.env` files  

---

## Status
**Accepted — 5 October 2025**  
Implemented in **Sprint 2 (Authentication Setup)**  
To be extended in Sprint 3 for role-based authorization (RBAC).

---

## References
- [Google OAuth 2.0 Documentation](https://developers.google.com/identity/protocols/oauth2)  
- [JWT.io Introduction](https://jwt.io/introduction)  
- [Supabase Auth Docs](https://supabase.com/docs/guides/auth)
