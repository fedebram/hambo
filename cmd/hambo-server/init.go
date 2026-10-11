package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fedebram/hambo/internal/pki"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the server certificate authority and TLS credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) (initErr error) {
			if dataDir == "" {
				return errors.New("data directory cannot be empty")
			}

			now := time.Now()
			ca, err := pki.GenerateCA(now)
			if err != nil {
				return fmt.Errorf("generate CA: %w", err)
			}

			serverCert, err := ca.IssueServerCertificate(now)
			if err != nil {
				return fmt.Errorf("issue server certificate: %w", err)
			}

			pkiDir := filepath.Join(dataDir, "pki")
			if err := ca.Save(pkiDir); err != nil {
				return fmt.Errorf("save CA: %w", err)
			}

			defer func() {
				if initErr != nil {
					if err := ca.Remove(pkiDir); err != nil {
						initErr = errors.Join(
							initErr,
							fmt.Errorf("clean up CA: %w", err),
						)
					}
				}
			}()

			if err := serverCert.Save(pkiDir, "server"); err != nil {
				return fmt.Errorf("save server certificate: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", defaultDataDir, "Directory for persistent server data")
	return cmd
}
