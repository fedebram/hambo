package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/fedebram/hambo/internal/noiseconn"
	"github.com/fedebram/hambo/muxer"
	"github.com/flynn/noise"
)

func main() {
	serverKey, err := noiseconn.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}
	clientKey, err := noiseconn.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}

	// allowlist: only our client may connect
	allowed := func(pub []byte) bool {
		return bytes.Equal(pub, clientKey.Public)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	fmt.Println("server listening on", ln.Addr())

	// server
	go func() {
		for {
			tcp, err := ln.Accept()
			if err != nil {
				return
			}
			go serve(tcp, serverKey, allowed)
		}
	}()

	// client
	tcp, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		log.Fatal(err)
	}
	conn, err := noiseconn.Client(tcp, clientKey, serverKey.Public)
	if err != nil {
		tcp.Close()
		log.Fatal("handshake: ", err)
	}

	client := muxer.Client(conn)
	defer client.Close()

	one, err := client.OpenStream("")
	if err != nil {
		fmt.Println("open:", err)
		return
	}
	three, err := client.OpenStream("")
	if err != nil {
		fmt.Println("open:", err)
		return
	}

	one.Write([]byte("hello from stream 1"))
	three.Write([]byte("hello from stream 3"))

	// done writing
	one.Close()
	three.Close()

	// read everything until the server closes
	echo1, err := io.ReadAll(one)
	fmt.Println("stream 1 echo:", string(echo1), err)

	echo3, err := io.ReadAll(three)
	fmt.Println("stream 3 echo:", string(echo3), err)
}

// serve runs the handshake on one TCP connection and echoes every stream.
func serve(tcp net.Conn, key noise.DHKey, allowed func([]byte) bool) {
	conn, peer, err := noiseconn.Server(tcp, key, allowed)
	if err != nil {
		fmt.Println("server: handshake:", err)
		tcp.Close()
		return
	}
	fmt.Println("server: client connected:", base64.StdEncoding.EncodeToString(peer))

	server := muxer.Server(conn)
	defer server.Close()

	for {
		st, err := server.Accept()
		if err != nil {
			return
		}
		go func() {
			io.Copy(st, st) // echo until the client closes (io.EOF)
			st.Close()
		}()
	}
}
