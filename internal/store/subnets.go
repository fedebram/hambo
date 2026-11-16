package store

import (
	"context"
	"database/sql"
	"errors"
	"net/netip"
)

// ErrNoFreeSubnet means every /24 of the cluster range is taken.
var ErrNoFreeSubnet = errors.New("no free subnet in the cluster range")

// takenSubnets returns the subnets the workers already have.
func takenSubnets(ctx context.Context, tx *sql.Tx) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, "SELECT subnet FROM clients WHERE subnet IS NOT NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	taken := make(map[string]bool)
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		taken[s] = true
	}
	return taken, rows.Err()
}

// freeSubnet returns the first /24 of cidr that is not taken:
// 10.100.1.0/24, then 10.100.2.0/24, 10.100.3.0/24, ...
func freeSubnet(cidr netip.Prefix, taken map[string]bool) (netip.Prefix, error) {
	addr := nextSlice(cidr.Addr())
	for cidr.Contains(addr) {
		subnet := netip.PrefixFrom(addr, 24)
		if !taken[subnet.String()] {
			return subnet, nil
		}

		addr = nextSlice(addr)
	}
	return netip.Prefix{}, ErrNoFreeSubnet
}

// nextSlice returns the first address of the next /24
func nextSlice(addr netip.Addr) netip.Addr {
	b := addr.As4()
	b[3] = 255
	return netip.AddrFrom4(b).Next()
}

// Subnet returns the subnet of the worker with this public key.
func (s *Store) Subnet(ctx context.Context, pub []byte) (netip.Prefix, error) {
	var subnet string
	err := s.read.QueryRowContext(ctx,
		"SELECT subnet FROM clients WHERE public_key = ? AND role = ?",
		pub, RoleWorker).Scan(&subnet)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.ParsePrefix(subnet)
}
