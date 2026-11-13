package main

import (
	"fmt"

	"github.com/fedebram/hambo/internal/rpc"
	"github.com/spf13/cobra"
)

func tokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage join tokens",
	}
	cmd.AddCommand(tokenCreateCmd())
	return cmd
}

func tokenCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a one-time join token",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTokenCreate(configPath)
		},
	}
}

// runTokenCreate asks the server for a new join token and prints it.
func runTokenCreate(configPath string) error {
	creds, err := loadCredentials(configPath)
	if err != nil {
		return err
	}

	sess, err := connect(creds)
	if err != nil {
		return err
	}
	defer sess.Close()

	var token string
	if err := rpc.Call(sess, "tokens.create", nil, &token); err != nil {
		return err
	}

	fmt.Println(token)
	return nil
}
