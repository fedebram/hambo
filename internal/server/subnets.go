package server

import (
	"errors"
	"net/netip"
	"slices"
)

var errNoFreeSubnet = errors.New("no free subnet in the cluster range")

// freeSubnet returns the first /24 of cidr that is not taken, skipping
// the first one (reserved): 10.100.1.0/24, then 10.100.2.0/24, ...
func freeSubnet(cidr netip.Prefix, taken []string) (netip.Prefix, error) {
	addr := nextSlice(cidr.Addr())
	for cidr.Contains(addr) {
		subnet := netip.PrefixFrom(addr, 24)
		if !slices.Contains(taken, subnet.String()) {
			return subnet, nil
		}
		addr = nextSlice(addr)
	}
	return netip.Prefix{}, errNoFreeSubnet
}

// nextSlice returns the first address of the next /24.
func nextSlice(addr netip.Addr) netip.Addr {
	b := addr.As4()
	b[3] = 255
	return netip.AddrFrom4(b).Next()
}
