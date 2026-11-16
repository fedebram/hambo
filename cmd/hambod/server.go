package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"

	"github.com/fedebram/hambo/internal/server"
	"github.com/fedebram/hambo/internal/store"
	"github.com/flynn/noise"
	"github.com/spf13/cobra"
)

func serverCmd() *cobra.Command {
	var dataDir string
	var listen string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(dataDir, listen)
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", "./data", "where the server's files are")
	cmd.Flags().StringVar(&listen, "listen", "127.0.0.1:6443", "the address to listen on")
	return cmd
}

func runServer(dataDir, listen string) error {
	sf, err := loadServerFile(dataDir)
	if err != nil {
		return err
	}
	key := noise.DHKey{Private: sf.Private, Public: sf.Public}
	fmt.Println("server public key:", base64.StdEncoding.EncodeToString(key.Public))
	fmt.Println("cluster cidr:", sf.CIDR)

	st, err := store.Open(filepath.Join(dataDir, "hambo.db"))
	if err != nil {
		return err
	}
	defer st.Close()

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	defer ln.Close()
	fmt.Println("listening on", ln.Addr())

	srv := server.New(key, sf.CIDR, st)
	return srv.Serve(ln)
}

// loadServerFile reads the server's key and cluster range from server.json.
func loadServerFile(dataDir string) (serverFile, error) {
	path := filepath.Join(dataDir, "server.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return serverFile{}, fmt.Errorf("not initialized: %s not found, run: hambod init", path)
	}
	if err != nil {
		return serverFile{}, err
	}

	var sf serverFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return serverFile{}, fmt.Errorf("%s: %w", path, err)
	}
	return sf, nil
}
