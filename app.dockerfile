# BUILD STAGE
FROM golang:1.25.5-alpine3.23 AS builder

WORKDIR /app

# Copy all Go files
COPY *.go ./

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o redis-clone .

# RUN STAGE
FROM alpine:3.23

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /app/redis-clone .

# Create directory for AOF file with proper permissions
RUN mkdir -p /app/data && chmod 755 /app/data

# Expose Redis default port
EXPOSE 6379

# Use non-root user for security
RUN addgroup -g 1000 redis && \
    adduser -D -u 1000 -G redis redis && \
    chown -R redis:redis /app

USER redis

# Run with AOF in data directory
CMD ["./redis-clone", "-aof", "/app/data/database.aof"]