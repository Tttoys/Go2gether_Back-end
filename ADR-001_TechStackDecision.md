# ADR-001: Tech Stack Decision

## Context
The **Go2gether** project is a group-travel planning platform that helps users coordinate trips,
align schedules, and manage budgets securely.

The system requires:
- A responsive web frontend for creating and managing trips.
- A backend API for authentication, data storage, and secure communication.
- Integration with **Google OAuth2** for sign-in.
- A reliable database service for user and trip data.

To meet these goals within the sprint timeline (Sprint 2: Authentication Setup),
the team evaluated several web technologies based on performance, learning curve,
and compatibility with OAuth and Supabase.

---

## Decision

### 🖥️ Frontend
- **Framework:** React (with Vite + TypeScript)
- **UI Styling:** TailwindCSS  
- **Routing:** React Router DOM  
- **Build Tool:** Vite  
- **API Communication:** Fetch API (`credentials: include`)  
- **Reasoning:**
  - Vite offers fast build/start times and good DX.
  - React’s component model makes it easy to reuse UI logic.
  - Compatible with Google OAuth flow (redirect & callback handling).
  - TailwindCSS provides consistent, responsive styling.

### ⚙️ Backend
- **Language:** Go (Golang)
- **Framework/Router:** `net/http` with `github.com/gorilla/mux`
- **Authentication:** `golang.org/x/oauth2` + `golang.org/x/oauth2/google`
- **Session Management:** JWT (`github.com/golang-jwt/jwt/v5`)
- **Database:** Supabase (PostgreSQL)
- **Environment Management:** `github.com/joho/godotenv`
- **Reasoning:**
  - Go is performant, concurrent, and simple to deploy.
  - Using built-in `net/http` keeps the server lightweight.
  - OAuth2 libraries integrate cleanly with Google sign-in.
  - Supabase provides managed PostgreSQL, authentication hooks,
    and easy integration with REST endpoints.

### ☁️ Deployment & Configuration
- Local development via `.env` files for both client and server.  
- Backend and DB can later be deployed to **Render** or **Supabase Cloud**.

---

## Alternatives Considered
| Option | Why Not Selected |
|--------|------------------|
| **Node.js + Express** | Familiar but heavier; Go chosen for speed & static typing. |
| **Firebase Auth + Firestore** | Simpler setup but less SQL flexibility; Supabase integrates better with Go. |
| **Gin Framework (Go)** | More features than needed for early sprint; `net/http` is lighter and easier to control. |

---

## Consequences
✅ **Fast development** with small, focused stack  
✅ **High performance & scalability** via Go and PostgreSQL  
✅ **Simple OAuth integration** using standard Google APIs  
⚠️ **Manual SQL management** (no ORM) requires careful query testing  
⚠️ **JWT storage** on frontend must be handled securely  

---

## Status
**Accepted — 1 October 2025**  
Used in **Sprint 2 (Authentication Setup)** and will remain the base architecture
for future sprints (Trip Management, Availability, and Budget modules).

---

## References
- [Go2gether Frontend Repository]([https://github.com/pprimrs/Go2gether_Front-end.git](https://github.com/pprimrs/Go2gether_Front-end.git))  
- [Go2gether Backend Repository]([https://github.com/kmutt-cpe334/Go2gether_Back-end](https://github.com/Tttoys/Go2gether_Back-end.git))  
- [Supabase Docs](https://supabase.com/docs)  
- [Google OAuth2 for Web](https://developers.google.com/identity/protocols/oauth2)
