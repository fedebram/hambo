package worker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func Reset(dataDir string) error {
	lock, err := lockDataDir(dataDir)
	if err != nil {
		return err
	}
	defer lock.Close()

	path := filepath.Join(dataDir, "worker.json")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove worker state: %w", err)
	}
	return nil
}
