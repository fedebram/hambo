package pki

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func saveCertificate(dir, name string, certificate *x509.Certificate, privateKey ed25519.PrivateKey) (saveErr error) {
	certificatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificate.Raw,
	})

	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("encode %s private key: %w", name, err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyDER,
	})

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create certificate directory: %w", err)
	}

	certificatePath := filepath.Join(dir, name+".crt")
	certificateFile, err := os.OpenFile(certificatePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create certificate file %q: %w", certificatePath, err)
	}

	keyPath := filepath.Join(dir, name+".key")
	keyCreated := false

	defer func() {
		if saveErr == nil {
			return
		}

		remove := func(path string) {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				saveErr = errors.Join(saveErr, fmt.Errorf("remove incomplete certificate file %q: %w", path, err))
			}
		}

		if keyCreated {
			remove(keyPath)
		}
		remove(certificatePath)
	}()

	if _, err := certificateFile.Write(certificatePEM); err != nil {
		closeErr := certificateFile.Close()
		return errors.Join(fmt.Errorf("write certificate file %q: %w", certificatePath, err), closeErr)
	}
	if err := certificateFile.Close(); err != nil {
		return fmt.Errorf("close certificate file %q: %w", certificatePath, err)
	}

	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create private key file %q: %w", keyPath, err)
	}
	keyCreated = true

	if _, err := keyFile.Write(keyPEM); err != nil {
		closeErr := keyFile.Close()
		return errors.Join(fmt.Errorf("write private key file %q: %w", keyPath, err), closeErr)
	}
	if err := keyFile.Close(); err != nil {
		return fmt.Errorf("close private key file %q: %w", keyPath, err)
	}

	return nil
}

func removeCertificate(dir, name string) error {
	var removeErr error

	for _, extension := range []string{".crt", ".key"} {
		path := filepath.Join(dir, name+extension)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			removeErr = errors.Join(
				removeErr,
				fmt.Errorf("remove certificate file %q: %w", path, err),
			)
		}
	}

	return removeErr
}
