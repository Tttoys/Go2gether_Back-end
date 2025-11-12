# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies and swag for swagger docs
RUN apk add --no-cache git ca-certificates tzdata
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code (excluding files in .dockerignore)
COPY . .

# Generate swagger docs if not already generated
# This ensures docs package is available for build
RUN if [ ! -f docs/docs.go ]; then \
      export PATH=$PATH:/root/go/bin && \
      swag init -g cmd/main.go -o docs || echo "Swagger generation skipped"; \
    fi

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/main ./cmd/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates and wget for HTTPS requests and health checks
RUN apk --no-cache add ca-certificates tzdata wget

# Set timezone
ENV TZ=Asia/Bangkok

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/main .

# Copy schema file (optional, for reference)
COPY --from=builder /app/schema.sql .

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Run the application
CMD ["./main"]

