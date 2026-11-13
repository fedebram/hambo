package main

import (
	"fmt"
	"time"

	"github.com/fedebram/hambo/internal/rpc"
	"github.com/spf13/cobra"
)

func pingCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Check the connection to the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPing(configPath)
		},
	}
}

// runPing connects to the server and calls ping.
func runPing(configPath string) error {
	creds, err := loadCredentials(configPath)
	if err != nil {
		return err
	}

	start := time.Now()

	sess, err := connect(creds)
	if err != nil {
		return err
	}
	defer sess.Close()

	var answer string
	if err := rpc.Call(sess, "ping", nil, &answer); err != nil {
		return err
	}

	fmt.Printf("%s from %s in %s\n", answer, creds.Server, time.Since(start).Round(time.Microsecond))
	return nil
}
