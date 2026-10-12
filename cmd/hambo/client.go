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

	httpClient, err := client.NewMTLSClientFromPEM(
		[]byte(cfg.CACert),
		[]byte(cfg.ClientCert),
		[]byte(cfg.ClientKey),
	)
	if err != nil {
		return nil, nil, err
	}

	apiClient, err := client.NewClient(cfg.ServerURL, httpClient)
	if err != nil {
		httpClient.CloseIdleConnections()
		return nil, nil, err
	}

	return apiClient, httpClient.CloseIdleConnections, nil
}
