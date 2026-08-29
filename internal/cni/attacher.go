package cni

import (
	"context"
	"fmt"
	"net/netip"

	gocni "github.com/containerd/go-cni"
	"github.com/fedebram/hambo/internal/container"
)

const defaultInterfaceName = "eth0"

type Attacher struct {
	client gocni.CNI
}

func NewAttacher(pluginDir, pluginConfDir string) (*Attacher, error) {
	client, err := gocni.New(
		gocni.WithPluginDir([]string{pluginDir}),
		gocni.WithPluginConfDir(pluginConfDir),
	)
	if err != nil {
		return nil, fmt.Errorf("create CNI client: %w", err)
	}

	if err := client.Load(
		gocni.WithLoNetwork,
		gocni.WithDefaultConf,
	); err != nil {
		return nil, fmt.Errorf("load CNI configuration: %w", err)
	}

	return &Attacher{client: client}, nil
}

func (a *Attacher) Attach(
	ctx context.Context,
	containerID string,
	netNSPath string,
) (container.NetworkAttachment, error) {
	result, err := a.client.Setup(ctx, containerID, netNSPath)
	if err != nil {
		return container.NetworkAttachment{}, fmt.Errorf(
			"set up CNI network for container %q: %w",
			containerID,
			err,
		)
	}
	// defensive...
	if result == nil {
		return container.NetworkAttachment{}, fmt.Errorf(
			"set up CNI network for container %q: empty result",
			containerID,
		)
	}

	// go-cni uses the eth prefix by default, so the default network is attached as eth0.
	interfaceConfig, ok := result.Interfaces[defaultInterfaceName]
	if !ok || interfaceConfig == nil {
		return container.NetworkAttachment{}, fmt.Errorf(
			"set up CNI network for container %q: interface %q not found",
			containerID,
			defaultInterfaceName,
		)
	}

	for _, ipConfig := range interfaceConfig.IPConfigs {
		if ipConfig == nil {
			continue
		}

		ip, ok := netip.AddrFromSlice(ipConfig.IP)
		if !ok {
			continue
		}

		return container.NetworkAttachment{IP: ip.Unmap()}, nil
	}

	return container.NetworkAttachment{}, fmt.Errorf(
		"set up CNI network for container %q: interface %q has no valid IP address",
		containerID,
		defaultInterfaceName,
	)
}

func (a *Attacher) Detach(ctx context.Context, containerID, netNSPath string) error {
	if err := a.client.Remove(ctx, containerID, netNSPath); err != nil {
		return fmt.Errorf("remove CNI network for container %q: %w", containerID, err)
	}

	return nil
}
