package worker

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/fedebram/hambo/client"
)

func Start(ctx context.Context, dataDir string) error {
	lock, err := lockDataDir(dataDir)
	if err != nil {
		return err
	}
	defer lock.Close()

	saved, err := loadState(dataDir)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("worker has not joined: run hambo-worker join first")
	}
	if err != nil {
		return err
	}

	if saved.NodeID == "" {
		return errors.New("worker enrollment is incomplete: run hambo-worker join again")
	}

	apiClient, err := client.NewClient(client.Config{
		ServerURL:     saved.ServerURL,
		CACertPEM:     []byte(saved.CACertificate),
		ClientCertPEM: []byte(saved.Certificate),
		ClientKeyPEM:  []byte(saved.PrivateKey),
	})
	if err != nil {
		return fmt.Errorf("create worker API client: %w", err)
	}
	defer apiClient.CloseIdleConnections()

	w := NewWorker(apiClient)
	return w.Run(ctx)
}
