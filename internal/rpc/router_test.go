package rpc

import (
	"errors"
	"net"
	"testing"

	"github.com/fedebram/hambo/muxer"
)

func TestRouter(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := muxer.Client(clientConn)
	server := muxer.Server(serverConn)
	defer client.Close()
	defer server.Close()

	r := NewRouter()
	r.Handle("a", func(s *Stream) error {
		return s.Send(msg{Text: "from a"})
	})
	r.Handle("b", func(s *Stream) error {
		return s.Send(msg{Text: "from b"})
	})
	r.Handle("fail", func(s *Stream) error {
		return errors.New("node w9 not found")
	})
	go r.Serve(server)

	tests := []struct {
		name    string
		method  string
		want    string
		wantErr string
	}{
		{"dispatch to a", "a", "from a", ""},
		{"dispatch to b", "b", "from b", ""},
		{"handler error", "fail", "", "node w9 not found"},
		{"unknown method", "nope", "", "unknown method: nope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewStream(client, tt.method)
			if err != nil {
				t.Fatal(err)
			}

			var got msg
			err = s.Recv(&got)

			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("got error %v, want %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}
			if got.Text != tt.want {
				t.Fatalf("got %q, want %q", got.Text, tt.want)
			}
		})
	}
}

func TestRouterBothSides(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := muxer.Client(clientConn)
	server := muxer.Server(serverConn)
	defer client.Close()
	defer server.Close()

	// the server side serves "server.hello"
	serverRouter := NewRouter()
	serverRouter.Handle("server.hello", func(s *Stream) error {
		return s.Send(msg{Text: "hello from server"})
	})
	go serverRouter.Serve(server)

	// the client side also serves: "client.hello"
	// this is the main reason the muxer and so on was built!!!!
	// maybe can be ux better but for now it is pretty good.
	clientRouter := NewRouter()
	clientRouter.Handle("client.hello", func(s *Stream) error {
		return s.Send(msg{Text: "hello from client"})
	})
	go clientRouter.Serve(client)

	// the client opens a stream to the server's handler
	s, err := NewStream(client, "server.hello")
	if err != nil {
		t.Fatal(err)
	}
	var got msg
	if err := s.Recv(&got); err != nil {
		t.Fatal(err)
	}
	if got.Text != "hello from server" {
		t.Fatalf("client got %q", got.Text)
	}

	// the server opens a stream to the client's handler
	s, err = NewStream(server, "client.hello")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Recv(&got); err != nil {
		t.Fatal(err)
	}
	if got.Text != "hello from client" {
		t.Fatalf("server got %q", got.Text)
	}
}
