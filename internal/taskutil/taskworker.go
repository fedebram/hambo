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

// General wait before retry.
const waitRetry = 5 * time.Second

type TaskWorker struct {
	client *containerd.Client
	store  *TaskStore
	name   string
}

type TaskRecord struct {
	Name   string   `json:"name"`
	Image  string   `json:"image"`
	Cmd    []string `json:"cmd,omitempty"`
	Seq    uint64   `json:"seq"`
	Delete bool     `json:"delete"`
}

func NewTaskWorker(client *containerd.Client, store *TaskStore, name string) *TaskWorker {
	return &TaskWorker{
		client: client,
		store:  store,
		name:   name,
	}
}

func (tw *TaskWorker) RunOnce(ctx context.Context, name, image string, cmd []string) (containerd.Task, <-chan containerd.ExitStatus, error) {
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
			containerd.WithNewSpec(oci.WithImageConfigArgs(crdImage, cmd)),
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
		// err check for race conditions, but in reality one task is handled at most with one task worker.
		if _, err := task.Delete(ctx); err != nil && !errdefs.IsNotFound(err) {
			return err
		}
	}

	log.Printf("%s deleting container", container.ID())
	if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
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

	if err := task.Kill(ctx, syscall.SIGTERM); err != nil &&
		// those errors checks are the same of containerd.WithProcessKill
		!errdefs.IsNotFound(err) && !errdefs.IsFailedPrecondition(err) {
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
		// fixed timeout for now...
	case <-time.After(10 * time.Second):
		log.Println("SIGTERM timeout, sending SIGKILL")
		if err := task.Kill(ctx, syscall.SIGKILL, containerd.WithKillAll); err != nil &&
			!errdefs.IsNotFound(err) && !errdefs.IsFailedPrecondition(err) {
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

func (tw *TaskWorker) Loop(ctx context.Context) uint64 {
	for {
		done, seq := tw.reconcile(ctx)
		if done {
			return seq
		}
	}
}

func (tw *TaskWorker) reconcile(ctx context.Context) (done bool, seq uint64) {
	subCh, cancelSub := tw.store.Sub(tw.name)
	defer cancelSub()

	rec, found, err := tw.store.Get(tw.name)
	if err != nil {
		log.Printf("loop: get %s: %v", tw.name, err)
		time.Sleep(waitRetry)
		return false, rec.Seq
	}

	if !found || rec.Delete {
		if err := tw.Delete(ctx, tw.name); err != nil {
			log.Printf("loop: delete %s: %v", tw.name, err)
			time.Sleep(waitRetry)
			return false, rec.Seq
		}
		return true, rec.Seq
	}

	_, exitStatusCh, err := tw.RunOnce(ctx, rec.Name, rec.Image, rec.Cmd)
	if err != nil {
		log.Printf("loop: run %s: %v", rec.Name, err)
		time.Sleep(waitRetry)
		return false, rec.Seq
	}

	select {
	case <-subCh:
		return false, rec.Seq
	case st := <-exitStatusCh:
		code, exitedAt, err := st.Result()
		if err != nil {
			log.Printf("loop: exit status %s: %v", tw.name, err)
			time.Sleep(waitRetry)
			return false, rec.Seq
		}
		log.Printf("task %s: exited with code %d at %s", tw.name, code, exitedAt)

		if err := tw.Delete(ctx, tw.name); err != nil {
			log.Printf("loop: delete %s: %v", tw.name, err)
			time.Sleep(waitRetry)
			return false, rec.Seq
		}
		return true, rec.Seq
	}
}
