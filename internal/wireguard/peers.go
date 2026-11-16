package wireguard

import (
	"fmt"
	"net"
	"net/netip"
	"slices"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Peer is another node that wg0 talks to.
type Peer struct {
	PublicKey  []byte
	Endpoint   netip.AddrPort
	AllowedIPs []netip.Prefix
}

// SetPeers makes wg0 have exactly these peers. It changes only what is
// different: the peers that didn't change keep their tunnels running.
func SetPeers(peers []Peer) error {
	c, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer c.Close()

	dev, err := c.Device(iface)
	if err != nil {
		return err
	}
	// the peers wg0 has right now, by key
	current := make(map[wgtypes.Key]wgtypes.Peer)
	for _, p := range dev.Peers {
		current[p.PublicKey] = p
	}

	var changes []wgtypes.PeerConfig
	wanted := make(map[wgtypes.Key]bool)

	// we add the new peers and update the changed ones
	for _, p := range peers {
		cfg, err := peerConfig(p)
		if err != nil {
			return err
		}
		wanted[cfg.PublicKey] = true

		cur, ok := current[cfg.PublicKey]
		if ok && samePeer(cur, cfg) {
			continue
		}
		changes = append(changes, cfg)
	}

	// we remove the peers that are not in the list anymore
	for key := range current {
		if !wanted[key] {
			changes = append(changes, wgtypes.PeerConfig{PublicKey: key, Remove: true})
		}
	}

	if len(changes) == 0 {
		return nil
	}
	return c.ConfigureDevice(iface, wgtypes.Config{Peers: changes})
}

// peerConfig turns p into the config wgctrl wants.
func peerConfig(p Peer) (wgtypes.PeerConfig, error) {
	key, err := wgtypes.NewKey(p.PublicKey)
	if err != nil {
		return wgtypes.PeerConfig{}, fmt.Errorf("bad peer key: %w", err)
	}

	var allowed []net.IPNet
	for _, prefix := range p.AllowedIPs {
		allowed = append(allowed, net.IPNet{
			IP:   prefix.Addr().AsSlice(),
			Mask: net.CIDRMask(prefix.Bits(), prefix.Addr().BitLen()),
		})
	}

	return wgtypes.PeerConfig{
		PublicKey:         key,
		Endpoint:          net.UDPAddrFromAddrPort(p.Endpoint),
		ReplaceAllowedIPs: true,
		AllowedIPs:        allowed,
	}, nil
}

// samePeer reports whether the kernel's peer already has the endpoint and
// the allowed ips of cfg. We compare them as sorted text.
func samePeer(cur wgtypes.Peer, cfg wgtypes.PeerConfig) bool {
	if cur.Endpoint == nil || cur.Endpoint.String() != cfg.Endpoint.String() {
		return false
	}
	return slices.Equal(ipsAsText(cur.AllowedIPs), ipsAsText(cfg.AllowedIPs))
}

// ipsAsText returns the ranges as sorted text.
func ipsAsText(ips []net.IPNet) []string {
	var text []string
	for _, n := range ips {
		text = append(text, n.String())
	}
	slices.Sort(text)
	return text
}
