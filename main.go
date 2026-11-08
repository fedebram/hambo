package main

import (
	"fmt"
	"io"
	"net"

	"github.com/fedebram/hambo/muxer"
)

func main() {
	serverConn, clientConn := net.Pipe()

	server := muxer.Server(serverConn)
	client := muxer.Client(clientConn)
	defer client.Close()
	defer server.Close()

	// now the server can accept any new stream and echoes back
	go func() {
		for {
			st, err := server.Accept()
			if err != nil {
				return
			}
			go func() {
				io.Copy(st, st) // echo until the client closes (io.EOF)
				st.Close()      // then close our side too
			}()
		}
	}()

	// only the client side now open the streams
	one, err := client.OpenStream() // id 1
	if err != nil {
		fmt.Println("open:", err)
		return
	}
	three, err := client.OpenStream() // id 3
	if err != nil {
		fmt.Println("open:", err)
		return
	}

	one.Write([]byte("hello from stream 1"))
	three.Write([]byte("hello from stream 3"))

	// done writing: the server's io.Copy sees io.EOF and closes its side
	one.Close()
	three.Close()

	// read everything until the server closes (io.ReadAll stops at io.EOF)
	echo1, err := io.ReadAll(one)
	fmt.Println("stream 1 echo:", string(echo1), err)

	echo3, err := io.ReadAll(three)
	fmt.Println("stream 3 echo:", string(echo3), err)
}
