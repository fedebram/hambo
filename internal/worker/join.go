package worker

import (
	"errors"
	"fmt"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/jointoken"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/transport"
)

// Join joins the server at address as a worker, with a join token.
// It creates the node's identity in dataDir the first time.
func Join(dataDir, address, token string) error {
	serverPublic, tokenID, secret, err := jointoken.Parse(token)
	if err != nil {
		return err
	}

	id, err := loadOrCreateIdentity(dataDir, address, serverPublic)
	if err != nil {
		return err
	}

	sess, err := transport.Dial(id.ServerAddress, id.noiseKey(), id.ServerPublic)
	if err != nil {
		return fmt.Errorf("can't connect to %s: %w", id.ServerAddress, err)
	}
	defer sess.Close()

	// "joined", or "already joined" if this worker joined before
	req := api.JoinRequest{ID: tokenID, Secret: secret, WGPublic: id.WGPublic}
	var answer string
	err = rpc.Call(sess, "join", req, &answer)
	var rpcErr *rpc.Error
	if errors.As(err, &rpcErr) {
		return fmt.Errorf("join failed: %w\ncreate a new token with: hambo token create", err)
	}
	if err != nil {
		return fmt.Errorf("join failed: %w\nrun hambod join again", err)
	}

	fmt.Println(answer, id.ServerAddress)
	return nil
}
