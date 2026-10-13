package main

import (
	"github.com/fedebram/hambo/client"
	"github.com/fedebram/hambo/internal/cliconfig"
)

func newAPIClient(configPath string) (*client.Client, func(), error) {
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
