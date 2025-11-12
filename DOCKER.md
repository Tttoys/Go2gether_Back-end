# Docker Setup Guide

## Prerequisites
- Docker Desktop หรือ Docker Engine
- Docker Compose

## Quick Start

### Production Mode

```bash
# Build and start all services
docker-compose up -d

# View logs
docker-compose logs -f backend

# Stop services
docker-compose down

# Stop and remove volumes (⚠️ จะลบข้อมูล database)
docker-compose down -v
```

### Development Mode

```bash
# Start with development configuration
docker-compose -f docker-compose.dev.yml up

# Or in detached mode
docker-compose -f docker-compose.dev.yml up -d
```

## Services

### Backend API
- **Port**: 8080
- **Health Check**: http://localhost:8080/healthz
- **Swagger**: http://localhost:8080/swagger/index.html

### Database
- **Type**: Supabase (External PostgreSQL)
- **Configuration**: ตั้งค่าในไฟล์ `.env`
- **Note**: หากต้องการใช้ local PostgreSQL ให้ uncomment postgres service ใน docker-compose.yml

## Environment Variables

ไฟล์ `.env` จะถูกโหลดอัตโนมัติผ่าน `env_file` ใน docker-compose.yml

**สำคัญ**: ต้องมีไฟล์ `.env` ที่มีค่าต่อไปนี้:

```env
# Database (Supabase)
DB_HOST=aws-1-ap-southeast-1.pooler.supabase.com
DB_PORT=6543
DB_USER=postgres.xiixrldpusdnifsifzno
DB_PASSWORD=your_password
DB_NAME=postgres
DB_SSLMODE=require

# Server
SERVER_PORT=8080

# JWT
JWT_SECRET=your-super-secret-jwt-key-change-in-production
JWT_EXPIRE_HOURS=24  # Note: config uses JWT_ACCESS_TTL, JWT_REFRESH_TTL instead

# Email
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
EMAIL_FROM=your-email@gmail.com
EMAIL_FROM_NAME=Go2gether Team

# Google OAuth
GOOGLE_CLIENT_ID=your-client-id
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_REDIRECT_URL=http://localhost:8080/api/auth/google/callback

# Frontend
FRONTEND_URL=http://localhost:8081
```

## Database Migration

### Using Supabase (Current Setup)
```bash
# Connect to Supabase and run schema
psql -h aws-1-ap-southeast-1.pooler.supabase.com -p 6543 -U postgres.xiixrldpusdnifsifzno -d postgres -f schema.sql

# Or run migration
psql -h aws-1-ap-southeast-1.pooler.supabase.com -p 6543 -U postgres.xiixrldpusdnifsifzno -d postgres -f migration_add_notifications.sql
```

### Using Local PostgreSQL (Optional)
หาก uncomment postgres service ใน docker-compose.yml:
- Schema จะถูกรันอัตโนมัติเมื่อ container เริ่มทำงานครั้งแรก
- หรือรัน manual: `docker-compose exec postgres psql -U go2gether -d go2gether_db -f /docker-entrypoint-initdb.d/schema.sql`

## Useful Commands

### View Logs
```bash
# All services
docker-compose logs -f

# Backend only
docker-compose logs -f backend

# PostgreSQL (if using local postgres service)
# docker-compose logs -f postgres
```

### Access Database

#### Supabase (Current Setup)
```bash
# Connect to Supabase
psql -h aws-1-ap-southeast-1.pooler.supabase.com -p 6543 -U postgres.xiixrldpusdnifsifzno -d postgres

# Or use external client (Supabase Dashboard)
# Connection string from Supabase dashboard
```

#### Local PostgreSQL (Optional)
หาก uncomment postgres service:
```bash
# Connect to local PostgreSQL
docker-compose exec postgres psql -U go2gether -d go2gether_db

# Or use external client
# Host: localhost
# Port: 5432
# User: go2gether
# Password: go2gether_password
# Database: go2gether_db
```

### Rebuild
```bash
# Rebuild backend image
docker-compose build backend

# Rebuild without cache
docker-compose build --no-cache backend

# Rebuild and restart
docker-compose up -d --build backend
```

### Clean Up
```bash
# Stop containers
docker-compose down

# Stop and remove volumes
docker-compose down -v

# Remove images
docker-compose down --rmi all

# Full cleanup
docker-compose down -v --rmi all
```

## Development with Hot Reload

### Using Air (Recommended)
1. Install air: `go install github.com/cosmtrek/air@latest`
2. Use `docker-compose.dev.yml`
3. Code changes จะ auto-reload

### Manual Reload
```bash
# Restart backend service
docker-compose restart backend
```

## Production Deployment

### Build Production Image
```bash
docker build -t go2gether-backend:latest .
```

### Run Production Container
```bash
docker run -d \
  --name go2gether-backend \
  -p 8080:8080 \
  --env-file .env \
  go2gether-backend:latest
```

## Troubleshooting

### Database Connection Issues
```bash
# Test backend connection to Supabase
docker-compose exec backend wget -O- http://localhost:8080/readyz

# Check backend logs for DB connection errors
docker-compose logs backend | grep -i "database\|connection\|error"

# Verify .env file has correct Supabase credentials
cat .env | grep DB_
```

### Port Already in Use
```bash
# Change ports in docker-compose.yml
ports:
  - "8081:8080"  # Use 8081 instead of 8080
```

### Permission Issues
```bash
# Fix file permissions
sudo chown -R $USER:$USER .
```

## Health Checks

```bash
# Backend health
curl http://localhost:8080/healthz

# Backend readiness (includes DB check)
curl http://localhost:8080/readyz

# Database health
docker-compose exec postgres pg_isready -U go2gether
```

