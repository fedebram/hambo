package main

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/moby/moby/client"
	"github.com/spf13/cobra"
)

type daemon struct {
	ctx        context.Context
	docker     *client.Client
	mu         sync.Mutex
	containers map[string]Container
}

func newDaemonCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the worker daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			docker, err := newDockerClient(cmd.Context())
			if err != nil {
				return err
			}
			defer docker.Close()

			d := &daemon{
				ctx:        cmd.Context(),
				docker:     docker,
				containers: make(map[string]Container),
			}
			server := newHTTPServer(d)
			listener, err := net.Listen("tcp", server.Addr)
			if err != nil {
				return fmt.Errorf("listen for HTTP requests: %w", err)
			}
			defer server.Close()

			serverErr := make(chan error, 1)
			go func() {
				serverErr <- server.Serve(listener)
			}()

			cmd.Printf("Worker daemon listening on http://%s\n", server.Addr)
			select {
			case err := <-serverErr:
				return fmt.Errorf("serve HTTP requests: %w", err)
			case <-cmd.Context().Done():
			}

			// basic graceful shutdown
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("stop HTTP server: %w", err)
			}

			cmd.Println("Worker daemon stopped")
			return nil
		},
	}
}
