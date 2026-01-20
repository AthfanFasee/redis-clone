# BUILD STAGE
FROM golang:1.25.5-alpine3.23 AS builder

WORKDIR /app

# Copy all Go files
COPY *.go ./

# Build the application
RUN go build -o redis-clone .

# RUN STAGE
FROM alpine:3.23

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /app/redis-clone .

# Set executable permissions
RUN chmod +x redis-clone

# Expose Redis default port
EXPOSE 6379

# Run the application
CMD ["./redis-clone"]