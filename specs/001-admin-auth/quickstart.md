# Quickstart: Admin Authentication

**Feature**: 001-admin-auth  
**Date**: 2026-04-20

## Prerequisites

- Go 1.25+
- Docker (for PostgreSQL in dev/test)
- `golang-migrate` CLI: `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`

## 1. Add missing dependency

```bash
cd backend
go get golang.org/x/crypto/bcrypt
```

## 2. Set environment variables

Copy `.env.example` to `.env` (create `.env.example` if it doesn't exist yet):

```bash
# backend/.env.example
DATABASE_URL=postgres://quizshow:quizshow@localhost:5432/quizshow?sslmode=disable
JWT_SECRET=dev-secret-change-in-production-min-32-chars
JWT_ISSUER=https://quizshow.local
COOKIE_SECURE=false
```

## 3. Start PostgreSQL

```bash
docker run --rm -d \
  --name quizshow-pg \
  -e POSTGRES_USER=quizshow \
  -e POSTGRES_PASSWORD=quizshow \
  -e POSTGRES_DB=quizshow \
  -p 5432:5432 \
  postgres:16
```

## 4. Run migrations

```bash
cd backend
migrate -path migrations -database "$DATABASE_URL" up
```

## 5. Seed the admin account

No admin creation endpoint exists in MVP. Insert the seed row directly:

```bash
# Generate bcrypt hash for "password" at cost 12
# You can use: htpasswd -bnBC 12 "" password | tr -d ':\n' | sed 's/$2y/$2a/'
# Or a small Go snippet:

go run - <<'EOF'
package main
import (
    "fmt"
    "golang.org/x/crypto/bcrypt"
)
func main() {
    hash, _ := bcrypt.GenerateFromPassword([]byte("changeme"), 12)
    fmt.Println(string(hash))
}
EOF
```

Then insert:

```sql
INSERT INTO admins (email, password_hash, name)
VALUES ('admin@quizshow.local', '<hash from above>', 'Admin');
```

## 6. Run the server

```bash
cd backend
go run ./cmd/server
```

## 7. Test the login endpoint

```bash
# Login
curl -c cookies.txt -X POST http://localhost:3000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@quizshow.local","password":"changeme"}'

# Should return: {"data":{"access_token":"...","expires_at":"..."}}
# The qz_refresh cookie is stored in cookies.txt

# Refresh
curl -b cookies.txt -c cookies.txt -X POST http://localhost:3000/api/v1/auth/refresh

# Logout (replace <token> with access_token from login)
curl -b cookies.txt -X POST http://localhost:3000/api/v1/auth/logout \
  -H 'Authorization: Bearer <token>'
```

## 8. Run tests

```bash
cd backend

# Unit tests (no DB needed)
go test ./internal/auth/... -run TestUnit -v

# Integration tests (requires running PostgreSQL + migrations)
DATABASE_URL="postgres://quizshow:quizshow@localhost:5432/quizshow?sslmode=disable" \
  go test ./internal/auth/... -run TestIntegration -v
```
