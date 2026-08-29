package container

import (
	"context"
	"net/netip"
)

type NetworkAttachment struct {
	IP netip.Addr `json:"ip"`
}

type NetworkAttacher interface {
	Attach(ctx context.Context, containerID, netNSPath string) (NetworkAttachment, error)
	Detach(ctx context.Context, containerID, netNSPath string) error
}
