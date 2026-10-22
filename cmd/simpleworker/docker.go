package main

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/errdefs"
	dockercontainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func newDockerClient(ctx context.Context) (*client.Client, error) {
	docker, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("create Docker client: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = docker.Ping(pingCtx, client.PingOptions{
		NegotiateAPIVersion: true,
	})
	cancel()
	if err != nil {
		docker.Close()
		return nil, fmt.Errorf("reach Docker: %w", err)
	}

	return docker, nil
}

func ensureImage(ctx context.Context, docker *client.Client, name string, policy PullPolicy) (client.ImageInspectResult, error) {
	switch policy {
	case PullNever:
		return docker.ImageInspect(ctx, name)
	case PullMissing:
		image, err := docker.ImageInspect(ctx, name)
		if err == nil {
			return image, nil
		}
		if !errdefs.IsNotFound(err) {
			return client.ImageInspectResult{}, fmt.Errorf("inspect image: %w", err)
		}
	case PullAlways:
	default:
		return client.ImageInspectResult{}, fmt.Errorf("unsupported pull policy %q", policy)
	}

	pull, err := docker.ImagePull(ctx, name, client.ImagePullOptions{})
	if err != nil {
		return client.ImageInspectResult{}, fmt.Errorf("pull image: %w", err)
	}
	if err := pull.Wait(ctx); err != nil {
		return client.ImageInspectResult{}, fmt.Errorf("pull image: %w", err)
	}

	return docker.ImageInspect(ctx, name)
}

func runContainer(ctx context.Context, docker *client.Client, c Container) (string, error) {
	image, err := ensureImage(ctx, docker, c.Image, c.PullPolicy)
	if err != nil {
		return "", err
	}

	options, err := containerCreateOptions(c, image)
	if err != nil {
		return "", err
	}

	created, err := docker.ContainerCreate(ctx, options)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	if _, err := docker.ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); err != nil {
		return created.ID, fmt.Errorf("start container: %w", err)
	}

	return created.ID, nil
}

func containerCreateOptions(c Container, image client.ImageInspectResult) (client.ContainerCreateOptions, error) {
	gracePeriod, err := time.ParseDuration(c.StopGracePeriod)
	if err != nil {
		return client.ContainerCreateOptions{}, fmt.Errorf("parse stop_grace_period: %w", err)
	}

	// docker takes the stop timeout in seconds
	stopTimeout := int(gracePeriod / time.Second)

	// docker takes the environment as KEY=value strings
	var environment []string
	for key, value := range c.Environment {
		environment = append(environment, key+"="+value)
	}

	options := client.ContainerCreateOptions{
		Name: c.Name,
		Config: &dockercontainer.Config{
			// we need to use the image id and not the image ref.
			Image:       image.ID,
			WorkingDir:  c.WorkingDir,
			Entrypoint:  c.Entrypoint,
			Cmd:         c.Command,
			Env:         environment,
			StopTimeout: &stopTimeout,
			Labels: map[string]string{
				"simpleworker.id": c.ID,
			},
		},
		HostConfig: &dockercontainer.HostConfig{
			RestartPolicy: dockercontainer.RestartPolicy{
				Name: dockercontainer.RestartPolicyMode(c.Restart),
			},
		},
	}

	// if the user provides an empty entrypoint slice, clear the image entrypoint
	if c.Entrypoint != nil && len(c.Entrypoint) == 0 {
		options.Config.Entrypoint = []string{""}
	}

	// if the user clears command, keep the image entrypoint explicitly
	// otherwise docker restores the image command.
	// see: https://github.com/moby/moby/blob/master/daemon/commit.go#L73-L76
	if c.Entrypoint == nil && c.Command != nil && len(c.Command) == 0 {
		if image.Config == nil || len(image.Config.Entrypoint) == 0 {
			return client.ContainerCreateOptions{},
				fmt.Errorf("command is cleared and the image has no entrypoint")
		}

		options.Config.Entrypoint = image.Config.Entrypoint
	}

	return options, nil
}

func inspectContainer(ctx context.Context, docker *client.Client, dockerID string) (ContainerStatus, error) {
	result, err := docker.ContainerInspect(ctx, dockerID, client.ContainerInspectOptions{})
	if errdefs.IsNotFound(err) {
		return ContainerStatus{
			DockerID: dockerID,
			State:    StateMissing,
		}, nil
	}
	if err != nil {
		return ContainerStatus{}, fmt.Errorf("inspect container: %w", err)
	}

	state := result.Container.State
	status := ContainerStatus{
		DockerID: result.Container.ID,
		State:    ContainerState(state.Status),
		Error:    state.Error,
	}

	// include the exit code only when the container has exited
	if status.State == StateExited {
		status.ExitCode = &state.ExitCode
	}

	return status, nil
}
