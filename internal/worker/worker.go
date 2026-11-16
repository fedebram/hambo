package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/transport"
	"github.com/fedebram/hambo/internal/wireguard"
)

var errNotJoined = errors.New("not joined: run hambod join --token <token>")

// worker is a node connected to its server.
type worker struct {
	id     identity
	router *rpc.Router
}

// Run connects to the server and answers the calls the server makes to
// this node. If the connection drops, it connects again.
func Run(dataDir string) error {
	id, err := loadIdentity(dataDir)
	if err != nil {
		return err
	}

	// fail now if we can't create wg0, like without root
	if err := wireguard.SetInterface(id.WGPrivate); err != nil {
		return fmt.Errorf("wireguard (needs root): %w\nif the key is missing, run hambod join again", err)
	}

	w := &worker{id: id}
	w.router = w.routes()

	for {
		err := w.connectAndServe()
		if errors.Is(err, errNotJoined) {
			return err // retrying won't help
		}
		fmt.Println("disconnected:", err)
		fmt.Println("connecting again in 3s")
		time.Sleep(3 * time.Second)
	}
}

// connectAndServe connects to the server and handles its requests
// until the connection ends.
func (w *worker) connectAndServe() error {
	sess, err := transport.Dial(w.id.ServerAddress, w.id.noiseKey(), w.id.ServerPublic)
	if err != nil {
		return err
	}
	defer sess.Close()

	// the server refuses ping from a key it doesn't know
	var pong string
	err = rpc.Call(sess, "ping", nil, &pong)
	var rpcErr *rpc.Error
	if errors.As(err, &rpcErr) {
		return errNotJoined
	}
	if err != nil {
		return err
	}

	fmt.Println("connected to", w.id.ServerAddress)
	return w.router.Serve(context.Background(), sess)
}
