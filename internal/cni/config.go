package cni

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	defaultConfigFileName = "hambo.conflist"
	defaultNetworkName    = "hambo"
	defaultBridgeName     = "hambo0"
	defaultSubnet         = "10.0.1.0/24"
)

var defaultConfig = fmt.Sprintf(`{
  "cniVersion": "1.0.0",
  "name": %q,
  "plugins": [
    {
      "type": "bridge",
      "bridge": %q,
      "isGateway": true,
      "ipMasq": true,
	  "hairpinMode": true,
      "ipam": {
        "type": "host-local",
        "ranges": [
          [
            {
              "subnet": %q
            }
          ]
        ],
        "routes": [
          {
            "dst": "0.0.0.0/0"
          }
        ]
      }
    }
  ]
}
`, defaultNetworkName, defaultBridgeName, defaultSubnet)

func EnsureDefaultConfig(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create CNI config directory %q: %w", dir, err)
	}

	path := filepath.Join(dir, defaultConfigFileName)

	info, err := os.Stat(path)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("existing CNI config %q is not a regular file", path)
		}
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect CNI config %q: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(defaultConfig), 0o644); err != nil {
		return fmt.Errorf("create CNI config %q: %w", path, err)
	}

	return nil
}
