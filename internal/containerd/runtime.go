package containerd

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"github.com/fedebram/hambo/internal/container"
)

type Runtime struct {
	client          *client.Client
	stopGracePeriod time.Duration
}

func NewRuntime(client *client.Client, options ...RuntimeOption) *Runtime {
	runtime := &Runtime{
		client:          client,
		stopGracePeriod: defaultStopGracePeriod,
	}
	for _, option := range options {
		option(runtime)
	}

	return runtime
}

func (r *Runtime) Inspect(ctx context.Context, id string) (container.RuntimeContainer, error) {
	c, err := r.client.LoadContainer(ctx, id)
	if errdefs.IsNotFound(err) {
		return container.RuntimeContainer{}, fmt.Errorf(
			"inspect runtime container %q: %w",
			id,
			container.ErrNotFound,
		)
	}
	if err != nil {
		return container.RuntimeContainer{}, fmt.Errorf(
			"inspect runtime container %q: load container from containerd: %w",
			id,
			err,
		)
	}

	cInfo, err := c.Info(ctx, client.WithoutRefreshedMetadata)
	if err != nil {
		return container.RuntimeContainer{}, fmt.Errorf(
			"inspect runtime container %q: read containerd container info: %w",
			id,
			err,
		)
	}

	rc := container.RuntimeContainer{
		ID:    cInfo.ID,
		Image: cInfo.Image,
	}
	t, err := c.Task(ctx, nil)
	if errdefs.IsNotFound(err) {
		return rc, nil
	}
	if err != nil {
		return container.RuntimeContainer{}, fmt.Errorf(
			"inspect runtime container %q: load containerd task: %w",
			id,
			err,
		)
	}

	tStatus, err := t.Status(ctx)
	if err != nil {
		return container.RuntimeContainer{}, fmt.Errorf(
			"inspect runtime container %q: read containerd task status: %w",
			id,
			err,
		)
	}

	processStatus := tStatus.Status

	switch processStatus {
	case client.Created:
		pid := t.Pid()
		rc.Task = &container.RuntimeTask{
			PID:       pid,
			NetNSPath: networkNamespacePath(pid),
			State:     container.TaskStateCreated,
		}
	case client.Running:
		pid := t.Pid()
		rc.Task = &container.RuntimeTask{
			PID:       pid,
			NetNSPath: networkNamespacePath(pid),
			State:     container.TaskStateRunning,
		}
	case client.Stopped:
		rc.Task = &container.RuntimeTask{
			State: container.TaskStateStopped,
		}
	default:
		return container.RuntimeContainer{}, fmt.Errorf(
			"inspect runtime container %q: unsupported containerd task status %q",
			id,
			processStatus,
		)
	}

	return rc, nil
}

func (r *Runtime) CreateContainer(ctx context.Context, id, image string) error {
	_, err := r.client.LoadContainer(ctx, id)
	switch {
	case err == nil:
		return fmt.Errorf(
			"create runtime container %q: %w",
			id,
			container.ErrAlreadyExists,
		)
	case !errdefs.IsNotFound(err):
		return fmt.Errorf(
			"create runtime container %q: check whether container exists: %w",
			id,
			err,
		)
	}

	// TODO: pull semaphore needs to be done!!
	crdImage, err := r.client.Pull(ctx, image, client.WithPullUnpack)
	if err != nil {
		return fmt.Errorf(
			"create runtime container %q: pull and unpack image %q: %w",
			id,
			image,
			err,
		)
	}

	_, err = r.client.NewContainer(
		ctx,
		id,
		client.WithImage(crdImage),
		client.WithNewSnapshot(id+"-snapshot", crdImage),
		client.WithNewSpec(oci.WithImageConfig(crdImage)),
		client.WithImageStopSignal(crdImage, "SIGTERM"),
	)
	if err != nil {
		return fmt.Errorf(
			"create runtime container %q: create containerd container: %w",
			id,
			err,
		)
	}

	return nil
}

func (r *Runtime) DeleteContainer(ctx context.Context, id string) error {
	c, err := r.client.LoadContainer(ctx, id)
	if errdefs.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf(
			"delete runtime container %q: load container from containerd: %w",
			id,
			err,
		)
	}

	_, err = c.Task(ctx, nil)
	switch {
	case err == nil:
		return fmt.Errorf(
			"delete runtime container %q while its task exists: %w",
			id,
			container.ErrOperationNotAllowed,
		)
	case !errdefs.IsNotFound(err):
		return fmt.Errorf(
			"delete runtime container %q: check for existing containerd task: %w",
			id,
			err,
		)
	}

	if err := c.Delete(ctx, client.WithSnapshotCleanup); err != nil {
		return fmt.Errorf(
			"delete runtime container %q and clean up its snapshot: %w",
			id,
			err,
		)
	}

	return nil
}

