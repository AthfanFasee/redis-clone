# Redis Clone in Go

A lightweight Redis implementation in Go that speaks the RESP protocol. Built to understand how Redis works internally - handles concurrent connections, persists data to disk, and works with standard Redis clients.

## Quick Start

```bash
# Start server (default port 6379)
go run .

# Custom port
go run . -port 8080

# Custom AOF file
go run . -aof mydata.aof

# Connect with redis-cli
redis-cli
```

## Docker

```bash
# Build the image
docker build -t redis-clone .

# Run the container
docker run -p 6379:6379 redis-clone

# Run with custom port
docker run -p 8080:8080 redis-clone -port 8080

# Run with volume for AOF persistence
docker run -p 6379:6379 -v $(pwd)/data:/app redis-clone -aof /app/database.aof
```

## Features

- RESP protocol support
- Concurrent client connections
- Thread-safe operations (RWMutex)
- AOF persistence with auto-restore
- Compatible with redis-cli

## Commands

### String Operations

```bash
# PING - Test connectivity
PING                    # Returns: PONG
PING "Hello"            # Returns: "Hello"

# SET - Store key-value
SET name "Alice"        # Returns: OK

# GET - Retrieve value
GET name                # Returns: "Alice"
GET missing             # Returns: (nil)

# DEL - Delete keys
DEL name                # Returns: (integer) 1
DEL key1 key2           # Returns: (integer) 2

# EXISTS - Check existence
EXISTS name             # Returns: (integer) 1
EXISTS key1 key2        # Returns: (integer) 2
```

### Hash Operations

```bash
# HSET - Set hash field
HSET user:1 name "Bob"           # Returns: OK
HSET user:1 email "bob@test.com" # Returns: OK

# HGET - Get hash field
HGET user:1 name                 # Returns: "Bob"
HGET user:1 missing              # Returns: (nil)

# HGETALL - Get all fields
HGETALL user:1
# Returns:
# 1) "name"
# 2) "Bob"
# 3) "email"
# 4) "bob@test.com"

# HDEL - Delete hash fields
HDEL user:1 email                # Returns: (integer) 1
HDEL user:1 f1 f2                # Returns: (integer) 2
```

## Architecture

**Core Components:**

- RESP parser/writer for protocol handling
- Thread-safe stores with RWMutex
- AOF persistence (syncs every 3s)
- Goroutine per client connection

**Persistence:**

- Write commands (SET, HSET) logged to AOF
- Data auto-restored on startup
- AOF file: `database.aof` (configurable)

Educational project, not production-ready.
