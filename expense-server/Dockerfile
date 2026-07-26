# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder
WORKDIR /app

# Copy dependency files first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the server binary as a statically compiled binary
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# ── Runner stage ──────────────────────────────────────────────────────────────
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

# Copy the compiled binary and the migrations directory from the builder
COPY --from=builder /app/server .
COPY --from=builder /app/migrations ./migrations

# Expose the default backend port
EXPOSE 8080

# Run the server
CMD ["./server"]
