package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fedebram/hambo/internal/blobstore"
	"github.com/fedebram/hambo/internal/image"
	"github.com/fedebram/hambo/internal/registry"
	"github.com/spf13/cobra"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	code := run(ctx)
	stop()
	os.Exit(code)
}

func run(ctx context.Context) int {
	rootCmd := newRootCommand()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		return 1
	}
	return 0
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "hambo",
		Short: "Manage containers and images",
	}

	root.AddCommand(&cobra.Command{
		Use:   "fetch <reference>",
		Short: "Fetch an OCI manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := registry.ParseImageReference(args[0])
			if err != nil {
				return err
			}

			store, err := blobstore.New("./data/content")
			if err != nil {
				return err
			}
			httpClient := &http.Client{
				Timeout: 30 * time.Second,
			}

			result, err := registry.FetchManifest(httpClient, ref)
			if err != nil {
				return err
			}

			d, err := store.PutBytes(result.Content)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), d)
			return err
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "pull <reference>",
		Short: "Pull an OCI image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := registry.ParseImageReference(args[0])
			if err != nil {
				return err
			}

			store, err := blobstore.New("./data/content")
			if err != nil {
				return err
			}

			httpClient := &http.Client{
				Timeout: 5 * time.Minute,
			}

			// pulling progress to stderr
			target, err := image.Pull(httpClient, store, ref, cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			metadataStore, err := image.Open("./data/metadata.db")
			if err != nil {
				return err
			}

			saveErr := metadataStore.Save(image.Image{
				Name:       ref.String(),
				Descriptor: target,
			})
			closeErr := metadataStore.Close()

			if err := errors.Join(saveErr, closeErr); err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), target.Digest)
			return err
		},
	})
	return root
}
