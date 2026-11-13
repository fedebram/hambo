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
// 10.100.0.0/24, then 10.100.1.0/24, 10.100.2.0/24, ...
func freeSubnet(cidr netip.Prefix, taken map[string]bool) (netip.Prefix, error) {
	addr := cidr.Addr() // eg. 10.100.0.0
	for cidr.Contains(addr) {
		subnet := netip.PrefixFrom(addr, 24)
		if !taken[subnet.String()] {
			return subnet, nil
		}

		// jump to the next /24: 10.100.0.0 → 10.100.0.255 → 10.100.1.0
		b := addr.As4()
		b[3] = 255
		addr = netip.AddrFrom4(b).Next()
	}
	return netip.Prefix{}, ErrNoFreeSubnet
}
