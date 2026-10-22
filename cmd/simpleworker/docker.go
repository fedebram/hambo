package main

import (
	"context"
	"fmt"
	"time"

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

func containerCreateOptions(c Container, imageEntrypoint []string) (client.ContainerCreateOptions, error) {
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
			Image:       c.Image,
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
		if len(imageEntrypoint) == 0 {
			return client.ContainerCreateOptions{},
				fmt.Errorf("command is cleared and the image has no entrypoint")
		}

		options.Config.Entrypoint = imageEntrypoint

	}

	return options, nil
}
