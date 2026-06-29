//go:build integration

package integration

import (
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/containerd/containerd/v2/pkg/netns"
	cnins "github.com/containernetworking/plugins/pkg/ns"
	"github.com/fedebram/hambo/internal/netutil"
)

const (
	pluginDir  = "/opt/cni/bin"
	testSubnet = "10.0.1.0/24"
)

var testCNIEnv *netutil.CNIEnv

func TestMain(m *testing.M) {
	entries, err := os.ReadDir(netutil.NetnsBaseDir)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "netns dir %s: %v\n", netutil.NetnsBaseDir, err)
			os.Exit(1)
		}
	} else if len(entries) > 0 {
		fmt.Fprintf(os.Stderr, "netns dir %s is not clean: %v\n", netutil.NetnsBaseDir, entries)
		os.Exit(1)
	}

	cflist, err := netutil.DefaultConfList(testSubnet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build conflist: %v\n", err)
		os.Exit(1)
	}
	cni, err := netutil.NewCNI(cflist, pluginDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new cni (are plugins in %s?): %v\n", pluginDir, err)
		os.Exit(1)
	}
	testCNIEnv = netutil.NewCNIEnv(cni)

	os.Exit(m.Run())
}

// cni id
func testID(t *testing.T) string {
	t.Helper()
	// cni id = container id
	return "net-" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "-")
}

// ifaceIP returns the ip configured inside the iface of the sandbox.
// ipv4 only.
func ifaceIP(t *testing.T, nsPath, iface string) net.IP {
	t.Helper()
	var ip net.IP
	err := netns.LoadNetNS(nsPath).Do(func(_ cnins.NetNS) error {
		i, err := net.InterfaceByName(iface)
		if err != nil {
			return err
		}
		addrs, err := i.Addrs()
		if err != nil {
			return err
		}
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLoopback() {
				ip = n.IP
				return nil
			}
		}
		return fmt.Errorf("no IPv4 on %s", iface)
	})
	if err != nil {
		t.Fatalf("read %s in %s: %v", iface, nsPath, err)
	}
	return ip
}
