package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/go-units"
	"github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/cli"
	"golang.org/x/term"
)

type pullProgressRenderer struct {
	out         io.Writer
	reference   string
	interactive bool

	status string
	items  map[string]api.ImagePullEvent
	order  []string

	renderedLines int
}

const pullProgressBarWidth = 16

const ansiClearLine = "\r\x1b[2K"

func isTerminal(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}

	return term.IsTerminal(int(file.Fd()))
}

func newPullProgressRenderer(
	out io.Writer,
	reference string,
	interactive bool,
) *pullProgressRenderer {
	return &pullProgressRenderer{
		out:         out,
		reference:   reference,
		interactive: interactive,
		items:       make(map[string]api.ImagePullEvent),
	}
}

func (r *pullProgressRenderer) Report(event api.ImagePullEvent) error {
	// when the event has no status, then it is a general status of the pulling progress like
	// Resolving from OCI Registry...
	// when instead there is an item, then the status represents what's happening to the item like:
	// layer 1234 "downloading"
	if event.Item == "" {
		r.status = event.Status
	} else {
		// we store every item and its latest event.
		// we store also the order of each unique item. This way we can have a stable display ordering.
		if _, exists := r.items[event.Item]; !exists {
			r.order = append(r.order, event.Item)
		}
		r.items[event.Item] = event
	}

	// if the output is an interactive terminal then we redraw in place.
	// if instead for example we redirect to a file then we print every event line by line
	if r.interactive {
		return r.redraw()
	}
	return r.writePlain(event)
}

func (r *pullProgressRenderer) writePlain(event api.ImagePullEvent) error {
	if event.Item == "" {
		_, err := fmt.Fprintln(r.out, event.Status)
		return err
	}

	// no symbols or other things. So redirect to a file is easy to read and understand.
	switch event.Status {
	// eg layer-sha256:abc...: downloading 50% 5MB/10MB
	case "downloading", "extracting":
		if event.TotalBytes > 0 {
			percentage := pullProgressPercentage(
				event.CurrentBytes,
				event.TotalBytes,
			)

			_, err := fmt.Fprintf(
				r.out,
				"%s: %s %d%% %s/%s\n",
				event.Item,
				event.Status,
				percentage,
				units.HumanSize(float64(event.CurrentBytes)),
				units.HumanSize(float64(event.TotalBytes)),
			)
			return err
		}
	}

	_, err := fmt.Fprintf(r.out, "%s: %s\n", event.Item, event.Status)
	return err
}

func (r *pullProgressRenderer) writeInteractiveRow(event api.ImagePullEvent, labelWidth int) error {
	symbol := pullProgressSymbol(event.Status)
	label := pullProgressLabel(event)

	switch event.Status {
	case "downloading", "extracting":
		if event.TotalBytes > 0 {
			_, err := fmt.Fprintf(
				r.out,
				"%s %-*s   %-11s %s %3d%% %s/%s\n",
				symbol,
				labelWidth,
				label,
				event.Status,
				pullProgressBar(
					event.CurrentBytes,
					event.TotalBytes,
					pullProgressBarWidth,
				),
				pullProgressPercentage(
					event.CurrentBytes,
					event.TotalBytes,
				),
				units.HumanSize(float64(event.CurrentBytes)),
				units.HumanSize(float64(event.TotalBytes)),
			)
			return err
		}
	}

	_, err := fmt.Fprintf(
		r.out,
		"%s %-*s   %s\n",
		symbol,
		labelWidth,
		label,
		event.Status,
	)
	return err
}

// TODO: handle terminal resizing and narrow terminal widths

