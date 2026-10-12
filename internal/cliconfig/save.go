package cliconfig

import (
	"errors"
	"fmt"
	"os"

	"go.yaml.in/yaml/v4"
)

func Save(path string, cfg Config) (saveErr error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode CLI configuration: %w", err)
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create CLI configuration file: %w", err)
	}

	defer func() {
		if saveErr == nil {
			return
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			saveErr = errors.Join(
				saveErr,
				fmt.Errorf("remove incomplete CLI configuration file: %w", err),
			)
		}
	}()

	if _, err := file.Write(data); err != nil {
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("write CLI configuration: %w", err), closeErr)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close CLI configuration file: %w", err)
	}

	return nil
}
