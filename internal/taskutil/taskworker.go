package taskutil

import (
	"context"
	"fmt"
	"log"
	"syscall"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
)

type TaskWorker struct {
	client *containerd.Client
	store  *TaskStore
	name   string
}

type TaskRecord struct {
	Name   string `json:"name"`
	Image  string `json:"image"`
	Seq    uint64 `json:"seq"`
	Delete bool   `json:"delete"`
}

func NewTaskWorker(client *containerd.Client, store *TaskStore, name string) *TaskWorker {
	return &TaskWorker{
		client: client,
		store:  store,
		name:   name,
	}
}

func (tw *TaskWorker) RunOnce(ctx context.Context, name, image string) (containerd.Task, <-chan containerd.ExitStatus, error) {
	container, err := tw.client.LoadContainer(ctx, name)
	if errdefs.IsNotFound(err) {
		var crdImage containerd.Image
		crdImage, err = tw.client.Pull(ctx, image, containerd.WithPullUnpack)
		if err != nil {
			return nil, nil, err
		}

		container, err = tw.client.NewContainer(
			ctx,
			name,
			containerd.WithImage(crdImage),
			containerd.WithNewSnapshot(name+"-snapshot", crdImage),
			containerd.WithNewSpec(oci.WithImageConfig(crdImage)),
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
	case containerd.Unknown:
		err = fmt.Errorf("task %s in unknown state", task.ID())
	}
	if err != nil {
		return nil, nil, err
	}

	return task, exitStatusCh, nil
}

func (tw *TaskWorker) Delete(ctx context.Context, name string) error {
	container, err := tw.client.LoadContainer(ctx, name)
	if errdefs.IsNotFound(err) {
		log.Printf("%s container not found, nothing to delete", name)
		return nil
	}
	if err != nil {
		return err
	}

	task, err := container.Task(ctx, nil)
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}

	if task != nil {
		if err := stop(ctx, task); err != nil {
			return err
		}
		if _, err := task.Delete(ctx); err != nil {
			return err
		}
	}

	log.Printf("%s deleting container", container.ID())
	return container.Delete(ctx, containerd.WithSnapshotCleanup)
}

func stop(ctx context.Context, task containerd.Task) error {
	status, err := task.Status(ctx)
	if err != nil {
		return err
	}

	switch status.Status {
	case containerd.Created, containerd.Stopped:
		return nil
	case containerd.Unknown:
		return fmt.Errorf("task %s is in unknown state", task.ID())
	case containerd.Paused, containerd.Pausing:
		if err := task.Resume(ctx); err != nil {
			return err
		}
	}

	exitStatusC, err := task.Wait(ctx)
	if err != nil {
		return err
	}

	if err := task.Kill(ctx, syscall.SIGTERM); err != nil {
		return err
	}

	select {
	case st := <-exitStatusC:
		code, exitedAt, err := st.Result()
		if err != nil {
			return err
		}
		log.Printf("task %s: exited with code %d at %s", task.ID(), code, exitedAt)
		return nil
	case <-time.After(10 * time.Second):
		log.Println("SIGTERM timeout, sending SIGKILL")
		if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
			return err
		}
		st := <-exitStatusC
		code, exitedAt, err := st.Result()
		if err != nil {
			return err
		}
		log.Printf("task %s: exited with code %d at %s", task.ID(), code, exitedAt)
		return nil
	}
}