func (r *Runtime) CreateTask(ctx context.Context, containerID string) (container.RuntimeTask, error) {
	c, err := r.client.LoadContainer(ctx, containerID)
	if errdefs.IsNotFound(err) {
		return container.RuntimeTask{}, fmt.Errorf(
			"create task for runtime container %q: container %w",
			containerID,
			container.ErrNotFound,
		)
	}
	if err != nil {
		return container.RuntimeTask{}, fmt.Errorf(
			"create task for runtime container %q: load container from containerd: %w",
			containerID,
			err,
		)
	}

	_, err = c.Task(ctx, nil)
	switch {
	case err == nil:
		return container.RuntimeTask{}, fmt.Errorf(
			"create task for runtime container %q: task %w",
			containerID,
			container.ErrAlreadyExists,
		)
	case !errdefs.IsNotFound(err):
		return container.RuntimeTask{}, fmt.Errorf(
			"create task for runtime container %q: check for existing containerd task: %w",
			containerID,
			err,
		)
	}

	task, err := c.NewTask(ctx, cio.NullIO)
	if errdefs.IsAlreadyExists(err) {
		return container.RuntimeTask{}, fmt.Errorf(
			"create task for runtime container %q: task %w",
			containerID,
			container.ErrAlreadyExists,
		)
	}
	if err != nil {
		return container.RuntimeTask{}, fmt.Errorf(
			"create task for runtime container %q: create containerd task: %w",
			containerID,
			err,
		)
	}

	pid := task.Pid()
	return container.RuntimeTask{
		PID:       pid,
		NetNSPath: networkNamespacePath(pid),
		State:     container.TaskStateCreated,
	}, nil
}

func networkNamespacePath(pid uint32) string {
	if pid == 0 {
		return ""
	}
	return fmt.Sprintf("/proc/%d/ns/net", pid)
}

func (r *Runtime) StartTask(ctx context.Context, containerID string) error {
	c, err := r.client.LoadContainer(ctx, containerID)
	if errdefs.IsNotFound(err) {
		return fmt.Errorf(
			"start task for runtime container %q: container %w",
			containerID,
			container.ErrNotFound,
		)
	}
	if err != nil {
		return fmt.Errorf(
			"start task for runtime container %q: load container from containerd: %w",
			containerID,
			err,
		)
	}

	t, err := c.Task(ctx, nil)
	if errdefs.IsNotFound(err) {
		return fmt.Errorf(
			"start task for runtime container %q: task %w",
			containerID,
			container.ErrNotFound,
		)
	}
	if err != nil {
		return fmt.Errorf(
			"start task for runtime container %q: load containerd task: %w",
			containerID,
			err,
		)
	}

	status, err := t.Status(ctx)
	if err != nil {
		return fmt.Errorf(
			"start task for runtime container %q: read containerd task status: %w",
			containerID,
			err,
		)
	}

	switch status.Status {
	case client.Created:
		if err := t.Start(ctx); err != nil {
			return fmt.Errorf(
				"start task for runtime container %q: start containerd task: %w",
				containerID,
				err,
			)
		}
		return nil
	case client.Running:
		return nil
	default:
		return fmt.Errorf(
			"start task for runtime container %q from state %q: %w",
			containerID,
			status.Status,
			container.ErrOperationNotAllowed,
		)
	}
}

