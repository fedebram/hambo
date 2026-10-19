package main

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fedebram/hambo/internal/cliconfig"
	"github.com/fedebram/hambo/internal/pki"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize server credentials and administrator CLI configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) (initErr error) {
			dataDir, err := cmd.Flags().GetString("data-dir")
			if err != nil {
				return err
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

			adminCert, err := ca.IssueAdminCertificate(now)
			if err != nil {
				return fmt.Errorf("issue admin certificate: %w", err)
			}

			adminKeyDER, err := x509.MarshalPKCS8PrivateKey(adminCert.PrivateKey)
			if err != nil {
				return fmt.Errorf("encode admin private key: %w", err)
			}

			cfg := cliconfig.Config{
				ServerURL: "https://localhost:8080",
				CACert: string(pem.EncodeToMemory(&pem.Block{
					Type:  "CERTIFICATE",
					Bytes: ca.Certificate.Raw,
				})),
				ClientCert: string(pem.EncodeToMemory(&pem.Block{
					Type:  "CERTIFICATE",
					Bytes: adminCert.Certificate.Raw,
				})),
				ClientKey: string(pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: adminKeyDER,
				})),
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

			defer func() {
				if initErr != nil {
					if err := serverCert.Remove(pkiDir, "server"); err != nil {
						initErr = errors.Join(
							initErr,
							fmt.Errorf("clean up server credentials: %w", err),
						)
					}
				}
			}()

			if err := cliconfig.Save(filepath.Join(dataDir, "admin.yaml"), cfg); err != nil {
				return fmt.Errorf("save admin configuration: %w", err)
			}
			return nil
		},
	}
	return cmd
}
