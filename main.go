package main

import (
	"fmt"
	"io"
	"net"
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

	defer conn.Close()

	defer func() {
		err := conn.Close()
		if err != nil {
			fmt.Printf("failed to close TCP connection: %v", err)
			return
		}
	}()

	resp := NewResp(conn)
	for {
		value, err := resp.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("error reading from client: %v", err)
		}

		fmt.Println(value)

		writer := NewWriter(conn)
		writer.Write(Value{typ: "string", str: "PONG"})
		if err != nil {
			fmt.Printf("error writing to client: %v", err)
		}
	}
}
