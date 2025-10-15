# ADR-002: Authentication Approach

## Context
The **Go2gether** platform provides a collaborative trip-planning system where users can create and manage trips, mark availability, and manage budgets.  
To protect user data and ensure a seamless sign-in experience, the system requires a **secure, multi-method authentication mechanism**.

Our goal is to support:
- **Google OAuth2 Login** for convenience and single-click access.
- **Traditional Email/Password Login and Registration** for manual sign-up users.
- **Forgot/Reset Password** flow for password recovery.
- **JWT (JSON Web Token)** based session management for secure, stateless communication.
- **Protected routes** that only allow access when the JWT is valid.

---

## Decision

### Authentication Methods Implemented
| Method | Description | Purpose |
|--------|--------------|----------|
| **Google OAuth2 Login** | Uses Google OAuth flow to authenticate users via Google account and issue a JWT token. | Fast and secure authentication without password handling. |
| **Email/Password Login** | Validates user credentials stored in Supabase PostgreSQL and issues JWT upon success. | Backup method for users who prefer traditional login. |
| **Register (Sign-Up)** | Creates new user records with hashed passwords. | Allows new users to sign up manually. |
| **Forgot/Reset Password** | Sends email with secure reset token, allowing users to set a new password. | Restores access in case of forgotten password. |
| **Logout** | Clears JWT token from client (frontend). | Ends session securely. |

---

## Authentication Flow Summary

### 1. Google OAuth Login Flow
| Step | Description |
|------|--------------|
| 1️⃣ | User clicks **“Sign in with Google”** on the frontend (`Login.tsx`). |
| 2️⃣ | Frontend requests backend endpoint `/api/auth/google`. |
| 3️⃣ | Backend redirects user to Google consent page. |
| 4️⃣ | Google calls backend callback `/api/auth/google/callback?code=...`. |
| 5️⃣ | Backend exchanges code for access token, retrieves Google user info. |
| 6️⃣ | Backend checks Supabase `users` table; creates or updates record. |
| 7️⃣ | Backend generates JWT and returns it to frontend. |
| 8️⃣ | Frontend stores JWT in localStorage and attaches it in all API calls. |

---

### 2. Email/Password Login Flow
| Step | Description |
|------|--------------|
| 1️⃣ | User enters email and password in login form. |
| 2️⃣ | Frontend sends `POST /api/auth/login` with credentials. |
| 3️⃣ | Backend verifies user and password hash in Supabase DB. |
| 4️⃣ | On success, backend issues JWT → frontend stores in localStorage. |
| 5️⃣ | Subsequent API requests include `Authorization: Bearer <token>`. |

---

### 3. Register (Sign-Up) Flow
| Step | Description |
|------|--------------|
| 1️⃣ | User fills in email, password, and name in registration form. |
| 2️⃣ | Frontend sends `POST /api/auth/register` with user data. |
| 3️⃣ | Backend validates fields, hashes password, inserts into Supabase DB. |
| 4️⃣ | Backend returns confirmation → user can now log in. |

---

### 4. Forgot / Reset Password Flow
| Step | Description |
|------|--------------|
| 1️⃣ | User submits email in “Forgot Password” page. |
| 2️⃣ | Frontend sends `POST /api/auth/forgot-password`. |
| 3️⃣ | Backend generates a secure token and sends reset link via email. |
| 4️⃣ | User clicks link → opens reset password form. |
| 5️⃣ | Frontend sends `POST /api/auth/reset-password` with token + new password. |
| 6️⃣ | Backend validates token, hashes password, and updates DB. |
| 7️⃣ | Backend returns confirmation that password has been changed. |

---

### 5. Logout & Protected Routes
| Step | Description |
|------|--------------|
| 1️⃣ | User clicks **Logout** on frontend. |
| 2️⃣ | Frontend clears JWT from localStorage and calls `/api/auth/logout`. |
| 3️⃣ | Backend confirms logout (optional) or simply returns 200. |
| 4️⃣ | Any further API request without JWT → returns `401 Unauthorized`. |

**Protected routes** such as `/api/auth/profile` use middleware to verify the JWT:  
- If valid → handler executes.  
- If invalid or expired → 401 Unauthorized.

