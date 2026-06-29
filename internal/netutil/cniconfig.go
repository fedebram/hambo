package netutil

import (
	"encoding/json"
	"fmt"
	"net"
)

// the types and part of the logic here are adapted from nerdctl's netutil
// package.
// TODO(license): Since it is Apache 2 we need to adhere to the license.
// For now, let's keep as it is. Once public we polish.

const (
	DefaultCNIVersion  = "1.0.0"
	DefaultNetworkName = "hambo"
	DefaultBridgeName  = "hambo0"
)

type CNIPlugin interface {
	GetPluginType() string
}

type cniNetworkConfig struct {
	CNIVersion string      `json:"cniVersion"`
	Name       string      `json:"name"`
	Plugins    []CNIPlugin `json:"plugins"`
}

type bridgeConfig struct {
	PluginType  string              `json:"type"`
	BrName      string              `json:"bridge,omitempty"`
	IsGW        bool                `json:"isGateway,omitempty"`
	IPMasq      bool                `json:"ipMasq,omitempty"`
	HairpinMode bool                `json:"hairpinMode,omitempty"`
	MTU         int                 `json:"mtu,omitempty"`
	IPAM        hostLocalIPAMConfig `json:"ipam"`
}

func (bridgeConfig) GetPluginType() string { return "bridge" }

type portMapConfig struct {
	PluginType   string          `json:"type"`
	Capabilities map[string]bool `json:"capabilities"`
}

func (portMapConfig) GetPluginType() string { return "portmap" }

type hostLocalIPAMConfig struct {
	PluginType string        `json:"type"`
	Routes     []IPAMRoute   `json:"routes,omitempty"`
	Ranges     [][]IPAMRange `json:"ranges,omitempty"`
}

type IPAMRange struct {
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway,omitempty"`
}

type IPAMRoute struct {
	Dst string `json:"dst,omitempty"`
}

func generateIPAM(subnet, gateway string) hostLocalIPAMConfig {
	return hostLocalIPAMConfig{
		PluginType: "host-local",

		// this way anything outside the local subnet goes through the gateway.
		Routes: []IPAMRoute{{Dst: "0.0.0.0/0"}},
		Ranges: [][]IPAMRange{
			{{Subnet: subnet, Gateway: gateway}},
		},
	}
}

func generateConfList(name string, plugins []CNIPlugin) ([]byte, error) {
	conf := cniNetworkConfig{
		CNIVersion: DefaultCNIVersion,
		Name:       name,
		Plugins:    plugins,
	}
	return json.MarshalIndent(conf, "", "  ")
}

// validateSubnet parses a subnet in cidr format like "10.0.1.0/24".
// ipv4 only.
func validateSubnet(subnet string) (*net.IPNet, error) {
	ip, cidr, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, fmt.Errorf("invalid subnet %q: %w", subnet, err)
	}

	if ip.To4() == nil {
		return nil, fmt.Errorf("subnet %q is not IPv4", subnet)
	}

	if !cidr.IP.Equal(ip) {
		return nil, fmt.Errorf("subnet %q must be the network address", subnet)
	}

	ones, _ := cidr.Mask.Size()

	// 30 ones means that there are 4 addresses to be used.
	// 2 addresses reserved for network and broadcast. So 2 usable host addresses:
	// 1 for the gateway and 1 for the container.
	if ones > 30 {
		return nil, fmt.Errorf("subnet %q is too small, must be /30 or larger", subnet)
	}

	return cidr, nil
}

// GatewayIP returns the first ip of the network (network address + 1),
// commonly used as the gateway. The cidr must be IPv4.
func GatewayIP(cidr *net.IPNet) (net.IP, error) {
	ip := cidr.IP.To4()
	if ip == nil {
		return nil, fmt.Errorf("gateway: %q is not IPv4", cidr.String())
	}

	gw := make(net.IP, len(ip))
	copy(gw, ip)
	gw[len(gw)-1]++
	return gw, nil
}

// DefaultConfList builds the default conflist to hand to cni: bridge, host-local ipam and portmap plugins.
// subnet must be an ipv4 network in cidr format like
// "10.0.1.0/24", the gateway is derived from it.
func DefaultConfList(subnet string) ([]byte, error) {
	cidr, err := validateSubnet(subnet)
	if err != nil {
		return nil, err
	}
	gwIP, err := GatewayIP(cidr)
	if err != nil {
		return nil, err
	}

	bridge := bridgeConfig{
		PluginType:  "bridge",
		BrName:      DefaultBridgeName,
		IsGW:        true,
		IPMasq:      true,
		HairpinMode: true,
		IPAM:        generateIPAM(cidr.String(), gwIP.String()),
	}
	portmap := portMapConfig{
		PluginType:   "portmap",
		Capabilities: map[string]bool{"portMappings": true},
	}
	return generateConfList(DefaultNetworkName, []CNIPlugin{bridge, portmap})
}
