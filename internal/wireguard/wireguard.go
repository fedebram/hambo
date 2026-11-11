package wireguard

import (
	"errors"
	"os"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Setup creates and brings up the wg0 interface. If it already exists, it does nothing.
func Setup(key wgtypes.Key) error {
	link := &netlink.Wireguard{Name: "wg0"}
	err := netlink.LinkAdd(link)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	c, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer c.Close()

	port := 51820

	err = c.ConfigureDevice("wg0", wgtypes.Config{
		PrivateKey: &key,
		ListenPort: &port,
	})
	if err != nil {
		return err
	}

	return netlink.LinkSetUp(link)
}
