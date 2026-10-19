package main

import (
	"github.com/fedebram/hambo/client"
	"github.com/fedebram/hambo/internal/cliconfig"
	"github.com/spf13/cobra"
)

func newAPIClient(cmd *cobra.Command) (*client.Client, func(), error) {
	configPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return nil, nil, err
	}
	cfg, err := cliconfig.Load(configPath)
	if err != nil {
		return nil, nil, err
	}

	apiClient, err := client.NewClient(client.Config{
		ServerURL:     cfg.ServerURL,
		CACertPEM:     []byte(cfg.CACert),
		ClientCertPEM: []byte(cfg.ClientCert),
		ClientKeyPEM:  []byte(cfg.ClientKey),
	})
	if err != nil {
		return nil, nil, err
	}

	return apiClient, apiClient.CloseIdleConnections, nil
}
