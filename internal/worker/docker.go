package worker

import (
	"context"
	"fmt"

	"github.com/fedebram/hambo/api"
	dockercontainer "github.com/moby/moby/api/types/container"
	dockerclient "github.com/moby/moby/client"
)

const containerIDLabel = "hambo.container.id"

func (w *Worker) listContainers(ctx context.Context) ([]dockercontainer.Summary, error) {
	filters := make(dockerclient.Filters)
	filters.Add("label", containerIDLabel)

	result, err := w.docker.ContainerList(ctx, dockerclient.ContainerListOptions{
		All:     true,
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("list Hambo containers: %w", err)
	}

	return result.Items, nil
}

func (w *Worker) pullImage(ctx context.Context, image string) error {
	pull, err := w.docker.ImagePull(
		ctx,
		image,
		dockerclient.ImagePullOptions{},
	)
	if err != nil {
		return fmt.Errorf("pull image %q: %w", image, err)
	}

	if err := pull.Wait(ctx); err != nil {
		return fmt.Errorf("pull image %q: %w", image, err)
	}

	return nil
}

func (w *Worker) createContainer(ctx context.Context, assignment api.ContainerAssignment) (string, error) {
	if err := w.pullImage(ctx, assignment.Image); err != nil {
		return "", err
	}

	created, err := w.docker.ContainerCreate(
		ctx,
		dockerclient.ContainerCreateOptions{
			Name: assignment.Name,
			Config: &dockercontainer.Config{
				Image: assignment.Image,
				Labels: map[string]string{
					containerIDLabel: assignment.ID,
				},
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf(
			"create container %q: %w",
			assignment.ID, err,
		)
	}

	return created.ID, nil
}

func (w *Worker) startContainer(ctx context.Context, dockerID string) error {
	_, err := w.docker.ContainerStart(
		ctx,
		dockerID,
		dockerclient.ContainerStartOptions{},
	)
	if err != nil {
		return fmt.Errorf("start Docker container %q: %w", dockerID, err)
	}

	return nil
}

func (w *Worker) waitContainer(ctx context.Context, dockerID string) (int64, error) {
	wait := w.docker.ContainerWait(
		ctx,
		dockerID,
		dockerclient.ContainerWaitOptions{
			Condition: dockercontainer.WaitConditionNotRunning,
		},
	)

	select {
	case result := <-wait.Result:
		if result.Error != nil {
			return 0, fmt.Errorf(
				"wait for Docker container %q: %s",
				dockerID, result.Error.Message,
			)
		}
		return result.StatusCode, nil

	case err := <-wait.Error:
		return 0, fmt.Errorf(
			"wait for Docker container %q: %w",
			dockerID, err,
		)
	}
}

func (w *Worker) inspectContainer(ctx context.Context, dockerID string) (dockercontainer.InspectResponse, error) {
	result, err := w.docker.ContainerInspect(
		ctx,
		dockerID,
		dockerclient.ContainerInspectOptions{},
	)
	if err != nil {
		return dockercontainer.InspectResponse{}, fmt.Errorf(
			"inspect Docker container %q: %w",
			dockerID, err,
		)
	}

	return result.Container, nil
}
