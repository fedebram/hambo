package worker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func lockDataDir(dataDir string) (*os.File, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create worker data directory: %w", err)
	}

	file, err := os.OpenFile(
		filepath.Join(dataDir, "worker.lock"),
		os.O_CREATE|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		return nil, fmt.Errorf("open worker lock file: %w", err)
	}

	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, errors.New("worker data directory is in use")
		}
		return nil, fmt.Errorf("lock worker data directory: %w", err)
	}

	return file, nil
}
