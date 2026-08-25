//go:build integration

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
	"github.com/fedebram/hambo/internal/container"
)

const (
	integrationNamespace   = "hambo-main-tests"
	integrationContainerID = "hambo-main-test"
	integrationImage       = "docker.io/library/alpine:latest"
)

func cleanupRuntimeContainer(t *testing.T, containerdClient *client.Client, id string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	crdContainer, err := containerdClient.LoadContainer(ctx, id)
	if errdefs.IsNotFound(err) {
		return
	}
	if err != nil {
		t.Errorf("cleanup load container %q: %v", id, err)
		return
	}

	task, err := crdContainer.Task(ctx, nil)
	switch {
	case err == nil:
		if _, err := task.Delete(ctx, client.WithProcessKill); err != nil && !errdefs.IsNotFound(err) {
			t.Errorf("cleanup task for container %q: %v", id, err)
		}
	case errdefs.IsNotFound(err):
	default:
		t.Errorf("cleanup load task for container %q: %v", id, err)
	}

	if err := crdContainer.Delete(ctx, client.WithSnapshotCleanup); err != nil && !errdefs.IsNotFound(err) {
		t.Errorf("cleanup container %q: %v", id, err)
	}
}

func getContainer(client *http.Client, baseURL, id string) (container.Container, int, error) {
	resp, err := client.Get(baseURL + "/containers/" + id)
	if err != nil {
		return container.Container{}, 0, err
	}

	body, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return container.Container{}, resp.StatusCode, err
	}

	if resp.StatusCode != http.StatusOK {
		return container.Container{}, resp.StatusCode, nil
	}

	var got container.Container
	if err := json.Unmarshal(body, &got); err != nil {
		return container.Container{}, resp.StatusCode, err
	}

	return got, resp.StatusCode, nil
}

func TestRunProcessesContainers(t *testing.T) {
	containerdClient, err := client.New(
		defaultContainerdAddress,
		client.WithDefaultNamespace(integrationNamespace),
	)
	if err != nil {
		t.Fatalf("create cleanup containerd client: %v", err)
	}
	t.Cleanup(func() {
		_ = containerdClient.Close()
	})

	cleanupRuntimeContainer(t, containerdClient, integrationContainerID)
	t.Cleanup(func() {
		cleanupRuntimeContainer(t, containerdClient, integrationContainerID)
	})

	ctx, cancel := context.WithCancel(context.Background())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	runErrC := make(chan error, 1)
	go func() {
		runErrC <- run(
			ctx,
			withListener(listener),
			withContainerdNamespace(integrationNamespace),
		)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-runErrC:
			if err != nil {
				t.Errorf("run returned an error: %v", err)
			}
		case <-time.After(30 * time.Second):
			t.Errorf("hambod did not stop during cleanup")
			_ = listener.Close()
		}
	})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	baseURL := "http://" + listener.Addr().String()
	createBody := strings.NewReader(fmt.Sprintf(
		`{"name":%q,"image":%q}`,
		integrationContainerID,
		integrationImage,
	))
	resp, err := httpClient.Post(baseURL+"/containers", "application/json", createBody)
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	_, drainErr := io.Copy(io.Discard, resp.Body)
	closeErr := resp.Body.Close()
	if err := errors.Join(drainErr, closeErr); err != nil {
		t.Fatalf("read create container response: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create container returned status %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	deadline := time.Now().Add(5 * time.Minute)
	for {
		got, status, err := getContainer(httpClient, baseURL, integrationContainerID)
		if err != nil {
			t.Fatalf("get container: %v", err)
		}
		if status != http.StatusOK {
			t.Fatalf("get container returned status %d, want %d", status, http.StatusOK)
		}
		if got.Error != "" {
			t.Fatalf("container creation failed: %s", got.Error)
		}
		if got.State == container.StateCreated {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("container state remained %q, want %q", got.State, container.StateCreated)
		}

		time.Sleep(100 * time.Millisecond)
	}

	req, err := http.NewRequest(http.MethodDelete, baseURL+"/containers/"+integrationContainerID, nil)
	if err != nil {
		t.Fatalf("create delete container request: %v", err)
	}
	resp, err = httpClient.Do(req)
	if err != nil {
		t.Fatalf("delete container: %v", err)
	}
	_, drainErr = io.Copy(io.Discard, resp.Body)
	closeErr = resp.Body.Close()
	if err := errors.Join(drainErr, closeErr); err != nil {
		t.Fatalf("read delete container response: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("delete container returned status %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	deadline = time.Now().Add(30 * time.Second)
	for {
		got, status, err := getContainer(httpClient, baseURL, integrationContainerID)
		if err != nil {
			t.Fatalf("get deleting container: %v", err)
		}
		if status == http.StatusNotFound {
			break
		}
		if status != http.StatusOK {
			t.Fatalf("get deleting container returned status %d", status)
		}
		if got.Error != "" {
			t.Fatalf("container deletion failed: %s", got.Error)
		}
		if time.Now().After(deadline) {
			t.Fatal("container was not deleted")
		}

		time.Sleep(100 * time.Millisecond)
	}
}
