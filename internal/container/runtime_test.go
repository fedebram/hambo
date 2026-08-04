package container

import (
	"errors"
	"testing"
)

func TestInspectMissingContainer(t *testing.T) {
	runtime := NewMemoryRuntime()

	found, err := runtime.Inspect("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if found {
		t.Error("container found, want missing")
	}
}

func TestCreateAndInspectContainer(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.Create("hello"); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	found, err := runtime.Inspect("hello")
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}

	if !found {
		t.Error("container missing, want found")
	}
}

func TestCreateRejectsDuplicateContainer(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.Create("hello"); err != nil {
		t.Fatalf("unexpected first create error: %v", err)
	}

	err := runtime.Create("hello")

	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("got error %v, want %v", err, ErrAlreadyExists)
	}
}
