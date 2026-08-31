package container

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"testing/synctest"
	"time"
)

type failingRuntime struct {
	err error
}

type networkAttacherFuncs struct {
	attach func(context.Context, string, string) (NetworkAttachment, error)
	detach func(context.Context, string, string) error
}

func (f networkAttacherFuncs) Attach(
	ctx context.Context,
	containerID string,
	netNSPath string,
) (NetworkAttachment, error) {
	if f.attach == nil {
		return NetworkAttachment{}, nil
	}
	return f.attach(ctx, containerID, netNSPath)
}

func (f networkAttacherFuncs) Detach(
	ctx context.Context,
	containerID string,
	netNSPath string,
) error {
	if f.detach == nil {
		return nil
	}
	return f.detach(ctx, containerID, netNSPath)
}

type startFailingRuntime struct {
	Runtime
	err error
}

type inspectRecordingRuntime struct {
	Runtime
	inspectCalls int
}

func (r *inspectRecordingRuntime) Inspect(ctx context.Context, id string) (RuntimeContainer, error) {
	r.inspectCalls++
	return r.Runtime.Inspect(ctx, id)
}

func (r startFailingRuntime) StartTask(context.Context, string) error {
	return r.err
}

func (r failingRuntime) CreateContainer(context.Context, string, string) error {
	return r.err
}

func (r failingRuntime) Inspect(context.Context, string) (RuntimeContainer, error) {
	panic("unexpected call to runtime.Inspect")
}

func (r failingRuntime) DeleteContainer(context.Context, string) error {
	panic("unexpected call to runtime.Delete")
}

func (r failingRuntime) CreateTask(context.Context, string) (RuntimeTask, error) {
	panic("unexpected call to runtime.CreateTask")
}

func (r failingRuntime) StartTask(context.Context, string) error {
	panic("unexpected call to runtime.StartTask")
}

func (r failingRuntime) StopTask(context.Context, string) error {
	panic("unexpected call to runtime.StopTask")
}

func (r failingRuntime) DeleteTask(context.Context, string) error {
	panic("unexpected call to runtime.DeleteTask")
}

type runtimeDeleteFunc func(id string) error

func (runtimeDeleteFunc) CreateContainer(context.Context, string, string) error {
	panic("unexpected call to runtime.Create")
}

func (runtimeDeleteFunc) Inspect(context.Context, string) (RuntimeContainer, error) {
	panic("unexpected call to runtime.Inspect")
}

func (f runtimeDeleteFunc) DeleteContainer(_ context.Context, id string) error {
	return f(id)
}

func (runtimeDeleteFunc) CreateTask(context.Context, string) (RuntimeTask, error) {
	panic("unexpected call to runtime.CreateTask")
}

func (runtimeDeleteFunc) StartTask(context.Context, string) error {
	panic("unexpected call to runtime.StartTask")
}

func (runtimeDeleteFunc) StopTask(context.Context, string) error {
	panic("unexpected call to runtime.StopTask")
}

func (runtimeDeleteFunc) DeleteTask(context.Context, string) error {
	panic("unexpected call to runtime.DeleteTask")
}

func TestWorkerHandlesCreatingContainer(t *testing.T) {
	store := NewMemoryStore()
	runtime := NewMemoryRuntime()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreating,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	network := networkAttacherFuncs{
		attach: func(context.Context, string, string) (NetworkAttachment, error) {
			t.Fatal("network attached while creating container")
			return NetworkAttachment{}, nil
		},
	}
	worker := newWorker(store, runtime, network, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	gotRuntime, err := runtime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	wantRuntime := RuntimeContainer{
		ID:    container.Name,
		Image: container.Image,
	}
	if gotRuntime != wantRuntime {
		t.Errorf("got runtime container %+v, want %+v", gotRuntime, wantRuntime)
	}

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}

	want := container
	want.State = StateCreated
	if got != want {
		t.Errorf("got stored container %+v, want %+v", got, want)
	}
}

