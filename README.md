# Go2gether Backend

A Go backend API for the Go2gether application with authentication functionality.

## Features

- User registration and login
- JWT-based authentication
- Password hashing with bcrypt
- PostgreSQL database integration
- Health check endpoints

## API Endpoints

### Authentication

- `POST /api/auth/register` - Register a new user
- `POST /api/auth/login` - Login user
- `GET /api/auth/profile` - Get user profile (requires authentication)

### Health Checks

- `GET /healthz` - Basic health check
- `GET /livez` - Process liveness check
- `GET /readyz` - Readiness check (includes database connectivity)

## Setup

1. Install dependencies:
   ```bash
   go mod tidy
   ```

2. Set up your database by running the schema:
   ```bash
   psql -h your-host -U your-username -d your-database -f schema.sql
   ```

3. Create a `.env` file with the following variables:
   ```
   DB_HOST=your-db-host
   DB_PORT=5432
   DB_USER=your-db-username
   DB_PASSWORD=your-db-password
   DB_NAME=your-db-name
   DB_SSLMODE=require
   SERVER_PORT=8080
   JWT_SECRET=your-super-secret-jwt-key-here
   ```

4. Run the application:
   ```bash
   go run cmd/main.go
   ```

## API Usage Examples

### Register User
```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "johndoe",
    "email": "john@example.com",
    "password": "password123"
  }'
```

### Login User
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com",
    "password": "password123"
  }'
```

### Get Profile (with JWT token)
```bash
curl -X GET http://localhost:8080/api/auth/profile \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```
