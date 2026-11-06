package main

import (
	"fmt"
	"io"
	"net"
)

func main() {
	server, client := net.Pipe()

	go func() {
		defer server.Close()
		io.Copy(server, server)
	}()

	client.Write([]byte("hello hambo"))

	buf := make([]byte, 1024)
	n, _ := client.Read(buf)
	fmt.Println("echo:", string(buf[:n]))

	client.Close()
}
