package taskutil

import (
	"context"
	"fmt"
	"log"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
)

func RunOnce(ctx context.Context, client *containerd.Client) (containerd.Task, <-chan containerd.ExitStatus, error) {
	image, err := client.Pull(ctx, "docker.io/library/redis:alpine", containerd.WithPullUnpack)
	if err != nil {
		return nil, nil, err
	}

	container, err := client.LoadContainer(ctx, "redis-server")
	if errdefs.IsNotFound(err) {
		container, err = client.NewContainer(
			ctx,
			"redis-server",
			containerd.WithImage(image),
			containerd.WithNewSnapshot("redis-server-snapshot", image),
			containerd.WithNewSpec(oci.WithImageConfig(image)),
		)
	}
	if err != nil {
		return nil, nil, err
	}

	task, err := container.Task(ctx, nil)
	if errdefs.IsNotFound(err) {
		log.Println("Task not found, creating a new one")
		task, err = container.NewTask(ctx, cio.NullIO)
	}
	if err != nil {
		return nil, nil, err
	}

	status, err := task.Status(ctx)
	if err != nil {
		return nil, nil, err
	}

	if status.Status == containerd.Stopped {
		if _, err := task.Delete(ctx); err != nil && !errdefs.IsNotFound(err) {
			return nil, nil, err
		}
		if task, err = container.NewTask(ctx, cio.NullIO); err != nil {
			return nil, nil, err
		}
		// if new task succeeds then its status is created
		status.Status = containerd.Created
	}

	exitStatusCh, err := task.Wait(ctx)
	if err != nil {
		return nil, nil, err
	}

	switch status.Status {
	case containerd.Created:
		log.Println("Start task")
		err = task.Start(ctx)
	// if the task is in pausing state, calling resume what is going to happen?
	case containerd.Paused, containerd.Pausing:
		log.Println("Resume task")
		err = task.Resume(ctx)
	case containerd.Running:
	default:
		err = fmt.Errorf("task %s in unexpected state %q", task.ID(), status.Status)
	}
	if err != nil {
		return nil, nil, err
	}

	return task, exitStatusCh, nil
}
