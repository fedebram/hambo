package cni

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	cnilibrary "github.com/containernetworking/cni/libcni"
)

func TestEnsureDefaultConfigCreatesConfig(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cni")

	if err := EnsureDefaultConfig(dir); err != nil {
		t.Fatalf("EnsureDefaultConfig() error: %v", err)
	}

	path := filepath.Join(dir, defaultConfigFileName)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}

	if !bytes.Equal(got, []byte(defaultConfig)) {
		t.Errorf("generated config does not match defaultConfig\ngot:\n%s\nwant:\n%s", got, defaultConfig)
	}

	if _, err := cnilibrary.ConfListFromBytes(got); err != nil {
		t.Errorf("generated config is not a valid CNI conflist: %v", err)
	}
}

func TestEnsureDefaultConfigPreservesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, defaultConfigFileName)
	want := []byte(`{"rewrite":true}`)

	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	if err := EnsureDefaultConfig(dir); err != nil {
		t.Fatalf("unexpected ensure default config error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read existing config: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("existing config was changed\ngot: %s\nwant: %s", got, want)
	}
}