---

## Session Strategy
| Item | Detail |
|------|---------|
| **Type** | JWT (JSON Web Token) |
| **Storage** | Client-side (localStorage) |
| **Lifetime** | 24 hours |
| **Verification** | Via `AuthMiddleware` for every protected route |
| **Design** | Stateless (no session stored on server) |
| **Benefit** | Fast, scalable, and independent of backend memory/state |

---

## User Store
| Item | Detail |
|------|---------|
| **Database** | Supabase (PostgreSQL) |
| **Table** | `users` |
| **Fields** | `id`, `google_id`, `email`, `password_hash`, `name`, `avatar`, `created_at` |
| **Flow** | Google OAuth users and registered users share same table. |
| **Password Hashing** | Bcrypt used for secure password storage. |

---

## Redirect URIs
| Environment | URI |
|--------------|-----|
| **Frontend (local)** | `http://localhost:5173/auth/callback` |
| **Backend (local)** | `http://localhost:8080/api/auth/google/callback` |
| **Production (planned)** | `https://go2gether.app/api/auth/google/callback` |

---

## Frameworks and Libraries
| Layer | Libraries / Tools | Purpose |
|--------|-------------------|----------|
| **Frontend** | React, Vite, TailwindCSS, React Router DOM, Fetch API | UI, routing, and API communication |
| **Backend** | `net/http`, `github.com/gorilla/mux` | Routing and middleware |
|  | `golang.org/x/oauth2`, `golang.org/x/oauth2/google` | Google OAuth 2.0 integration |
|  | `github.com/golang-jwt/jwt/v5` | JWT generation and validation |
|  | `github.com/joho/godotenv` | Load environment variables |
|  | `golang.org/x/crypto/bcrypt` | Password hashing |
| **Database** | Supabase (PostgreSQL) | Store and manage user data |

---

## Error Handling (Summary)
| Scenario | Code | Example Message |
|-----------|------|----------------|
| Missing/Invalid OAuth Code | 400 | `"Invalid or missing authorization code"` |
| Invalid Email/Password | 401 | `"Incorrect credentials"` |
| Email Already Registered | 409 | `"Email already registered"` |
| Expired/Invalid JWT | 401 | `"Session expired or token invalid"` |
| Expired Reset Token | 400 | `"Invalid or expired reset token"` |
| DB/Server Error | 500 | `"Internal server error"` |

---

## Reasons for This Design
| Criteria | Justification |
|-----------|----------------|
| **Security** | OAuth tokens verified by Google, passwords hashed with bcrypt, JWT signed and time-limited. |
| **Scalability** | Stateless JWT allows horizontal scaling with no in-memory sessions. |
| **User Experience** | Fast login with Google or simple email option. |
| **Maintainability** | Handlers modularized (`AuthHandler`, `GoogleAuthHandler`, `ForgotPasswordHandler`). |
| **Integration** | Works seamlessly with React frontend via Fetch API. |

---

## Alternatives Considered
| Option | Reason Not Chosen |
|--------|------------------|
| Cookie-based session | Requires sticky sessions and more server memory. |
| Firebase Auth | Limited control over custom DB and token claims. |
| OAuth-only system | Users without Google accounts need email option. |

---

## Consequences
✅ **Advantages**
- Complete authentication suite (OAuth + Email + Reset + JWT)  
- Stateless, scalable backend  
- Unified structure for user table and routes  

⚠️ **Considerations**
- JWT must be protected from XSS (stored safely in frontend).  
- Reset tokens must have short TTL and be hashed before storage.  
- OAuth credentials should remain in `.env` and never committed.

---

## Status
**Accepted — 10 October 2025**  
Implemented during **Sprint 2 (Authentication Setup)**.  
Will extend in Sprint 3 to support **Role-Based Access Control (RBAC)**.

---

## References
- [Google OAuth 2.0 for Web](https://developers.google.com/identity/protocols/oauth2)  
- [Supabase Documentation](https://supabase.com/docs)  
- [JWT.io Introduction](https://jwt.io/introduction)  
- [Bcrypt Password Hashing](https://pkg.go.dev/golang.org/x/crypto/bcrypt)
