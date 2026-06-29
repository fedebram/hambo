package netutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/pkg/netns"
	gocni "github.com/containerd/go-cni"
)

const NetnsBaseDir = "/run/hambo/netns"

type CNIEnv struct {
	mu    sync.Mutex
	cni   gocni.CNI
	nsDir string
}

func NewCNIEnv(cni gocni.CNI) *CNIEnv {
	return &CNIEnv{cni: cni, nsDir: NetnsBaseDir}
}

func NewCNI(confList []byte, pluginDir string) (gocni.CNI, error) {
	return gocni.New(
		gocni.WithPluginDir([]string{pluginDir}),
		gocni.WithConfListBytes(confList),
		// WithLoNetwork must be called last, otherwise we mess up the network count
		gocni.WithLoNetwork,
	)
}

// Setup creates a new bind-mounted network namespace and sets up the network
// based on the cni configuration already provided.
//
// It returns the network namespace path and the ip assigned inside the namespace.
func (n *CNIEnv) Setup(ctx context.Context, id string) (path string, ip net.IP, err error) {
	// this is safe to run concurrently,
	// but it basically runs in a dedicated thread so it can be resource intensive.
	// this is not a problem, since we don't spawn thousands of goroutines all at once.
	ns, err := netns.NewNetNS(n.nsDir)
	if err != nil {
		return "", nil, fmt.Errorf("create netns: %w", err)
	}
	nsPath := ns.GetPath()

	// we lock here because cni plugins that run concurrently may lead to problems, even if one plugin is safe to run concurrently
	// the chain of plugins can lead to race conditions and other unpredictable behaviours.
	// Only one instance of hambo must run at any given time!
	n.mu.Lock()
	defer n.mu.Unlock()

	defer func() {
		if err != nil {
			if terr := n.teardownLocked(ctx, id, nsPath); terr != nil {
				err = fmt.Errorf("%w (cleanup failed: %v)", err, terr)
			}
		}
	}()

	res, err := n.cni.Setup(ctx, id, nsPath)
	if err != nil {
		return "", nil, fmt.Errorf("cni setup %s: %w", id, err)
	}

	ip = sandboxIP(res)
	if ip == nil {
		return "", nil, fmt.Errorf("no sandbox IP assigned for %s", id)
	}
	return nsPath, ip, nil
}

// Teardown removes the network that was previously setup by cni plugins, removes also the network namespace.
// Teardown is idempotent, an already deleted network doesn't return an error. So it is safe to be called multiple times
func (n *CNIEnv) Teardown(ctx context.Context, id, path string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.teardownLocked(ctx, id, path)
}

// teardownLocked performs the actual teardown. Caller must hold n.mu.
func (n *CNIEnv) teardownLocked(ctx context.Context, id, path string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	// we don't return on the first error. Even if cni remove fails
	// we still want to delete the net namespace.
	var errs []error
	if err := n.cni.Remove(ctx, id, path); err != nil {
		errs = append(errs, fmt.Errorf("cni remove %s: %w", id, err))
	}
	if err := netns.LoadNetNS(path).Remove(); err != nil {
		errs = append(errs, fmt.Errorf("netns remove %s: %w", id, err))
	}
	return errors.Join(errs...)
}

// sandboxIP retrieves the ip assigned to the sandbox.
func sandboxIP(res *gocni.Result) net.IP {
	// With the default cni config, only one network interface is created inside the sandbox (the network where the container lives).
	// That is eth0.
	// And only one ip is assigned to the iface.
	for _, cfg := range res.Interfaces {
		if cfg.Sandbox == "" {
			continue // host-side
		}
		for _, ipc := range cfg.IPConfigs {
			if ip := ipc.IP; ip != nil && !ip.IsLoopback() {
				return ip
			}
		}
	}
	return nil
}
