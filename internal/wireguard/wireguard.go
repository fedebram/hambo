package wireguard

import (
	"errors"
	"fmt"
	"os"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	iface = "wg0"
	port  = 51820
)

// NewKey creates a new wireguard keypair.
func NewKey() (private, public []byte, err error) {
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, nil, err
	}
	pub := key.PublicKey()
	return key[:], pub[:], nil
}

// Up creates wg0 if it doesn't exist.
func Up(private []byte) error {
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

	listen := port
	err = c.ConfigureDevice(iface, wgtypes.Config{
		PrivateKey: &key,
		ListenPort: &listen,
	})
	if err != nil {
		return err
	}

	return netlink.LinkSetUp(link)
}
