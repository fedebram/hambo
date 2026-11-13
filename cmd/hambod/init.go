package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fedebram/hambo/internal/noiseconn"
	"github.com/flynn/noise"
	"github.com/spf13/cobra"
)

// credentials let the CLI talk to the server.
type credentials struct {
	Server        string `json:"server"`
	ServerPublic  []byte `json:"server_public"`
	ClientPrivate []byte `json:"client_private"`
	ClientPublic  []byte `json:"client_public"`
}

func initCmd() *cobra.Command {
	var dataDir string
	var addr string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the server: keys and admin credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			return initServer(dataDir, addr)
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", "./data", "where to store the server's files")
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:6443", "the server address to write in the admin credentials")
	return cmd
}

// keyFile is how a keypair is stored on disk.
type keyFile struct {
	Private []byte `json:"private"`
	Public  []byte `json:"public"`
}

func initServer(dataDir, addr string) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}

	key, err := writeServerKey(dataDir)
	if err != nil {
		return err
	}
	adminPublic, err := writeAdminCredentials(dataDir, addr, key.Public)
	if err != nil {
		return err
	}
	if err := writeClientsFile(dataDir, adminPublic); err != nil {
		return err
	}

	fmt.Println("initialized", dataDir)
	return nil
}

func writeServerKey(dataDir string) (noise.DHKey, error) {
	key, err := noiseconn.GenerateKey()
	if err != nil {
		return noise.DHKey{}, err
	}
	data, err := json.MarshalIndent(keyFile{Private: key.Private, Public: key.Public}, "", "  ")
	if err != nil {
		return noise.DHKey{}, err
	}

	keyPath := filepath.Join(dataDir, "server.json")
	f, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return noise.DHKey{}, fmt.Errorf("%s exists", keyPath)
	}
	if err != nil {
		return noise.DHKey{}, err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return noise.DHKey{}, err
	}
	return key, nil
}

func writeAdminCredentials(dataDir, addr string, serverPublic []byte) ([]byte, error) {
	admin, err := noiseconn.GenerateKey()
	if err != nil {
		return nil, err
	}

	creds := credentials{
		Server:        addr,
		ServerPublic:  serverPublic,
		ClientPrivate: admin.Private,
		ClientPublic:  admin.Public,
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dataDir, "admin.json")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("already initialized: %s exists", path)
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return nil, err
	}
	fmt.Println("admin credentials:", path)
	return admin.Public, nil
}

func writeClientsFile(dataDir string, adminPublic []byte) error {
	list := clientsFile{
		Clients: []client{
			{Name: "admin", Public: adminPublic, Role: roleAdmin},
		},
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dataDir, "clients.json")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("already initialized: %s exists", path)
	}
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return err
	}
	return nil
}