func (r *Runtime) StopTask(ctx context.Context, containerID string) error {
	c, err := r.client.LoadContainer(ctx, containerID)
	if errdefs.IsNotFound(err) {
		return fmt.Errorf(
			"stop task for runtime container %q: container %w",
			containerID,
			container.ErrNotFound,
		)
	}
	if err != nil {
		return fmt.Errorf(
			"stop task for runtime container %q: load container from containerd: %w",
			containerID,
			err,
		)
	}

	t, err := c.Task(ctx, nil)
	if errdefs.IsNotFound(err) {
		return fmt.Errorf(
			"stop task for runtime container %q: task %w",
			containerID,
			container.ErrNotFound,
		)
	}
	if err != nil {
		return fmt.Errorf(
			"stop task for runtime container %q: load containerd task: %w",
			containerID,
			err,
		)
	}

	status, err := t.Status(ctx)
	if err != nil {
		return fmt.Errorf(
			"stop task for runtime container %q: read containerd task status: %w",
			containerID,
			err,
		)
	}

	switch status.Status {
	case client.Running:
		stopSignal, err := client.GetStopSignal(ctx, c, syscall.SIGTERM)
		if err != nil {
			return fmt.Errorf(
				"stop task for runtime container %q: read configured stop signal: %w",
				containerID,
				err,
			)
		}

		// we need a derived context because task.Wait starts a goroutine that blocks on an rpc call.
		// this way if the process has not exited and we return early, we can abort the rpc call.
		waitCtx, cancelWait := context.WithCancel(ctx)
		defer cancelWait()

		exitStatusC, err := t.Wait(waitCtx)
		if err != nil {
			return fmt.Errorf(
				"stop task for runtime container %q: wait for containerd task exit: %w",
				containerID,
				err,
			)
		}

		if err := t.Kill(ctx, stopSignal); err != nil {
			// if the process has already exited then a not found error is returned by the shim
			if !errdefs.IsNotFound(err) {
				return fmt.Errorf(
					"stop task for runtime container %q: send stop signal %s to containerd task: %w",
					containerID,
					stopSignal,
					err,
				)
			}
		}

		exited, err := waitForTaskExit(exitStatusC, r.stopGracePeriod)
		if err != nil {
			return fmt.Errorf(
				"stop task for runtime container %q: wait after stop signal %s: %w",
				containerID,
				stopSignal,
				err,
			)
		}
		if exited {
			return nil
		}

		if err := t.Kill(ctx, syscall.SIGKILL, client.WithKillAll); err != nil {
			if !errdefs.IsNotFound(err) {
				return fmt.Errorf(
					"stop task for runtime container %q: send SIGKILL to containerd task: %w",
					containerID,
					err,
				)
			}
		}

		exited, err = waitForTaskExit(exitStatusC, r.stopGracePeriod)
		if err != nil {
			return fmt.Errorf(
				"stop task for runtime container %q: wait after SIGKILL: %w",
				containerID,
				err,
			)
		}
		if !exited {
			return fmt.Errorf(
				"stop task for runtime container %q: task did not exit within %s after SIGKILL",
				containerID,
				r.stopGracePeriod,
			)
		}

		return nil
	case client.Stopped:
		return nil
	default:
		return fmt.Errorf(
			"stop task for runtime container %q from state %q: %w",
			containerID,
			status.Status,
			container.ErrOperationNotAllowed,
		)
	}
}

func waitForTaskExit(
	exitStatusC <-chan client.ExitStatus,
	timeout time.Duration,
) (bool, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case exitStatus := <-exitStatusC:
		if err := exitStatus.Error(); err != nil {
			return false, err
		}
		return true, nil
	case <-timer.C:
		return false, nil
	}
}

func (r *Runtime) DeleteTask(ctx context.Context, containerID string) error {
	c, err := r.client.LoadContainer(ctx, containerID)
	if errdefs.IsNotFound(err) {
		return fmt.Errorf(
			"delete task for runtime container %q: container %w",
			containerID,
			container.ErrNotFound,
		)
	}
	if err != nil {
		return fmt.Errorf(
			"delete task for runtime container %q: load container from containerd: %w",
			containerID,
			err,
		)
	}

	t, err := c.Task(ctx, nil)
	if errdefs.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf(
			"delete task for runtime container %q: load containerd task: %w",
			containerID,
			err,
		)
	}

	status, err := t.Status(ctx)
	if err != nil {
		return fmt.Errorf(
			"delete task for runtime container %q: read containerd task status: %w",
			containerID,
			err,
		)
	}

	switch status.Status {
	case client.Created:
		// task in created state needs to be first killed.
		if _, err := t.Delete(ctx, client.WithProcessKill); err != nil {
			return fmt.Errorf(
				"delete task for runtime container %q: force-delete created containerd task: %w",
				containerID,
				err,
			)
		}
		return nil
	case client.Stopped:
		if _, err := t.Delete(ctx); err != nil {
			return fmt.Errorf(
				"delete task for runtime container %q: delete stopped containerd task: %w",
				containerID,
				err,
			)
		}
		return nil
	case client.Running:
		return fmt.Errorf(
			"delete running task for runtime container %q: %w",
			containerID,
			container.ErrOperationNotAllowed,
		)
	default:
		return fmt.Errorf(
			"delete task for runtime container %q from state %q: %w",
			containerID,
			status.Status,
			container.ErrOperationNotAllowed,
		)
	}
}
