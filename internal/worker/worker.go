package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/backoff"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/transport"
	"github.com/fedebram/hambo/internal/wireguard"
)

var errNotJoined = errors.New("not joined: run hambod join --token <token>")

// worker is a node connected to its server.
type worker struct {
	id     identity
	router *rpc.Router

	// one wg.config call at a time
	wgMu sync.Mutex
}

// Run connects to the server and answers the calls the server makes to
// this node. If the connection drops, it connects again with backoff.
func Run(dataDir string) error {
	id, err := loadIdentity(dataDir)
	if err != nil {
		return err
	}

	// fail now if we can't create wg0, like without root
	if err := wireguard.SetInterface(id.WGPrivate, api.WGPort); err != nil {
		return fmt.Errorf("wireguard (needs root): %w\nif the key is missing, run hambod join again", err)
	}

	w := &worker{id: id}
	w.router = w.routes()

	retry := backoff.NewBackoff(time.Second, 30*time.Second)
	for {
		connected, err := w.connectAndServe()
		if errors.Is(err, errNotJoined) {
			return err // retrying won't help
		}
		// we were connected: the next try starts again from 1s (base)
		if connected {
			retry.Reset("server")
		}
		delay := retry.Next("server")
		fmt.Println("disconnected:", err)
		fmt.Println("connecting again in", delay)
		time.Sleep(delay)
	}
}

// connectAndServe connects to the server and handles its requests
// until the connection ends. connected is true if the server accepted
// the node.
func (w *worker) connectAndServe() (connected bool, err error) {
	sess, err := transport.Dial(w.id.ServerAddress, w.id.noiseKey(), w.id.ServerPublic)
	if err != nil {
		return false, err
	}
	defer sess.Close()

	// the server refuses ping from a key it doesn't know
	var pong string
	err = rpc.Call(sess, "ping", nil, &pong)
	var rpcErr *rpc.Error
	if errors.As(err, &rpcErr) {
		return false, errNotJoined
	}
	if err != nil {
		return false, err
	}

	fmt.Println("connected to", w.id.ServerAddress)
	return true, w.router.Serve(context.Background(), sess)
}
