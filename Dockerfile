# Build stage
FROM golang:alpine AS builder
WORKDIR /app

# Install build dependencies - cache this layer
RUN apk add --no-cache --virtual .build-deps \
    libxml2-dev gcc musl-dev

# Copy go.mod and go.sum first for better dependency caching
COPY go.mod go.sum ./

# Download dependencies - this layer will be cached unless go.mod/go.sum changes
RUN go mod download && go mod verify

# Copy source code (this invalidates cache when code changes)
COPY . .

# Build with cache mount for Go build cache and module cache
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o main .

# Runtime dependencies stage - cache this separately
FROM alpine:latest AS runtime-deps

# Install runtime dependencies
RUN apk add --no-cache libxml2

# Download the cec binary - cache this layer
RUN ARCH=$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/') && \
    wget -O /usr/local/bin/cec "https://download.pydio.com/latest/cells-client/release/4.2.1/linux-${ARCH}/cec" && \
    chmod +x /usr/local/bin/cec

# Final stage
FROM alpine:latest

# Install only runtime dependencies
RUN apk add --no-cache libxml2

# Create a non-root user with UID 1000
RUN adduser -D -u 1000 appuser 

# Copy cec binary from runtime-deps stage
COPY --from=runtime-deps /usr/local/bin/cec /usr/local/bin/cec

# Set up user's home directory and ensure proper permissions
WORKDIR /home/appuser
RUN mkdir -p .config/pydio/cells-client && chown -R appuser:appuser /home/appuser

# Copy the binary from builder
COPY --from=builder /app/main .
RUN chown appuser:appuser ./main

# Create logs directory following Linux standards
RUN mkdir -p /var/log/curate && chown -R appuser:appuser /var/log/curate

# Switch to non-root user
USER appuser

CMD ["./main", "--serve"]

EXPOSE 6905