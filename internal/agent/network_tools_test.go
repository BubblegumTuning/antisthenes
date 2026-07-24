package agent

import (
	"errors"
	"net"
	"strings"
	"testing"
)

func TestHexLEToIPv4(t *testing.T) {
	tests := []struct {
		hex  string
		want string
	}{
		{"0101A8C0", "192.168.1.1"},
		{"0100000A", "10.0.0.1"},
		{"FEA9FEA9", "169.254.169.254"},
	}
	for _, tt := range tests {
		got, err := hexLEToIPv4(tt.hex)
		if err != nil {
			t.Fatalf("hexLEToIPv4(%q): %v", tt.hex, err)
		}
		if got != tt.want {
			t.Errorf("hexLEToIPv4(%q) = %q, want %q", tt.hex, got, tt.want)
		}
	}
}

func TestParseDefaultGateways(t *testing.T) {
	data := `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	00000000	0101A8C0	0003	0	0	100	00000000	0	0	0
wlan0	00000000	C001A8C0	0003	0	0	600	00000000	0	0	0
`
	gw := parseDefaultGateways([]byte(data))
	if gw["eth0"] != "192.168.1.1" {
		t.Errorf("eth0 gateway = %q, want 192.168.1.1", gw["eth0"])
	}
	if gw["wlan0"] != "192.168.1.192" {
		t.Errorf("wlan0 gateway = %q, want 192.168.1.192", gw["wlan0"])
	}
}

func TestParseResolvConf(t *testing.T) {
	data := `# comment
nameserver 127.0.0.53
search example.com local
options edns0 trust-ad
nameserver 1.1.1.1
`
	info := parseResolvConf([]byte(data))
	if len(info.Nameservers) != 2 || info.Nameservers[0] != "127.0.0.53" {
		t.Fatalf("nameservers: %#v", info.Nameservers)
	}
	if len(info.Search) != 2 || info.Search[0] != "example.com" {
		t.Fatalf("search: %#v", info.Search)
	}
	if len(info.Options) != 2 {
		t.Fatalf("options: %#v", info.Options)
	}
}

func TestUsesSystemdStub(t *testing.T) {
	if !usesSystemdStub([]string{"127.0.0.53"}) {
		t.Error("expected stub detection")
	}
	if usesSystemdStub([]string{"8.8.8.8"}) {
		t.Error("expected no stub")
	}
}

func TestCollectDNS_SystemdUpstream(t *testing.T) {
	origRead := readFileFn
	t.Cleanup(func() { readFileFn = origRead })

	readFileFn = func(path string) ([]byte, error) {
		switch path {
		case resolvConfPath:
			return []byte("nameserver 127.0.0.53\n"), nil
		case systemdResolvPath:
			return []byte("nameserver 10.0.0.1\nnameserver 1.1.1.1\n"), nil
		default:
			return nil, errors.New("unexpected path")
		}
	}

	info := collectDNS()
	if info.Source != systemdResolvPath+" (systemd-resolved)" {
		t.Fatalf("source = %q", info.Source)
	}
	if len(info.Nameservers) != 2 || info.Nameservers[0] != "10.0.0.1" {
		t.Fatalf("nameservers: %#v", info.Nameservers)
	}
}

func TestCollectNetworkStatus_FilterAndFormat(t *testing.T) {
	origList := listInterfacesFn
	origRead := readFileFn
	origHost := hostnameFn
	t.Cleanup(func() {
		listInterfacesFn = origList
		readFileFn = origRead
		hostnameFn = origHost
	})

	hostnameFn = func() (string, error) { return "testhost", nil }
	readFileFn = func(path string) ([]byte, error) {
		switch path {
		case routeFilePath:
			return []byte("Iface\tDestination\tGateway\neth0\t00000000\t0101A8C0\n"), nil
		case resolvConfPath:
			return []byte("nameserver 8.8.8.8\nsearch lan\n"), nil
		default:
			return nil, errors.New("unexpected")
		}
	}
	listInterfacesFn = func() ([]net.Interface, error) {
		return []net.Interface{
			{
				Name:  "eth0",
				Flags: net.FlagUp | net.FlagBroadcast | net.FlagMulticast,
				MTU:   1500,
				HardwareAddr: func() net.HardwareAddr {
					m, _ := net.ParseMAC("52:54:00:ab:cd:ef")
					return m
				}(),
			},
			{
				Name:  "lo",
				Flags: net.FlagUp | net.FlagLoopback,
				MTU:   65536,
			},
		}, nil
	}

	snap := collectNetworkStatus(networkStatusOptions{Interface: "eth0"})
	out := formatNetworkStatus(snap)
	for _, want := range []string{
		"hostname: testhost",
		"eth0 (up, mtu=1500, mac=52:54:00:ab:cd:ef)",
		"default_gateway: 192.168.1.1",
		"nameservers: 8.8.8.8",
		"search: lan",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestRegisterNetworkToolsDisabled(t *testing.T) {
	r := NewToolRegistry()
	RegisterNetworkTools(r, false)
	if _, err := r.Call("network_status", nil); err == nil {
		t.Fatal("expected unknown tool when network status disabled")
	}
}

func TestRegisterNetworkToolsEnabled(t *testing.T) {
	r := NewToolRegistry()
	RegisterNetworkTools(r, true)
	res, err := r.Call("network_status", map[string]any{"include_loopback": true})
	if err != nil || res == "" {
		t.Fatalf("network_status registered: %v %s", err, res)
	}
	if !strings.Contains(res, "network_status:") {
		t.Fatalf("unexpected output: %s", res)
	}
}
