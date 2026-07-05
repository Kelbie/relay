# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies for CGO (required for SQLite)
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the relay with CGO enabled and FTS5 support
RUN CGO_ENABLED=1 go build --tags "fts5" -o /relay cmd/main.go

# Build the crawler, pinned to the same version as in go.mod
RUN CGO_ENABLED=1 GOBIN=/ go install --tags "fts5" github.com/vertex-lab/crawler_v2/cmd/crawl@v1.6.2

# Build the sync tool (rebuilds the Redis graph from the SQLite event store —
# required recovery step whenever Redis is lost/flushed but SQLite is not)
RUN CGO_ENABLED=1 GOBIN=/ go install --tags "fts5" github.com/vertex-lab/crawler_v2/cmd/sync@v1.6.2

# Runtime stage
FROM alpine:3.21

# Install runtime dependencies (bash is used by start.sh for `wait -n`)
RUN apk add --no-cache sqlite-libs ca-certificates tzdata bash

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /relay /app/relay
COPY --from=builder /crawl /app/crawl
COPY --from=builder /sync /app/sync
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh

# Create data directory for SQLite (mount a volume here)
RUN mkdir -p /data

CMD ["/app/start.sh"]
