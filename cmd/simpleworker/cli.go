package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

func newRunCommand() *cobra.Command {
	var filename string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a container from a config file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := os.Open(filename)
			if err != nil {
				return fmt.Errorf("open config: %w", err)
			}
			defer file.Close()

			var config ContainerConfig
			decoder := yaml.NewDecoder(file)
			decoder.KnownFields(true)
			if err := decoder.Decode(&config); err != nil {
				return fmt.Errorf("decode config: %w", err)
			}
			if err := decoder.Decode(&struct{}{}); err != io.EOF {
				return fmt.Errorf("config file must contain one document")
			}

			body, err := json.Marshal(config)
			if err != nil {
				return fmt.Errorf("encode config: %w", err)
			}
			c, err := requestContainer(cmd.Context(), http.MethodPost, "/containers", bytes.NewReader(body))
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), c.ID)
			return err
		},
	}
	cmd.Flags().StringVarP(&filename, "file", "f", "", "Container config file (YAML or JSON)")
	cmd.MarkFlagRequired("file")
	return cmd
}

func newGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a container and its current status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := requestContainer(cmd.Context(), http.MethodGet, "/containers/"+url.PathEscape(args[0]), nil)
			if err != nil {
				return err
			}

			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(c)
		},
	}
}

func requestContainer(ctx context.Context, method, path string, body io.Reader) (Container, error) {
	request, err := http.NewRequestWithContext(ctx, method, "http://127.0.0.1:8080"+path, body)
	if err != nil {
		return Container{}, fmt.Errorf("create worker request: %w", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	response, err := httpClient.Do(request)
	if err != nil {
		return Container{}, fmt.Errorf("request worker: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, err := io.ReadAll(response.Body)
		if err != nil {
			return Container{}, fmt.Errorf("read worker error: %w", err)
		}
		return Container{}, fmt.Errorf("worker returned %s: %s", response.Status, strings.TrimSpace(string(message)))
	}

	var c Container
	if err := json.NewDecoder(response.Body).Decode(&c); err != nil {
		return Container{}, fmt.Errorf("decode worker response: %w", err)
	}
	return c, nil
}
