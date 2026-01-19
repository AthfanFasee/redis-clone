package main

import (
	"fmt"
	"io"
	"net"
	"strings"
)

func main() {
	listener, err := net.Listen("tcp", ":6379")
	if err != nil {
		fmt.Printf("failed to listen on :6379: %v", err)
		return
	}

	fmt.Println("Listening on port :6379")

	conn, err := listener.Accept()
	if err != nil {
		fmt.Printf("failed to accept TCP connection: %v", err)
		return
	}

	defer func() {
		err := conn.Close()
		if err != nil {
			fmt.Printf("failed to close TCP connection: %v", err)
			return
		}
	}()

	for {
		resp := NewResp(conn)
		value, err := resp.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("error reading from client: %v", err)
		}

		if value.typ != "array" {
			fmt.Println("Invalid request, expected array")
			continue
		}

		if len(value.array) == 0 {
			fmt.Println("Invalid request, expected array length > 0")
			continue
		}

		command := strings.ToUpper(value.array[0].bulk)

		// In Go, slicing beyond the length returns an empty slice, not a panic.
		// Hence no need to check for array length here.
		args := value.array[1:]

		writer := NewWriter(conn)

		handler, ok := Handlers[command]
		if !ok {
			fmt.Println("Invalid command: ", command)
			writer.Write(Value{typ: "string", str: ""})
			continue
		}

		result := handler(args)
		writer.Write(result)
	}
}
