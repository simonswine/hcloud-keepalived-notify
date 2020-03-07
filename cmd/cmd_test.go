package cmd

import (
	"net"
	"testing"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

func TestMatchFloatingIP(t *testing.T) {
	config.FloatingIPs = []net.IP{
		net.ParseIP("1.2.3.4"),
		net.ParseIP("2600::1"),
	}

	if exp, act := true, matchFloatingIP(&hcloud.FloatingIP{
		IP:   net.ParseIP("1.2.3.4"),
		Type: hcloud.FloatingIPTypeIPv4,
	}); exp != act {
		t.Errorf("unexpected result exp=%v act=%v", exp, act)
	}

	if exp, act := false, matchFloatingIP(&hcloud.FloatingIP{
		IP:   net.ParseIP("1.2.3.5"),
		Type: hcloud.FloatingIPTypeIPv4,
	}); exp != act {
		t.Errorf("unexpected result exp=%v act=%v", exp, act)
	}

	_, network1, err := net.ParseCIDR("2600::/64")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, network2, err := net.ParseCIDR("2700::/64")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if exp, act := true, matchFloatingIP(&hcloud.FloatingIP{
		Type:    hcloud.FloatingIPTypeIPv6,
		Network: network1,
	}); exp != act {
		t.Errorf("unexpected result exp=%v act=%v", exp, act)
	}

	if exp, act := false, matchFloatingIP(&hcloud.FloatingIP{
		Type:    hcloud.FloatingIPTypeIPv6,
		Network: network2,
	}); exp != act {
		t.Errorf("unexpected result exp=%v act=%v", exp, act)
	}
}