func TestWorkerSetsUpNetworkBeforeStartingTask(t *testing.T) {
	store := NewMemoryStore()
	runtime := NewMemoryRuntime()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateStarting,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	if err := runtime.CreateContainer(t.Context(), container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	wantNetwork := NetworkAttachment{
		IP: netip.MustParseAddr("10.0.1.2"),
	}

	attachCalled := false
	network := networkAttacherFuncs{attach: func(ctx context.Context, containerID, netNSPath string) (NetworkAttachment, error) {
		attachCalled = true
		if containerID != container.Name {
			t.Fatalf("got container ID %q, want %q", containerID, container.Name)
		}

		gotRuntime, err := runtime.Inspect(ctx, containerID)
		if err != nil {
			t.Fatalf("unexpected runtime inspect error during network setup: %v", err)
		}
		if gotRuntime.Task == nil {
			t.Fatal("got nil runtime task during network setup, want created task")
		}
		if gotRuntime.Task.State != TaskStateCreated {
			t.Fatalf(
				"got task state %q during network setup, want %q",
				gotRuntime.Task.State,
				TaskStateCreated,
			)
		}
		if netNSPath == "" {
			t.Fatal("got empty network namespace path")
		}
		if netNSPath != gotRuntime.Task.NetNSPath {
			t.Errorf(
				"got network namespace path %q, want task path %q",
				netNSPath,
				gotRuntime.Task.NetNSPath,
			)
		}
		return wantNetwork, nil
	}}

	worker := newWorker(store, runtime, network, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}
	if !attachCalled {
		t.Fatal("network attach was not called")
	}

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}

	want := container
	want.State = StateRunning
	want.Network = wantNetwork

	if got != want {
		t.Errorf("got stored container %+v, want %+v", got, want)
	}
}

