package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/spf13/cobra"
)

func nodesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "Manage nodes",
	}
	cmd.AddCommand(nodesListCmd(), nodesRemoveCmd())
	return cmd
}

func nodesRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a node: it can't connect again without a new token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNodesRemove(configPath, args[0])
		},
	}
}

// runNodesRemove asks the server to remove the node with this name.
func runNodesRemove(configPath, name string) error {
	creds, err := loadCredentials(configPath)
	if err != nil {
		return err
	}

	sess, err := connect(creds)
	if err != nil {
		return err
	}
	defer sess.Close()

	var answer string
	if err := rpc.Call(sess, "nodes.remove", name, &answer); err != nil {
		return err
	}
	fmt.Println(answer, name)
	return nil
}

func nodesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the nodes that joined the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNodesList(configPath)
		},
	}
}

// runNodesList asks the server for its nodes and prints them as a table.
func runNodesList(configPath string) error {
	creds, err := loadCredentials(configPath)
	if err != nil {
		return err
	}

	sess, err := connect(creds)
	if err != nil {
		return err
	}
	defer sess.Close()

	var nodes []api.Node
	if err := rpc.Call(sess, "nodes.list", nil, &nodes); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tROLE\tSTATUS\tSUBNET\tAGE")
	for _, n := range nodes {
		status := "offline"
		if n.Connected {
			status = "connected"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", n.Name, n.Role, status, n.Subnet, age(n.JoinedAt))
	}
	return w.Flush()
}

// age formats how long ago t was, like 45s, 12m, 5h or 3d.
func age(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
