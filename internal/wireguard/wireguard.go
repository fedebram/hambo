package wireguard

import (
	"errors"
	"fmt"
	"net/netip"
	"os"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const iface = "wg0"

// NewKey creates a new wireguard keypair.
func NewKey() (private, public []byte, err error) {
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, nil, err
	}
	pub := key.PublicKey()
	return key[:], pub[:], nil
}

// SetInterface makes sure wg0 exists, with this private key and port,
// and is up. It changes only what is different.
func SetInterface(private []byte, port int) error {
	key, err := wgtypes.NewKey(private)
	if err != nil {
		return fmt.Errorf("bad wireguard key: %w", err)
	}

	link := &netlink.Wireguard{Name: iface}
	err = netlink.LinkAdd(link)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	c, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer c.Close()

	err = c.ConfigureDevice(iface, wgtypes.Config{
		PrivateKey: &key,
		ListenPort: &port,
	})
	if err != nil {
		return err
	}

	return netlink.LinkSetUp(link)
}

// SetAddress makes wg0 have exactly one address: the first address of the
// node's subnet, with the size of the cluster.
//
// eg. 10.100.1.0/24 in 10.100.0.0/16 gives 10.100.1.0/16.
func SetAddress(subnet, cluster netip.Prefix) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return err
	}
	want, err := netlink.ParseAddr(netip.PrefixFrom(subnet.Addr(), cluster.Bits()).String())
	if err != nil {
		return err
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return err
	}
	// we have already set the address
	if len(addrs) == 1 && addrs[0].Equal(*want) {
		return nil
	}

	// remove the old addresses: for example the node rejoined the cluster
	// and got a different subnet, but wg0 still has the old address.
	for _, a := range addrs {
		if a.Equal(*want) {
			continue
		}
		if err := netlink.AddrDel(link, &a); err != nil {
			return err
		}
	}

	// replace because it is idempotent:
	// if the address is already set, it won't fail.
	return netlink.AddrReplace(link, want)
}
