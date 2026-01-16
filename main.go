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

	for {
		buf := make([]byte, 1024)

		// Read message from client
		_, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("error reading from client: %v", err)
		}

		_, err = conn.Write([]byte("+PONG\r\n"))
		if err != nil {
			fmt.Printf("error writing to client: %v", err)
		}
	}
}
