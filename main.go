package main

import (
	"fmt"
	"io"
	"net"

	"github.com/fedebram/hambo/muxer"
)

func main() {
	serverConn, clientConn := net.Pipe()

	server := muxer.NewSession(serverConn)
	client := muxer.NewSession(clientConn)

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
