package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

type RunFunc func(ctx context.Context, args []string, out, errOut io.Writer) error

type ArgsValidator func(args []string) error

type Command struct {
	name         string
	description  string
	ArgsUsage    string
	ValidateArgs ArgsValidator
	Run          RunFunc
	subcommands  []*Command
}

func NewCommand(name, description string) *Command {
	return &Command{
		name:        name,
		description: description,
	}
}

func (c *Command) AddCommand(commands ...*Command) {
	c.subcommands = append(c.subcommands, commands...)
}

func (c *Command) Execute(ctx context.Context, args []string, out, errOut io.Writer) error {
	return c.execute(ctx, args, out, errOut, []string{c.name})
}

func (c *Command) execute(ctx context.Context, args []string, out, errOut io.Writer, path []string) error {
	if len(args) > 0 {
		for _, subcommand := range c.subcommands {
			if args[0] == subcommand.name {
				return subcommand.execute(
					ctx,
					args[1:],
					out,
					errOut,
					append(path, subcommand.name),
				)
			}
		}
	}

	if len(args) > 0 {
		if args[0] == "-h" || args[0] == "--help" {
			c.printHelp(out, path)
			return nil
		}
	}

	if c.Run != nil {
		if c.ValidateArgs != nil {
			if err := c.ValidateArgs(args); err != nil {
				setUsageErrorPath(err, path)
				return err
			}
		}

		err := c.Run(ctx, args, out, errOut)
		if err == nil {
			return nil
		}

		setUsageErrorPath(err, path)
		return err
	}

	if len(args) == 0 {
		c.printHelp(out, path)
		return nil
	}

	unknownPath := append(path, args[0])
	return &UsageError{
		path: path,
		err:  fmt.Errorf("unknown command %q", strings.Join(unknownPath, " ")),
	}
}

func (c *Command) printHelp(out io.Writer, path []string) {
	fmt.Fprintln(out, c.description)
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Usage:")
	if len(c.subcommands) > 0 {
		fmt.Fprintf(out, "  %s <command>\n", strings.Join(path, " "))
	} else {
		usage := strings.Join(path, " ")
		if c.ArgsUsage != "" {
			usage += " " + c.ArgsUsage
		}
		fmt.Fprintf(out, "  %s\n", usage)
	}

	if len(c.subcommands) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Available Commands:")

		for _, subcommand := range c.subcommands {
			fmt.Fprintf(
				out,
				"  %-12s %s\n",
				subcommand.name,
				subcommand.description,
			)
		}
	}
}

func setUsageErrorPath(err error, path []string) {
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		usageErr.path = path
	}
}
