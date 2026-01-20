package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
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

	defer func() {
		if err := listener.Close(); err != nil {
			fmt.Printf("error closing listener: %v\n", err)
		}
	}()

	fmt.Printf("Listening on port %s\n", *port)

	// Initialize AOF (Append-Only File) for persistence
	aof, err := NewAOF(*aofFile)
	if err != nil {
		return fmt.Errorf("failed to create AOF: %w", err)
	}

	defer func() {
		if err := aof.Close(); err != nil {
			fmt.Printf("error closing AOF: %v\n", err)
		}
	}()

	// Restore data from AOF file if exists
	if err := aof.Read(func(value Value) {
		command := strings.ToUpper(value.array[0].bulk)
		args := value.array[1:]

		handler, ok := Handlers[command]
		if !ok {
			fmt.Printf("WARNING: Invalid command in AOF, skipping: %s\n", command)
			return
		}

		handler(args)
	}); err != nil {
		fmt.Printf("WARNING: Failed to restore from AOF: %v\n", err)
		fmt.Println("Starting with empty/partial database...")
		// Continue to serve requests than crash
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("failed to accept TCP connection: %v\n", err)
			continue // Keep accepting other connections
		}

		fmt.Printf("Client connected: %s\n", conn.RemoteAddr())

		go handleConnection(conn, aof)
	}
}

// handleConnection processes commands from a single client connection.
func handleConnection(conn net.Conn, aof *Aof) {
	defer func() {
		fmt.Printf("Client disconnected: %s\n", conn.RemoteAddr())
		if err := conn.Close(); err != nil {
			fmt.Printf("error closing connection for %s: %v\n", conn.RemoteAddr(), err)
		}
	}()

	// Create RESP reader once per connection to preserve buffer state between reads.
	// The 4KB buffer refills automatically as data is consumed, preventing data loss
	// when commands span multiple network packets. Also buffer won't overflow because
	// calling Read() consumes data from buffer.
	resp := NewResp(conn)

	for {
		// r.reader is bufio.Reader, bufio.Reader calls conn.Read()
		// conn.Read() is a SYSTEM CALL that blocks until:
		//   - Data arrives on the TCP socket
		//   - Connection closes
		//   - Error occurs
		// hence OS puts goroutine to sleep and No CPU used while waiting
		value, err := resp.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("error reading from client %s: %v\n", conn.RemoteAddr(), err)
			break
		}

		if value.typ != TYPE_ARRAY {
			fmt.Printf("Invalid request from %s, expected array\n", conn.RemoteAddr())
			continue
		}

		if len(value.array) == 0 {
			fmt.Printf("Invalid request from %s, expected array length > 0\n", conn.RemoteAddr())
			continue
		}

		command := strings.ToUpper(value.array[0].bulk)
		args := value.array[1:]

		writer := NewWriter(conn)

		handler, ok := Handlers[command]
		if !ok {
			fmt.Printf("Unknown command from %s: %s\n", conn.RemoteAddr(), command)
			writer.Write(Value{typ: TYPE_ERROR, str: fmt.Sprintf("ERR unknown command '%s'", command)})
			continue
		}

		// Write commands to AOF for persistence
		if command == "SET" || command == "HSET" {
			aof.Write(value)
		}

		result := handler(args)
		writer.Write(result)
	}
}
