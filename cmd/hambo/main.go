package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/fedebram/hambo/cli"
	hamboclient "github.com/fedebram/hambo/client"
)

const defaultDaemonURL = "http://127.0.0.1:8080"

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	apiClient, err := hamboclient.NewClient(defaultDaemonURL, http.DefaultClient)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	os.Exit(run(ctx, apiClient, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, client imageClient, args []string, out, errOut io.Writer) int {
	rootCmd := newRootCommand(client)

	err := rootCmd.Execute(ctx, args, out, errOut)
	if err == nil {
		return 0
	}

	fmt.Fprintln(errOut, "Error:", err)
	printResponseErrorFields(errOut, err)

	var usageErr *cli.UsageError
	if errors.As(err, &usageErr) {
		fmt.Fprintln(errOut)
		fmt.Fprintf(
			errOut,
			"See '%s -h' for help.\n",
			usageErr.CommandPath(),
		)
		return 2
	}

	return 1
}

func printResponseErrorFields(out io.Writer, err error) {
	var responseErr *hamboclient.ResponseError
	if !errors.As(err, &responseErr) {
		return
	}

	fields := make([]string, 0, len(responseErr.Fields))
	for field := range responseErr.Fields {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	for _, field := range fields {
		fmt.Fprintf(out, "  %s: %s\n", field, responseErr.Fields[field])
	}
}

func newRootCommand(client imageClient) *cli.Command {
	rootCmd := cli.NewCommand("hambo", "Manage containers and images")
	rootCmd.AddCommand(newImageCommand(client))
	return rootCmd
}
