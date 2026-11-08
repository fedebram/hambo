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
	defer client.Close()
	defer server.Close()

	// now the server can accept any new stream and echoes back
	go func() {
		for {
			st, err := server.Accept()
			if err != nil {
				return
			}
			go io.Copy(st, st)
		}
	}()

	// only the client side now open the streams
	one := client.OpenStream(1)
	three := client.OpenStream(3)

	one.Write([]byte("hello from stream 1"))
	three.Write([]byte("hello from stream 3"))

	buf := make([]byte, 1024)

	n, _ := one.Read(buf)
	fmt.Println("stream 1 echo:", string(buf[:n]))

	n, _ = three.Read(buf)
	fmt.Println("stream 3 echo:", string(buf[:n]))
}