func TestWorkerHandlesStartingContainer(t *testing.T) {
	store := NewMemoryStore()
	runtime := NewMemoryRuntime()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateStarting,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	if err := runtime.CreateContainer(t.Context(), container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	worker := newWorker(store, runtime, networkAttacherFuncs{}, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	gotRuntime, err := runtime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	if gotRuntime.ID != container.Name {
		t.Errorf("got runtime container ID %q, want %q", gotRuntime.ID, container.Name)
	}
	if gotRuntime.Image != container.Image {
		t.Errorf("got runtime image %q, want %q", gotRuntime.Image, container.Image)
	}
	if gotRuntime.Task == nil {
		t.Fatal("got nil runtime task, want running task")
	}
	if gotRuntime.Task.State != TaskStateRunning {
		t.Errorf("got runtime task state %q, want %q", gotRuntime.Task.State, TaskStateRunning)
	}

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}
	want := container
	want.State = StateRunning
	if got != want {
		t.Errorf("got stored container %+v, want %+v", got, want)
	}
}

func TestWorkerCleansUpTaskWhenNetworkAttachFails(t *testing.T) {
	wantErr := errors.New("attach network")
	store := NewMemoryStore()
	runtime := NewMemoryRuntime()
	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateStarting,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	if err := runtime.CreateContainer(t.Context(), container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	var attachedNetNSPath string
	detachCalled := false
	network := networkAttacherFuncs{
		attach: func(_ context.Context, containerID, netNSPath string) (NetworkAttachment, error) {
			if containerID != container.Name {
				t.Errorf("got container ID %q, want %q", containerID, container.Name)
			}
			attachedNetNSPath = netNSPath
			return NetworkAttachment{}, wantErr
		},
		detach: func(ctx context.Context, containerID, netNSPath string) error {
			detachCalled = true
			if containerID != container.Name {
				t.Errorf("got container ID %q, want %q", containerID, container.Name)
			}
			if netNSPath == "" || netNSPath != attachedNetNSPath {
				t.Errorf("got network namespace path %q, want %q", netNSPath, attachedNetNSPath)
			}

			gotRuntime, err := runtime.Inspect(ctx, containerID)
			if err != nil {
				t.Fatalf("inspect runtime during detach: %v", err)
			}
			if gotRuntime.Task == nil || gotRuntime.Task.State != TaskStateCreated {
				t.Fatalf("got task %+v during detach, want created task", gotRuntime.Task)
			}
			return nil
		},
	}
	worker := newWorker(store, runtime, network, NewMemoryQueue())

	if err := worker.handle(t.Context(), container.Name); !errors.Is(err, wantErr) {
		t.Fatalf("got handle error %v, want %v", err, wantErr)
	}
	if !detachCalled {
		t.Fatal("network detach was not called")
	}

	gotRuntime, err := runtime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	if gotRuntime.Task != nil {
		t.Fatalf("got runtime task %+v, want nil after cleanup", gotRuntime.Task)
	}
}

func TestWorkerCleansUpNetworkAndTaskWhenTaskStartFails(t *testing.T) {
	wantErr := errors.New("start task")
	store := NewMemoryStore()
	memoryRuntime := NewMemoryRuntime()
	runtime := startFailingRuntime{Runtime: memoryRuntime, err: wantErr}
	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateStarting,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	if err := runtime.CreateContainer(t.Context(), container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	var attachedNetNSPath string
	detachCalled := false
	network := networkAttacherFuncs{
		attach: func(_ context.Context, _ string, netNSPath string) (NetworkAttachment, error) {
			attachedNetNSPath = netNSPath
			return NetworkAttachment{IP: netip.MustParseAddr("10.0.1.2")}, nil
		},
		detach: func(ctx context.Context, containerID, netNSPath string) error {
			detachCalled = true
			if netNSPath == "" || netNSPath != attachedNetNSPath {
				t.Errorf("got network namespace path %q, want %q", netNSPath, attachedNetNSPath)
			}

			gotRuntime, err := runtime.Inspect(ctx, containerID)
			if err != nil {
				t.Fatalf("inspect runtime during detach: %v", err)
			}
			if gotRuntime.Task == nil || gotRuntime.Task.State != TaskStateCreated {
				t.Fatalf("got task %+v during detach, want created task", gotRuntime.Task)
			}
			return nil
		},
	}
	worker := newWorker(store, runtime, network, NewMemoryQueue())

	if err := worker.handle(t.Context(), container.Name); !errors.Is(err, wantErr) {
		t.Fatalf("got handle error %v, want %v", err, wantErr)
	}
	if !detachCalled {
		t.Fatal("network detach was not called")
	}

	gotRuntime, err := runtime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	if gotRuntime.Task != nil {
		t.Fatalf("got runtime task %+v, want nil after cleanup", gotRuntime.Task)
	}
}

func TestWorkerRechecksRunningContainer(t *testing.T) {
	store := NewMemoryStore()
	memoryRuntime := NewMemoryRuntime()
	runtime := &inspectRecordingRuntime{Runtime: memoryRuntime}

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateRunning,
		Network: NetworkAttachment{
			IP: netip.MustParseAddr("10.0.1.2"),
		},
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	if err := memoryRuntime.CreateContainer(t.Context(), container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}
	if _, err := memoryRuntime.CreateTask(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}
	if err := memoryRuntime.StartTask(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected task start error: %v", err)
	}

	worker := newWorker(store, runtime, networkAttacherFuncs{
		detach: func(context.Context, string, string) error {
			t.Fatal("network detached while runtime task is running")
			return nil
		},
	}, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	if runtime.inspectCalls != 1 {
		t.Errorf("got %d runtime inspect calls, want 1", runtime.inspectCalls)
	}

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}
	if got != container {
		t.Errorf("got stored container %+v, want unchanged %+v", got, container)
	}

	gotRuntime, err := memoryRuntime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	if gotRuntime.Task == nil || gotRuntime.Task.State != TaskStateRunning {
		t.Errorf("got runtime task %+v, want running task", gotRuntime.Task)
	}
}

func TestWorkerCleansUpStoppedTaskForRunningContainer(t *testing.T) {
	store := NewMemoryStore()
	memoryRuntime := NewMemoryRuntime()
	runtime := &inspectRecordingRuntime{Runtime: memoryRuntime}

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateRunning,
		Network: NetworkAttachment{
			IP: netip.MustParseAddr("10.0.1.2"),
		},
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	if err := memoryRuntime.CreateContainer(t.Context(), container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}
	if _, err := memoryRuntime.CreateTask(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}
	if err := memoryRuntime.StartTask(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected task start error: %v", err)
	}
	if err := memoryRuntime.StopTask(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected task stop error: %v", err)
	}
	stoppedRuntimeContainer, err := memoryRuntime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected stopped runtime inspect error: %v", err)
	}
	if stoppedRuntimeContainer.Task == nil {
		t.Fatal("got nil stopped runtime task")
	}
	if stoppedRuntimeContainer.Task.ExitedAt.IsZero() {
		t.Fatal("got stopped runtime task exit time zero")
	}
	wantExit := ContainerExit{
		Code:     stoppedRuntimeContainer.Task.ExitCode,
		ExitedAt: stoppedRuntimeContainer.Task.ExitedAt,
	}

	detachCalled := false
	network := networkAttacherFuncs{
		detach: func(ctx context.Context, containerID, netNSPath string) error {
			detachCalled = true
			if containerID != container.Name {
				t.Errorf("got container ID %q, want %q", containerID, container.Name)
			}
			if netNSPath != "" {
				t.Errorf("got network namespace path %q, want empty path", netNSPath)
			}

			gotRuntime, err := memoryRuntime.Inspect(ctx, containerID)
			if err != nil {
				t.Fatalf("inspect runtime during detach: %v", err)
			}
			if gotRuntime.Task == nil || gotRuntime.Task.State != TaskStateStopped {
				t.Fatalf("got runtime task %+v during detach, want stopped task", gotRuntime.Task)
			}
			return nil
		},
	}

	worker := newWorker(store, runtime, network, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}
	if !detachCalled {
		t.Fatal("network detach was not called")
	}
	if runtime.inspectCalls != 1 {
		t.Errorf("got %d runtime inspect calls, want 1", runtime.inspectCalls)
	}

	gotRuntime, err := memoryRuntime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	wantRuntimeC := RuntimeContainer{
		ID:    container.Name,
		Image: container.Image,
	}
	if !reflect.DeepEqual(gotRuntime, wantRuntimeC) {
		t.Errorf("got runtime container %+v, want %+v", gotRuntime, wantRuntimeC)
	}

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}
	want := container
	want.State = StateStopped
	want.Network = NetworkAttachment{}
	want.Exit = wantExit
	if got != want {
		t.Errorf("got stored container %+v, want %+v", got, want)
	}
}

func TestWorkerHandlesStoppingContainer(t *testing.T) {
	store := NewMemoryStore()
	runtime := NewMemoryRuntime()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateStopping,
		Network: NetworkAttachment{
			IP: netip.MustParseAddr("10.0.1.2"),
		},
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	if err := runtime.CreateContainer(t.Context(), container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}
	if _, err := runtime.CreateTask(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}
	if err := runtime.StartTask(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected task start error: %v", err)
	}

	detachCalled := false
	var wantExit ContainerExit
	network := networkAttacherFuncs{detach: func(ctx context.Context, containerID, netNSPath string) error {
		detachCalled = true
		if containerID != container.Name {
			t.Errorf("got container ID %q, want %q", containerID, container.Name)
		}
		if netNSPath != "" {
			t.Errorf("got network namespace path %q, want empty path", netNSPath)
		}

		gotRuntime, err := runtime.Inspect(ctx, containerID)
		if err != nil {
			t.Fatalf("inspect runtime during detach: %v", err)
		}
		if gotRuntime.Task == nil {
			t.Fatal("task deleted before network detach")
		}
		if gotRuntime.Task.State != TaskStateStopped {
			t.Errorf("got task state %q during detach, want %q", gotRuntime.Task.State, TaskStateStopped)
		}
		if gotRuntime.Task.ExitedAt.IsZero() {
			t.Error("got task exit time zero during detach")
		}
		wantExit = ContainerExit{
			Code:     gotRuntime.Task.ExitCode,
			ExitedAt: gotRuntime.Task.ExitedAt,
		}
		return nil
	}}
	worker := newWorker(store, runtime, network, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}
	if !detachCalled {
		t.Fatal("network detach was not called")
	}

	gotRuntime, err := runtime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	wantRuntime := RuntimeContainer{
		ID:    container.Name,
		Image: container.Image,
	}
	if !reflect.DeepEqual(gotRuntime, wantRuntime) {
		t.Errorf("got runtime container %+v, want %+v", gotRuntime, wantRuntime)
	}

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}
	want := container
	want.State = StateStopped
	want.Network = NetworkAttachment{}
	want.Exit = wantExit
	if got != want {
		t.Errorf("got stored container %+v, want %+v", got, want)
	}
}

