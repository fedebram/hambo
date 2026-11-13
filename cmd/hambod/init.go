package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/fedebram/hambo/internal/noiseconn"
	"github.com/fedebram/hambo/internal/store"
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
	var cidr string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the server: keys and admin credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			return initServer(dataDir, addr, cidr)
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", "./data", "where to store the server's files")
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:6443", "the server address to write in the admin credentials")
	cmd.Flags().StringVar(&cidr, "cidr", "10.100.0.0/16", "the ip range of the cluster, each node gets a /24 of it")
	return cmd
}

// serverFile is the server's identity and cluster's ip range, stored in server.json
type serverFile struct {
	Private []byte       `json:"private"`
	Public  []byte       `json:"public"`
	CIDR    netip.Prefix `json:"cidr"`
}

func initServer(dataDir, addr, cidr string) error {
	prefix, err := parseCIDR(cidr)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}

	sf, err := writeServerFile(dataDir, prefix)
	if err != nil {
		return err
	}
	adminPublic, err := writeAdminCredentials(dataDir, addr, sf.Public)
	if err != nil {
		return err
	}
	if err := addAdmin(dataDir, adminPublic); err != nil {
		return err
	}

	fmt.Println("initialized", dataDir)
	return nil
}

// parseCIDR checks that s is an ipv4 range bigger than a /24, written
// from its first address, like 10.100.0.0/16.
func parseCIDR(s string) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid cidr %q, use something like 10.100.0.0/16", s)
	}
	if !p.Addr().Is4() {
		return netip.Prefix{}, fmt.Errorf("cidr %s: only ipv4 for now", s)
	}
	// each node takes a /24, so the cluster must fit at least two
	if p.Bits() >= 24 {
		return netip.Prefix{}, fmt.Errorf("cidr %s is too small: each node takes a /24, use a /23 or bigger, like /16", s)
	}
	if p != p.Masked() {
		return netip.Prefix{}, fmt.Errorf("cidr %s does not start at the range's first address, did you mean %s?", s, p.Masked())
	}
	return p, nil
}

func writeServerFile(dataDir string, cidr netip.Prefix) (serverFile, error) {
	key, err := noiseconn.GenerateKey()
	if err != nil {
		return serverFile{}, err
	}
	sf := serverFile{Private: key.Private, Public: key.Public, CIDR: cidr}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return serverFile{}, err
	}

	path := filepath.Join(dataDir, "server.json")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return serverFile{}, fmt.Errorf("%s exists", path)
	}
	if err != nil {
		return serverFile{}, err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return serverFile{}, err
	}
	return sf, nil
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

// addAdmin creates the sqlite db and puts the admin in it.
func addAdmin(dataDir string, adminPublic []byte) error {
	st, err := store.Open(filepath.Join(dataDir, "hambo.db"))
	if err != nil {
		return err
	}
	defer st.Close()

	return st.AddClient(context.Background(), store.Client{PublicKey: adminPublic, Name: "admin", Role: store.RoleAdmin})
}