func (r *pullProgressRenderer) redraw() error {
	if r.renderedLines > 0 {
		if _, err := fmt.Fprintf(
			r.out,
			"\x1b[%dA",
			r.renderedLines,
		); err != nil {
			return err
		}
	}

	renderedLines := 0
	if r.status != "" {
		if _, err := fmt.Fprintf(
			r.out,
			"%s▸ %s\n",
			ansiClearLine,
			r.status,
		); err != nil {
			return err
		}
		renderedLines++
	}

	labelWidth := r.interactiveLabelWidth()
	for _, item := range r.order {
		if _, err := fmt.Fprint(r.out, ansiClearLine); err != nil {
			return err
		}
		if err := r.writeInteractiveRow(
			r.items[item],
			labelWidth,
		); err != nil {
			return err
		}
		renderedLines++
	}

	r.renderedLines = renderedLines
	return nil
}

func (r *pullProgressRenderer) interactiveLabelWidth() int {
	width := 0
	for _, item := range r.order {
		width = max(width, len(pullProgressLabel(r.items[item])))
	}

	return width
}

func pullProgressPercentage(current, total int64) int {
	if total <= 0 {
		return 0
	}

	percentage := int(float64(current) / float64(total) * 100)
	return max(0, min(percentage, 100))
}

func pullProgressLabel(event api.ImagePullEvent) string {
	// kinds came from containerd transfer source code.
	// core/transfer progress name
	for _, kind := range [...]string{
		"index",
		"manifest",
		"config",
		"layer",
		"attestation",
		"unknown",
	} {
		// layer-sha256:b1de9b9d5a817a02... becomes layer b1de9b9d5a81
		if event.Digest != "" && strings.HasPrefix(event.Item, kind+"-") {
			return kind + " " + shortImageDigest(event.Digest)
		}
	}

	// if someone pull using registry/repo@fullsha we need to shorten because a common event like:
	// registry/repo@sha1234 :saved
	// with all the hexs is too much long.
	return shortPullReference(event.Item)
}

func shortPullReference(reference string) string {
	name, digest, found := strings.Cut(reference, "@")
	if !found {
		return reference
	}

	algorithm, encoded, found := strings.Cut(digest, ":")
	if !found || len(encoded) <= shortImageDigestLength {
		return reference
	}

	return name +
		"@" +
		algorithm +
		":" +
		encoded[:shortImageDigestLength] +
		"..."
}

func pullProgressSymbol(status string) string {
	// a little bit flaky to map strings to symbols like that...
	switch status {
	case "waiting":
		return "○"
	case "downloading":
		return "↓"
	case "extracting":
		return "↳"
	case "complete", "extracted", "already exists", "saved":
		return "✓"
	default:
		return "▸"
	}
}

func pullProgressBar(current, total int64, width int) string {
	if width <= 0 {
		return ""
	}

	progress := 0.0
	if total > 0 {
		progress = float64(current) / float64(total)
		progress = max(0, min(progress, 1))
	}

	filled := int(progress * float64(width))
	return "[" +
		strings.Repeat("█", filled) +
		strings.Repeat("░", width-filled) +
		"]"
}

func newPullImageCommand(client imageClient) *cli.Command {
	command := cli.NewCommand("pull", "Pull an image")
	command.ArgsUsage = "<reference>"
	command.ValidateArgs = cli.ExactArgs(1)

	command.Run = func(
		ctx context.Context,
		args []string,
		out, errOut io.Writer,
	) error {
		reference := args[0]
		renderer := newPullProgressRenderer(
			errOut,
			reference,
			isTerminal(errOut),
		)

		// all the progress during pull goes to the stderr
		// after pull completion the digest is printed to the stdout. This way we can catch the digest on a success pull.

		digest, err := client.PullImage(ctx, reference, renderer.Report)
		if err != nil {
			return fmt.Errorf("pull image %q: %w", reference, err)
		}

		if _, err := fmt.Fprintln(errOut, "Pulled:", reference); err != nil {
			return fmt.Errorf("write pulled image reference: %w", err)
		}
		if _, err := fmt.Fprintln(out, digest); err != nil {
			return fmt.Errorf("write pulled image digest: %w", err)
		}

		return nil
	}

	return command
}