func TestWorkerHandlesDeletingContainer(t *testing.T) {
	store := NewMemoryStore()
	runtime := NewMemoryRuntime()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreated,
		DeletionTimestamp: time.Date(
			2026, time.July, 19, 15, 0, 0, 0, time.UTC,
		),
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	if err := runtime.CreateContainer(t.Context(), container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	worker := newWorker(store, runtime, networkAttacherFuncs{}, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	_, err := runtime.Inspect(t.Context(), container.Name)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got runtime inspect error %v, want %v", err, ErrNotFound)
	}

	_, err = store.Get(container.Name)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got store error %v, want %v", err, ErrNotFound)
	}
}

func TestWorkerMarksContainerDeletingBeforeRuntimeDeletion(t *testing.T) {
	store := NewMemoryStore()
	container := Container{
		Name:              "hello",
		State:             StateCreated,
		DeletionTimestamp: time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC),
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	deleteCalled := false
	runtime := runtimeDeleteFunc(func(name string) error {
		deleteCalled = true

		got, err := store.Get(name)
		if err != nil {
			t.Errorf("get container during runtime deletion: %v", err)
			return nil
		}
		if got.State != StateDeleting {
			t.Errorf("got state %q during runtime deletion, want %q", got.State, StateDeleting)
		}
		return nil
	})

	worker := newWorker(store, runtime, networkAttacherFuncs{}, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}
	if !deleteCalled {
		t.Fatal("runtime delete was not called")
	}
}

