//go:build integration

package integration

import (
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/fedebram/hambo/internal/taskutil"
)

func TestRunOnce(t *testing.T) {
	// No container yet. RunOnce must create it and start the task.
	t.Run("JustCreate", func(t *testing.T) {
		e := newTestEnv(t)

		task, exitCh, err := e.tw.RunOnce(e.ctx, e.name, testImage, nil)
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if task == nil {
			t.Fatal("RunOnce returned a nil task")
		}
		if !e.containerExists() {
			t.Fatal("container should exist after RunOnce")
		}

		// busybox exits almost instantly with code 0 if there are no problems.
		// default commands is sh and we NullIO.
		if code := e.waitExit(exitCh, 15*time.Second); code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})

	// Container already exists but has no task.
	t.Run("ContainerExists", func(t *testing.T) {
		e := newTestEnv(t)
		e.createContainer()

		task, exitCh, err := e.tw.RunOnce(e.ctx, e.name, testImage, nil)
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if task == nil {
			t.Fatal("RunOnce returned a nil task")
		}
		if code := e.waitExit(exitCh, 15*time.Second); code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})

	// Task exists in Created state.
	t.Run("TaskCreated", func(t *testing.T) {
		e := newTestEnv(t)
		created := e.newTask(e.createContainer())
		e.waitStatus(created, containerd.Created, 5*time.Second)

		_, exitCh, err := e.tw.RunOnce(e.ctx, e.name, testImage, nil)
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if code := e.waitExit(exitCh, 15*time.Second); code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})

	// Task already Running. RunOnce is a no-op.
	t.Run("TaskRunning", func(t *testing.T) {
		e := newTestEnv(t)
		running := e.runningTask("sleep", "3600")
		e.waitStatus(running, containerd.Running, 5*time.Second)

		ret, _, err := e.tw.RunOnce(e.ctx, e.name, testImage, nil)
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if got := e.status(ret); got != containerd.Running {
			t.Errorf("status = %q, want running", got)
		}
	})

	// Task is Paused. RunOnce must resume it.
	// Task in a Pausing state can't be tested deterministically,
	// Probably task resume on a pausing state leads to an error.
	t.Run("TaskPaused", func(t *testing.T) {
		e := newTestEnv(t)
		task := e.runningTask("sleep", "3600")
		e.waitStatus(task, containerd.Running, 5*time.Second)

		if err := task.Pause(e.ctx); err != nil {
			t.Fatalf("Pause: %v", err)
		}
		e.waitStatus(task, containerd.Paused, 5*time.Second)

		ret, _, err := e.tw.RunOnce(e.ctx, e.name, testImage, nil)
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		e.waitStatus(ret, containerd.Running, 5*time.Second)
	})

	// Task has exited (Stopped state).
	// RunOnce must delete the dead task and start fresh with a new task.
	t.Run("TaskStopped", func(t *testing.T) {
		e := newTestEnv(t)
		stopped := e.runningTask()
		e.waitStatus(stopped, containerd.Stopped, 10*time.Second)
		oldPid := stopped.Pid()

		freshTask, exitCh, err := e.tw.RunOnce(e.ctx, e.name, testImage, nil)
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		// A fresh task means a new PID.
		if freshTask.Pid() == oldPid {
			t.Errorf("expected a new task, got the same pid %d", oldPid)
		}
		if code := e.waitExit(exitCh, 15*time.Second); code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})

	// A bad image makes RunOnce fail at Pull.
	t.Run("BadImage", func(t *testing.T) {
		e := newTestEnv(t)

		// we use localhost to fake registry and image.
		// This way we force pull image error.
		_, _, err := e.tw.RunOnce(e.ctx, e.name, "127.0.0.1:1/does-not-exist:latest", nil)
		if err == nil {
			t.Fatal("RunOnce with a bad image should return an error")
		}
	})

	t.Run("WithCmd", func(t *testing.T) {
		e := newTestEnv(t)

		cmd := []string{"sh", "-c", "exit 7"}
		_, exitCh, err := e.tw.RunOnce(e.ctx, e.name, testImage, cmd)
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if code := e.waitExit(exitCh, 15*time.Second); code != 7 {
			t.Errorf("exit code = %d, want 7", code)
		}
	})

	// Cmd only applies when RunOnce creates the container.
	// maybe we should refactor RunOnce and split the function?
	t.Run("CmdIgnoredOnExistingContainer", func(t *testing.T) {
		e := newTestEnv(t)
		e.createContainer()

		cmd := []string{"sh", "-c", "exit 7"}
		_, exitCh, err := e.tw.RunOnce(e.ctx, e.name, testImage, cmd)
		if err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if code := e.waitExit(exitCh, 15*time.Second); code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})
}

