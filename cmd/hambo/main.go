package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/fedebram/hambo/api"
	hamboclient "github.com/fedebram/hambo/client"
)

const defaultAddress = "http://127.0.0.1:8080"

type app struct {
	client *hamboclient.Client
	stdout io.Writer
	stderr io.Writer
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	client, err := hamboclient.NewClient(defaultAddress, &http.Client{})
	if err == nil {
		app := app{
			client: client,
			stdout: os.Stdout,
			stderr: os.Stderr,
		}
		err = app.run(ctx, os.Args[1:])
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "hambo:", err)
		os.Exit(1)
	}
}

func (app *app) run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return app.printUsage()
	}

	switch args[0] {
	case "help", "-h", "--help":
		return app.printUsage()
	case "health":
		return app.runHealth(ctx, args[1:])
	case "create":
		return app.runCreate(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (app *app) runHealth(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: hambo health")
	}

	health, err := app.client.Health(ctx)
	if err != nil {
		return fmt.Errorf("check daemon health: %w", err)
	}

	if _, err := fmt.Fprintln(app.stdout, health.Status); err != nil {
		return fmt.Errorf("write health output: %w", err)
	}

	return nil
}

func (app *app) runCreate(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: hambo create NAME IMAGE")
	}

	container, err := app.client.CreateContainer(ctx, api.CreateContainerRequest{
		Name:  args[0],
		Image: args[1],
	})
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	if _, err := fmt.Fprintln(app.stdout, container.Name); err != nil {
		return fmt.Errorf("write create output: %w", err)
	}

	return nil
}

func (app *app) printUsage() error {
	if _, err := fmt.Fprint(app.stdout, "Usage:\n  hambo health\n  hambo create NAME IMAGE\n"); err != nil {
		return fmt.Errorf("write usage: %w", err)
	}

	return nil
}