func TestWorkerHandlesNextQueuedContainer(t *testing.T) {
	store := NewMemoryStore()
	runtime := NewMemoryRuntime()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreating,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	queue := &recordingQueue{next: container.Name}

	worker := newWorker(store, runtime, networkAttacherFuncs{}, queue)
	if _, err := worker.handleNext(t.Context()); err != nil {
		t.Fatalf("unexpected handle next error: %v", err)
	}

	gotRuntime, err := runtime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	wantRuntime := RuntimeContainer{
		ID:    container.Name,
		Image: container.Image,
	}
	if gotRuntime != wantRuntime {
		t.Errorf("got runtime container %+v, want %+v", gotRuntime, wantRuntime)
	}

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}

	want := container
	want.State = StateCreated
	if got != want {
		t.Errorf("got stored container %+v, want %+v", got, want)
	}

	if queue.doneCalls != 1 {
		t.Fatalf("done calls: %d, want 1", queue.doneCalls)
	}
	if queue.doneName != container.Name {
		t.Fatalf("done name %q, want %q", queue.doneName, container.Name)
	}
}

func TestWorkerHandleNextRecordsAndReportsRuntimeFailure(t *testing.T) {
	wantErr := errors.New("runtime unavailable")
	store := NewMemoryStore()
	queue := NewMemoryQueue()
	defer queue.Shutdown()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreating,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	queue.Add(container.Name)

	worker := newWorker(store, failingRuntime{err: wantErr}, networkAttacherFuncs{}, queue)
	if _, err := worker.handleNext(t.Context()); !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}

	want := container
	want.Error = wantErr.Error()

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}
	if got != want {
		t.Errorf("got stored container %+v, want %+v", got, want)
	}
}

func TestWorkerHandleNextShutdown(t *testing.T) {
	queue := NewMemoryQueue()
	queue.Shutdown()

	worker := newWorker(NewMemoryStore(), NewMemoryRuntime(), networkAttacherFuncs{}, queue)

	shutdown, err := worker.handleNext(t.Context())
	if err != nil {
		t.Fatalf("unexpected handle next error: %v", err)
	}
	if !shutdown {
		t.Fatal("got shutdown false, want true")
	}
}

