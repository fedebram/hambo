package netutil

import (
	"net"
	"testing"

	"github.com/containernetworking/cni/libcni"
)

func TestValidateSubnet(t *testing.T) {
	tests := []struct {
		name     string
		subnet   string
		wantErr  bool
		wantCIDR string
	}{
		{name: "valid /24", subnet: "10.0.1.0/24", wantCIDR: "10.0.1.0/24"},
		{name: "valid /30", subnet: "10.0.1.0/30", wantCIDR: "10.0.1.0/30"},
		{name: "too small /31", subnet: "10.0.1.0/31", wantErr: true},
		{name: "too small /32", subnet: "10.0.1.0/32", wantErr: true},
		{name: "host address not network", subnet: "10.0.1.5/24", wantErr: true},
		{name: "ipv6 rejected", subnet: "fd00::/64", wantErr: true},
		{name: "missing prefix", subnet: "10.0.1.0", wantErr: true},
		{name: "garbage", subnet: "garbage", wantErr: true},
		{name: "empty", subnet: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cidr, err := validateSubnet(tc.subnet)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if cidr != nil {
					t.Errorf("expected nil cidr, got %v", cidr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cidr == nil {
				t.Fatal("expected non-nil cidr, got nil")
			}
			if got := cidr.String(); got != tc.wantCIDR {
				t.Errorf("cidr = %q, want %q", got, tc.wantCIDR)
			}
		})
	}
}

func TestGatewayIP(t *testing.T) {
	tests := []struct {
		name    string
		subnet  string
		wantGW  string
		wantErr bool
	}{
		{name: "/24", subnet: "10.0.1.0/24", wantGW: "10.0.1.1"},
		{name: "different subnet", subnet: "192.168.1.0/24", wantGW: "192.168.1.1"},

		// /25 means that inside the last octet, the leading bit is part of the mask.
		// the /24 is split into two halves, we pick the upper one .128 -> leading bit 1
		{name: "non-zero last octet", subnet: "10.0.1.128/25", wantGW: "10.0.1.129"},

		// reject ipv6
		{name: "ipv6", subnet: "fd00::/64", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, cidr, err := net.ParseCIDR(tc.subnet)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.subnet, err)
			}

			gw, err := GatewayIP(cidr)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got gateway %v", gw)
				}
				if gw != nil {
					t.Errorf("expected nil gateway, got %v", gw)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := gw.String(); got != tc.wantGW {
				t.Errorf("gateway = %q, want %q", got, tc.wantGW)
			}
		})
	}
}

// to see the specification cni.dev
const wantConfList = `{
  "cniVersion": "1.0.0",
  "name": "hambo",
  "plugins": [
    {
      "type": "bridge",
      "bridge": "hambo0",
      "isGateway": true,
      "ipMasq": true,
      "hairpinMode": true,
      "ipam": {
        "type": "host-local",
        "routes": [
          {
            "dst": "0.0.0.0/0"
          }
        ],
        "ranges": [
          [
            {
              "subnet": "10.0.1.0/24",
              "gateway": "10.0.1.1"
            }
          ]
        ]
      }
    },
    {
      "type": "portmap",
      "capabilities": {
        "portMappings": true
      }
    }
  ]
}`

func TestDefaultConfList(t *testing.T) {
	b, err := DefaultConfList("10.0.1.0/24")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := string(b); got != wantConfList {
		t.Errorf("conflist mismatch:\ngot:\n%s\nwant:\n%s", got, wantConfList)
	}

	// go-cni WithConfListBytes calls libcni ConfListFromBytes internally.
	// So we test if the default configuration is rejected when creating the cni instance.
	if _, err := libcni.ConfListFromBytes(b); err != nil {
		t.Fatalf("generated conflist rejected by CNI parser: %v", err)
	}
}

func TestDefaultConfListBadSubnet(t *testing.T) {
	b, err := DefaultConfList("10.0.1.5/24")
	if err == nil {
		t.Error("expected error, got nil")
	}
	if b != nil {
		t.Errorf("expected nil bytes, got %v", b)
	}
}
