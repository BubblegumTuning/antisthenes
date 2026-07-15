package agent

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func testRegistryWithNmap() *ToolRegistry {
	r := NewToolRegistry()
	RegisterNmapTools(r, true)
	return r
}

func TestValidateNmapTarget(t *testing.T) {
	valid := []string{"127.0.0.1", "192.168.1.0/24", "scanme.nmap.org", "host.local"}
	for _, target := range valid {
		if err := validateNmapTarget(target); err != nil {
			t.Errorf("valid target %q: %v", target, err)
		}
	}
	invalid := []string{"", "0.0.0.0/0", "::/0", "host;rm -rf", "1.2.3.4|cat /etc/passwd"}
	for _, target := range invalid {
		if err := validateNmapTarget(target); err == nil {
			t.Errorf("expected invalid target %q", target)
		}
	}
}

func TestValidateNmapPorts(t *testing.T) {
	valid := []string{"22", "22,80,443", "1-1000", "80-443,8080"}
	for _, ports := range valid {
		if err := validateNmapPorts(ports); err != nil {
			t.Errorf("valid ports %q: %v", ports, err)
		}
	}
	invalid := []string{"", "abc", "70000", "80;", "1-70000"}
	for _, ports := range invalid {
		if err := validateNmapPorts(ports); err == nil {
			t.Errorf("expected invalid ports %q", ports)
		}
	}
}

func TestBuildNmapArgs(t *testing.T) {
	cases := []struct {
		scanType string
		target   string
		ports    string
		want     []string
	}{
		{"ping", "127.0.0.1", "", []string{"-sn", "127.0.0.1"}},
		{"quick", "10.0.0.1", "", []string{"-F", "10.0.0.1"}},
		{"ports", "10.0.0.1", "22,80", []string{"-sT", "-p", "22,80", "10.0.0.1"}},
		{"service", "10.0.0.1", "443", []string{"-sT", "-sV", "-p", "443", "10.0.0.1"}},
	}
	for _, tc := range cases {
		got, err := buildNmapArgs(tc.scanType, tc.target, tc.ports)
		if err != nil {
			t.Fatalf("%s: %v", tc.scanType, err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("%s: got %v want %v", tc.scanType, got, tc.want)
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Fatalf("%s: got %v want %v", tc.scanType, got, tc.want)
			}
		}
	}
}

func TestNmapScanMissingTarget(t *testing.T) {
	r := testRegistryWithNmap()
	res, err := r.Call("nmap_scan", map[string]any{})
	if err != nil || !strings.Contains(res, "target is required") {
		t.Fatalf("missing target: %v %s", err, res)
	}
}

func TestNmapScanBlocksInternetSweep(t *testing.T) {
	r := testRegistryWithNmap()
	res, err := r.Call("nmap_scan", map[string]any{"target": "0.0.0.0/0"})
	if err != nil || !strings.Contains(res, "entire internet") {
		t.Fatalf("block sweep: %v %s", err, res)
	}
}

func TestNmapScanMissingBinary(t *testing.T) {
	r := testRegistryWithNmap()
	orig := resolveNmapBinary
	defer func() { resolveNmapBinary = orig }()
	resolveNmapBinary = func() (string, error) { return "", fmt.Errorf("missing") } //nolint:reassign

	res, err := r.Call("nmap_scan", map[string]any{"target": "127.0.0.1"})
	if err != nil || !strings.Contains(res, "install_tool") {
		t.Fatalf("missing binary: %v %s", err, res)
	}
}

func TestNmapScanRequiresApproval(t *testing.T) {
	if _, err := exec.LookPath("nmap"); err != nil {
		t.Skip("nmap not installed")
	}
	r := testRegistryWithNmap()
	res, err := r.Call("nmap_scan", map[string]any{"target": "127.0.0.1", "scan_type": "ping"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "approval required") {
		t.Fatalf("expected approval gate: %s", res)
	}
}

func TestNmapScanApprovedLocalhost(t *testing.T) {
	if _, err := exec.LookPath("nmap"); err != nil {
		t.Skip("nmap not installed")
	}
	r := testRegistryWithNmap()
	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) { return true, ApprovalOnce })
	res, err := r.Call("nmap_scan", map[string]any{"target": "127.0.0.1", "scan_type": "ping", "timeout": 30})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "nmap_scan:") || !strings.Contains(res, "Log:") {
		t.Fatalf("unexpected scan result: %s", res)
	}
}