func TestWorkerRunHandlesQueuedContainers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := NewMemoryStore()
		runtime := NewMemoryRuntime()
		queue := NewMemoryQueue()

		worker := newWorker(store, runtime, networkAttacherFuncs{}, queue)

		errCh := make(chan error, 1)

		go func() {
			errCh <- worker.run(t.Context())
		}()

		deletionTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
		tests := []struct {
			container       Container
			runtimeExists   bool
			wantRuntime     bool
			wantStoredState State
		}{
			{
				container: Container{
					Name:  "hello",
					Image: "docker.io/library/alpine:latest",
					State: StateCreating,
				},
				wantRuntime:     true,
				wantStoredState: StateCreated,
			},
			{
				container: Container{
					Name:              "database",
					Image:             "docker.io/library/postgres:latest",
					State:             StateCreated,
					DeletionTimestamp: deletionTime,
				},
				runtimeExists: true,
			},
		}

		for _, tt := range tests {
			if err := store.Create(tt.container); err != nil {
				t.Fatalf("unexpected store create error: %v", err)
			}
			if tt.runtimeExists {
				if err := runtime.CreateContainer(t.Context(), tt.container.Name, tt.container.Image); err != nil {
					t.Fatalf("unexpected runtime create error: %v", err)
				}
			}
			queue.Add(tt.container.Name)
		}

		// wait on the worker to process all the containers
		synctest.Wait()

		if got := queue.len(); got != 0 {
			t.Fatalf("got queue length: %d, want 0", got)
		}

		// the queue is now empty, the worker must block and not exit.
		select {
		case err := <-errCh:
			t.Fatalf("worker returned before shutdown: %v", err)
		default:
		}

		for _, tt := range tests {
			_, err := runtime.Inspect(t.Context(), tt.container.Name)
			if err != nil && !errors.Is(err, ErrNotFound) {
				t.Fatalf("unexpected runtime inspect error: %v", err)
			}
			runtimeExists := err == nil
			if runtimeExists != tt.wantRuntime {
				t.Errorf(
					"container %q runtime existence: got %t, want %t",
					tt.container.Name,
					runtimeExists,
					tt.wantRuntime,
				)
			}

			got, err := store.Get(tt.container.Name)
			if tt.wantStoredState == "" {
				if !errors.Is(err, ErrNotFound) {
					t.Errorf(
						"container %q store error: got %v, want %v",
						tt.container.Name,
						err,
						ErrNotFound,
					)
				}
				continue
			}
			if err != nil {
				t.Fatalf("unexpected store get error: %v", err)
			}

			want := tt.container
			want.State = tt.wantStoredState
			if got != want {
				t.Errorf("got stored container %+v, want %+v", got, want)
			}
		}

		queue.Shutdown()

		synctest.Wait()

		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("unexpected worker run error: %v", err)
			}
		default:
			t.Fatal("worker did not return after shutdown")
		}
	})
}

func TestWorkerHandleNextDoesNotRequeueFailedWork(t *testing.T) {
	wantErr := errors.New("runtime unavailable")
	container := Container{Name: "hello", State: StateCreating}
	store := NewMemoryStore()
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	queue := &recordingQueue{next: container.Name}

	worker := newWorker(
		store,
		failingRuntime{err: wantErr},
		networkAttacherFuncs{},
		queue,
	)

	shutdown, err := worker.handleNext(t.Context())
	if shutdown {
		t.Fatal("got shutdown true, want false")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}

	if queue.addAfterCalls != 0 {
		t.Fatalf("add after calls: %d, want 0", queue.addAfterCalls)
	}

	if queue.doneCalls != 1 {
		t.Fatalf("done calls: %d, want 1", queue.doneCalls)
	}

	if queue.doneName != "hello" {
		t.Fatalf("done name %q, want %q", queue.doneName, "hello")
	}
}
