# Build stage
FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o main ./webhook/

# Final stage
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main .
CMD ["./main"]

EXPOSE 6905