func TestDelete(t *testing.T) {
	// Deleting a container that doesn't exist is a no-op.
	t.Run("MissingContainer", func(t *testing.T) {
		e := newTestEnv(t)

		if err := e.tw.Delete(e.ctx, e.name); err != nil {
			t.Fatalf("Delete on missing container: %v", err)
		}
	})

	t.Run("Graceful", func(t *testing.T) {
		e := newTestEnv(t)
		// installing a term signal handler with trap inside busybox.
		task := e.runningTask("sh", "-c", `trap "exit 0" TERM; while true; do sleep 0.2; done`)
		e.waitStatus(task, containerd.Running, 5*time.Second)

		start := time.Now()
		if err := e.tw.Delete(e.ctx, e.name); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		// Graceful means it exits on SIGTERM.
		// TaskWorker delete has a fixed timeout of 10 seconds before sigkill.
		if elapsed := time.Since(start); elapsed >= 5*time.Second {
			t.Errorf("expected graceful SIGTERM exit, took %s (likely hit SIGKILL fallback)", elapsed)
		}
		if e.containerExists() {
			t.Fatal("container should be gone after Delete")
		}
	})

	// A process that ignores sigterm, gets sigkill after 10 seonds.
	t.Run("SigKill", func(t *testing.T) {
		e := newTestEnv(t)

		// trap "" TERM makes SIGTERM a no-op; SIGKILL can't be trapped.
		task := e.runningTask("sh", "-c", `trap "" TERM; while true; do sleep 1; done`)
		e.waitStatus(task, containerd.Running, 5*time.Second)

		start := time.Now()
		if err := e.tw.Delete(e.ctx, e.name); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if elapsed := time.Since(start); elapsed < 10*time.Second {
			t.Errorf("expected SIGKILL fallback after ~10s, returned in %s", elapsed)
		}
		if e.containerExists() {
			t.Fatal("container should be gone after Delete")
		}
	})

	t.Run("TaskStopped", func(t *testing.T) {
		e := newTestEnv(t)
		task := e.runningTask("sh", "-c", "exit 0")
		e.waitStatus(task, containerd.Stopped, 10*time.Second)

		if err := e.tw.Delete(e.ctx, e.name); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if e.containerExists() {
			t.Fatal("container should be gone after Delete")
		}
	})

	t.Run("TaskPaused", func(t *testing.T) {
		e := newTestEnv(t)

		task := e.runningTask("sh", "-c", `trap "exit 0" TERM; while true; do sleep 0.2; done`)
		e.waitStatus(task, containerd.Running, 5*time.Second)

		if err := task.Pause(e.ctx); err != nil {
			t.Fatalf("Pause: %v", err)
		}
		e.waitStatus(task, containerd.Paused, 5*time.Second)

		if err := e.tw.Delete(e.ctx, e.name); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if e.containerExists() {
			t.Fatal("container should be gone after Delete")
		}
	})

	// Delete skips the task handling if there is only the container,
	// and correctly delete the container.
	t.Run("ContainerOnly", func(t *testing.T) {
		e := newTestEnv(t)
		e.createContainer("sleep", "3600")

		if err := e.tw.Delete(e.ctx, e.name); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if e.containerExists() {
			t.Fatal("container should be gone after Delete")
		}
	})
}

func TestLoop(t *testing.T) {
	// here we see how it works in practice. Loop is decoupled from who writes the task record.
	t.Run("SimpleRun", func(t *testing.T) {
		e := newTestEnv(t)

		wantSeq, err := e.store.Put(taskutil.TaskRecord{Name: e.name, Image: testImage})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}

		seqCh := make(chan uint64, 1)
		go func() { seqCh <- e.tw.Loop(e.ctx) }()

		select {
		case gotSeq := <-seqCh:
			if gotSeq != wantSeq {
				t.Errorf("Loop returned seq %d, want %d", gotSeq, wantSeq)
			}
		case <-time.After(20 * time.Second):
			t.Fatal("Loop did not exit within 20 seconds")
		}

		if e.containerExists() {
			t.Fatal("container should be gone after Loop exits")
		}
	})

	// when a new write happens on the record, the loop reconciles.
	// we can also acknowledge that loop acted on the new "version" (sequence) of the record.
	t.Run("DeleteOnDeleteFlag", func(t *testing.T) {
		e := newTestEnv(t)

		cmd := []string{"sh", "-c", `trap "exit 0" TERM; while true; do sleep 0.2; done`}
		var putSeq uint64
		putSeq, err := e.store.Put(taskutil.TaskRecord{Name: e.name, Image: testImage, Cmd: cmd})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}

		seqCh := make(chan uint64, 1)
		go func() { seqCh <- e.tw.Loop(e.ctx) }()

		e.waitStatusByName(containerd.Running, 10*time.Second)

		wantSeq, err := e.store.Update(e.name, func(cur taskutil.TaskRecord, found bool) (taskutil.TaskRecord, bool) {
			cur.Delete = true
			return cur, true
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}

		select {
		case gotSeq := <-seqCh:
			// a bit redundant...
			if gotSeq == putSeq {
				t.Errorf("Loop not handled the second write")
			}
			if gotSeq != wantSeq {
				t.Errorf("Loop returned seq %d, want %d", gotSeq, wantSeq)
			}
		case <-time.After(20 * time.Second):
			t.Fatal("Loop did not tear down within 20s after Delete flag")
		}

		if e.containerExists() {
			t.Fatal("container should be gone after Delete flag")
		}
	})

	t.Run("EmptyStore", func(t *testing.T) {
		e := newTestEnv(t)

		seqCh := make(chan uint64, 1)
		go func() { seqCh <- e.tw.Loop(e.ctx) }()

		select {
		case gotSeq := <-seqCh:
			if gotSeq != 0 {
				t.Errorf("Loop returned seq %d, want 0", gotSeq)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Loop did not return on an empty store")
		}

		if e.containerExists() {
			t.Fatal("no container should have been created")
		}
	})
}
