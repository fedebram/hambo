package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

func containersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "containers",
		Short: "Manage containers",
	}
	cmd.AddCommand(containersRunCmd(), containersListCmd(), containersRemoveCmd())
	return cmd
}

func containersRunCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "run -f <file>",
		Short: "Run a container from a spec file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContainersRun(configPath, file)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "the container spec (yaml)")
	cmd.MarkFlagRequired("file")
	return cmd
}

// runContainersRun reads the spec in file and asks the server to run it.
func runContainersRun(configPath, file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var spec api.ContainerSpec
	if err := yaml.Load(data, &spec, yaml.WithKnownFields()); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}

	creds, err := loadCredentials(configPath)
	if err != nil {
		return err
	}

	sess, err := connect(creds)
	if err != nil {
		return err
	}
	defer sess.Close()

	var c api.Container
	if err := rpc.Call(sess, "containers.run", spec, &c); err != nil {
		return err
	}
	fmt.Printf("added %s (%s)\n", c.Spec.Name, c.ID)
	return nil
}

func containersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContainersList(configPath)
		},
	}
}

// runContainersList asks the server for its containers and prints them as a table.
func runContainersList(configPath string) error {
	creds, err := loadCredentials(configPath)
	if err != nil {
		return err
	}

	sess, err := connect(creds)
	if err != nil {
		return err
	}
	defer sess.Close()

	var containers []api.Container
	if err := rpc.Call(sess, "containers.list", nil, &containers); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tID\tIMAGE\tAGE")
	for _, c := range containers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.Spec.Name, c.ID, c.Spec.Image, age(c.CreatedAt))
	}
	return w.Flush()
}

func containersRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runContainersRemove(configPath, args[0])
		},
	}
}

// runContainersRemove asks the server to remove the container with this name.
func runContainersRemove(configPath, name string) error {
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
	if err := rpc.Call(sess, "containers.remove", name, &answer); err != nil {
		return err
	}
	fmt.Println(answer, name)
	return nil
}
