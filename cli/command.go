package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

type RunFunc func(args []string, out io.Writer) error

type Command struct {
	name        string
	description string
	Run         RunFunc
	subcommands []*Command
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

func (c *Command) Execute(args []string, out io.Writer) error {
	return c.execute(args, out, []string{c.name})
}

func (c *Command) execute(args []string, out io.Writer, path []string) error {
	if len(args) > 0 {
		for _, subcommand := range c.subcommands {
			if args[0] == subcommand.name {
				return subcommand.execute(
					args[1:],
					out,
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
		err := c.Run(args, out)
		if err == nil {
			return nil
		}

		var usageErr *UsageError
		if errors.As(err, &usageErr) {
			usageErr.path = path
		}

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
		fmt.Fprintf(out, "  %s\n", strings.Join(path, " "))
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
