//go:build integration

package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"

	"github.com/fedebram/hambo/internal/netutil"
)

// TestSetupAllocation checks that the ip returned by setup belongs to the
// configured subnet, is not the gateway ip and comes from the configured namespace
// and inside the correct iface(eth0).
func TestSetupAllocation(t *testing.T) {
	id := testID(t)
	ctx := context.Background()

	_, cidr, err := net.ParseCIDR(testSubnet)
	if err != nil {
		t.Fatalf("parse subnet %q: %v", testSubnet, err)
	}

	gw, err := netutil.GatewayIP(cidr)
	if err != nil {
		t.Fatalf("gateway for %q: %v", testSubnet, err)
	}

	nsPath, ip, err := testCNIEnv.Setup(ctx, id)
	if err != nil {
		t.Fatalf("setup(%s): %v", id, err)
	}
	t.Cleanup(func() {
		if err := testCNIEnv.Teardown(ctx, id, nsPath); err != nil {
			t.Errorf("teardown(%s): %v", id, err)
		}
	})

	if ip == nil {
		t.Fatal("Setup returned a nil IP")
	}
	if ip.To4() == nil {
		t.Fatalf("expected an IPv4 address, got %v", ip)
	}

	// check if the ip belongs to the subnet
	if !cidr.Contains(ip) {
		t.Errorf("ip %v is outside subnet %v", ip, cidr)

	}
	// check that the ip is not the gateway ip of the subnet
	if ip.Equal(gw) {
		t.Errorf("ip %v collides with the gateway %v", ip, gw)
	}

	// bind mounted namespace is present
	if _, err := os.Stat(nsPath); err != nil {
		t.Errorf("netns path %s: %v", nsPath, err)
	}

	// default cni config -> one iface called eth0.
	// not considering loopback.
	if got := ifaceIP(t, nsPath, "eth0"); !got.Equal(ip) {
		t.Errorf("eth0 inside netns has %v, Setup returned %v", got, ip)
	}
}

// TestConcurrentSetupTeardown runs many Setup/Teardown cycles concurrently
// checks that none return an error and that no network namespaces are left behind.
func TestConcurrentSetupTeardown(t *testing.T) {
	const workers = 20

	base := testID(t)

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	fail := func(err error) {
		mu.Lock()
		errs = append(errs, err)
		mu.Unlock()
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("%s-%d", base, i)
			ctx := context.Background()

			nsPath, _, err := testCNIEnv.Setup(ctx, id)
			if err != nil {
				fail(fmt.Errorf("Setup(%s): %w", id, err))
				return
			}

			if err := testCNIEnv.Teardown(ctx, id, nsPath); err != nil {
				fail(fmt.Errorf("Teardown(%s): %w", id, err))
			}
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		t.Error(err)
	}

	// all setups were followed by teardowns, the netns dir should be empty
	entries, err := os.ReadDir(netutil.NetnsBaseDir)
	if err != nil {
		t.Errorf("read netns dir: %v", err)
	} else if len(entries) > 0 {
		t.Errorf("netns dir not empty after teardown: %v", entries)
	}
}

// TestTeardownRemovesNetns checks that teardown removes the network namespace
// created by setup.
func TestTeardownRemovesNetns(t *testing.T) {
	id := testID(t)
	ctx := context.Background()

	nsPath, _, err := testCNIEnv.Setup(ctx, id)
	if err != nil {
		t.Fatalf("setup(%s): %v", id, err)
	}
	t.Cleanup(func() {
		// here indirectly we are testing the idempotency of teardown.
		if err := testCNIEnv.Teardown(ctx, id, nsPath); err != nil {
			t.Errorf("teardown(%s): %v", id, err)
		}
	})

	if _, err := os.Stat(nsPath); err != nil {
		t.Fatalf("netns should exist after Setup: %v", err)
	}

	if err := testCNIEnv.Teardown(ctx, id, nsPath); err != nil {
		t.Fatalf("Teardown: %v", err)
	}

	if _, err := os.Stat(nsPath); !os.IsNotExist(err) {
		t.Errorf("netns %s still present after Teardown (stat err = %v)", nsPath, err)
	}
}

// TestNoIPExhaustion checks that the teardown actually works and we release ips.
func TestNoIPExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: cycles through the whole IP pool")
	}

	// test subnet is "10.0.1.0/24".
	// with 300 setups and teardowns, ips from the pool must be reused.
	const cycles = 300

	base := testID(t)
	ctx := context.Background()

	for i := 0; i < cycles; i++ {
		id := fmt.Sprintf("%s-%d", base, i)

		nsPath, _, err := testCNIEnv.Setup(ctx, id)
		if err != nil {
			t.Fatalf("Setup at cycle %d: %v", i, err)
		}
		if err := testCNIEnv.Teardown(ctx, id, nsPath); err != nil {
			t.Fatalf("Teardown at cycle %d: %v", i, err)
		}
	}
	entries, err := os.ReadDir(netutil.NetnsBaseDir)
	if err != nil {
		t.Errorf("read netns dir: %v", err)
	} else if len(entries) > 0 {
		t.Errorf("netns dir not empty after all the cycles: %v", entries)
	}
}
