package main

import (
	"context"
	"log"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/fedebram/hambo/internal/taskutil"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		return err
	}
	defer client.Close()

	ctx := namespaces.WithNamespace(context.Background(), "example")

	task, exitStatusCh, err := taskutil.RunOnce(ctx, client)
	if err != nil {
		return err
	}

	st := <-exitStatusCh
	code, exitedAt, err := st.Result()
	if err != nil {
		return err
	}
	log.Printf("task %s: exited with code %d at %s", task.ID(), code, exitedAt)
	return nil
}
