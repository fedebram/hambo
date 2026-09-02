package main

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"

	hamboclient "github.com/fedebram/hambo/client"
	internalapi "github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/container"
)

func TestHealth(t *testing.T) {
	server := httptest.NewServer(internalapi.NewServer(nil))
	t.Cleanup(server.Close)

	client, err := hamboclient.NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := app{
		client: client,
		stdout: &stdout,
		stderr: &stderr,
	}

	err = app.run(context.Background(), []string{"health"})
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if got, want := stdout.String(), "ok\n"; got != want {
		t.Errorf("got stdout %q, want %q", got, want)
	}

	if got := stderr.String(); got != "" {
		t.Errorf("got stderr %q, want no output", got)
	}
}

func TestHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no arguments", nil},
		{"help command", []string{"help"}},
		{"short help flag", []string{"-h"}},
		{"long help flag", []string{"--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			app := app{
				stdout: &stdout,
				stderr: &stderr,
			}

			err := app.run(context.Background(), tt.args)
			if err != nil {
				t.Fatalf("unexpected run error: %v", err)
			}

			want := "Usage:\n  hambo health\n  hambo create NAME IMAGE\n"
			if got := stdout.String(); got != want {
				t.Errorf("got stdout %q, want %q", got, want)
			}

			if got := stderr.String(); got != "" {
				t.Errorf("got stderr %q, want no output", got)
			}
		})
	}
}

func TestCreateContainer(t *testing.T) {
	service := container.NewService(
		container.NewMemoryStore(),
		container.NewMemoryQueue(),
	)
	server := httptest.NewServer(internalapi.NewServer(service))
	t.Cleanup(server.Close)

	client, err := hamboclient.NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := app{
		client: client,
		stdout: &stdout,
		stderr: &stderr,
	}

	err = app.run(context.Background(), []string{
		"create",
		"hello",
		"docker.io/library/alpine:latest",
	})
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if got, want := stdout.String(), "hello\n"; got != want {
		t.Errorf("got stdout %q, want %q", got, want)
	}

	if got := stderr.String(); got != "" {
		t.Errorf("got stderr %q, want no output", got)
	}
}
