package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/fedebram/hambo/cli"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	rootCmd := newRootCommand()

	err := rootCmd.Execute(args, out)
	if err == nil {
		return 0
	}

	fmt.Fprintln(errOut, "Error:", err)

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

func newRootCommand() *cli.Command {
	rootCmd := cli.NewCommand("hambo", "Manage containers and images")
	helloCmd := cli.NewCommand("hello", "Print a greeting")

	helloCmd.Run = func(args []string, out io.Writer) error {
		if len(args) != 0 {
			return cli.UsageErrorf("unexpected argument %q", args[0])
		}
		fmt.Fprintln(out, "hello hambo")
		return nil
	}

	rootCmd.AddCommand(helloCmd)
	return rootCmd
}
