# Build stage
FROM golang:alpine AS builder
WORKDIR /app

# Install dependencies first - these rarely change
# RUN apk add --no-cache libxml2 libxml2-dev gcc musl-dev
RUN apk add --no-cache --virtual .build-deps \
    libxml2 libxml2-dev gcc musl-dev

# Copy only go.mod and go.sum first to leverage caching
COPY go.mod go.sum ./
RUN go mod download

# Then copy the rest and build
COPY . .
RUN go build -o main .

# Final stage
FROM alpine:latest

# Create a non-root user with UID 1000
RUN adduser -D -u 1000 appuser 

# Install runtime dependencies for libxml2
RUN apk add --no-cache libxml2

# Download the cec binary for the current architecture
RUN ARCH=$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/') && \
    wget -O /usr/local/bin/cec "https://download.pydio.com/latest/cells-client/release/4.2.1/linux-${ARCH}/cec" && \
    chmod +x /usr/local/bin/cec

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

# Fix syntax error in CMD (missing comma)
CMD ["./main", "--serve"]

EXPOSE 6905