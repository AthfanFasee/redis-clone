package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var (
	port    = flag.String("port", "6379", "Port to listen on")
	aofFile = flag.String("aof", "database.aof", "Path to AOF file")
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	addr := ":" + *port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	log.Printf("Redis server listening on port %s\n", *port)

	// Initialize AOF (Append-Only File) for persistence
	aof, err := NewAOF(*aofFile)
	if err != nil {
		return fmt.Errorf("failed to create AOF: %w", err)
	}

	// Restore data from AOF file
	restoredCount := 0
	if err := aof.Read(func(value Value) {
		command := strings.ToUpper(value.array[0].bulk)
		args := value.array[1:]

		handler, ok := Handlers[command]
		if !ok {
			log.Printf("WARNING: Invalid command in AOF, skipping: %s\n", command)
			return
		}

		handler(args)
		restoredCount++
	}); err != nil {
		log.Printf("WARNING: Failed to restore from AOF: %v\n", err)
		log.Println("Starting with empty/partial database...")
	} else {
		if restoredCount > 0 {
			log.Printf("Successfully restored %d commands from AOF\n", restoredCount)
		}
	}

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-shutdown
		log.Println("\nReceived shutdown signal, closing server...")

		if err := listener.Close(); err != nil {
			log.Printf("error closing listener: %v\n", err)
		}

		if err := aof.Close(); err != nil {
			log.Printf("error closing AOF: %v\n", err)
		}

		os.Exit(0)
	}()

	// Accept connections loop
	for {
		conn, err := listener.Accept()
		if err != nil {
			// net.ErrClosed is the standard error for closed listener
			if errors.Is(err, net.ErrClosed) {
				log.Println("Server stopped accepting connections")
				return nil
			}
			log.Printf("failed to accept TCP connection: %v\n", err)
			continue
		}

		log.Printf("Client connected: %s\n", conn.RemoteAddr())
		go handleConnection(conn, aof)
	}
}

// handleConnection processes commands from a single client connection.
func handleConnection(conn net.Conn, aof *Aof) {
	remoteAddr := conn.RemoteAddr().String()

	defer func() {
		log.Printf("Client disconnected: %s\n", remoteAddr)
		if err := conn.Close(); err != nil {
			log.Printf("error closing connection for %s: %v\n", remoteAddr, err)
		}
	}()

	// Create RESP reader once per connection to preserve buffer state between reads.
	// The 4KB buffer refills automatically as data is consumed, preventing data loss
	// when commands span multiple network packets. Buffer won't overflow because
	// calling Read() consumes data from buffer.
	resp := NewResp(conn)
	writer := NewWriter(conn)

	for {
		// bufio.Reader.ReadByte() calls conn.Read() which is a blocking system call.
		// The OS puts the goroutine to sleep until:
		//   - Data arrives on the TCP socket
		//   - Connection closes (EOF)
		//   - Error occurs
		// No CPU cycles wasted while waiting.
		value, err := resp.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("error reading from client %s: %v\n", remoteAddr, err)
			break
		}

		if value.typ != TYPE_ARRAY {
			log.Printf("Invalid request from %s: expected array, got %s\n", remoteAddr, value.typ)
			continue
		}

		if len(value.array) == 0 {
			log.Printf("Invalid request from %s: empty command array\n", remoteAddr)
			continue
		}

		command := strings.ToUpper(value.array[0].bulk)
		args := value.array[1:]

		handler, ok := Handlers[command]
		if !ok {
			log.Printf("Unknown command from %s: %s\n", remoteAddr, command)
			writer.Write(Value{typ: TYPE_ERROR, str: fmt.Sprintf("ERR unknown command '%s'", command)})
			continue
		}

		// Persist write commands to AOF
		if command == "SET" || command == "HSET" || command == "DEL" || command == "HDEL" {
			if err := aof.Write(value); err != nil {
				log.Printf("WARNING: Failed to write to AOF: %v\n", err)
			}
		}

		result := handler(args)
		if err := writer.Write(result); err != nil {
			log.Printf("error writing response to %s: %v\n", remoteAddr, err)
			continue
		}
	}
}
