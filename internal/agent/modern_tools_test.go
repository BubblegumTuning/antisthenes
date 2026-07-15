package agent

import (
	"os"
	"strings"
	"testing"
)

func TestModernCLIStatus(t *testing.T) {
	r := NewToolRegistry()
	res, err := r.Call("modern_cli_status", map[string]any{})
	if err != nil || !strings.Contains(res, "deprecated") || !strings.Contains(res, "Distro:") {
		t.Fatalf("status: %v %s", err, res)
	}
	if strings.Contains(res, "  rg:") || strings.Contains(res, "  nmap:") {
		t.Fatalf("prefcli shim should not list rg/nmap: %s", res)
	}
	if !strings.Contains(res, "  fd:") {
		t.Fatalf("prefcli shim should list fd: %s", res)
	}
}

func TestInstallModernCLIShim(t *testing.T) {
	r := NewToolRegistry()
	res, err := r.Call("install_modern_cli", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "deprecated") {
		t.Fatalf("expected deprecation note: %s", res)
	}
	switch {
	case strings.Contains(res, "approval required"):
	case strings.Contains(res, "already available"):
	case strings.Contains(res, "All preferred CLI tools are already available"):
	case strings.Contains(res, "unsupported or unknown distro"):
	default:
		t.Fatalf("unexpected shim result: %s", res)
	}
}

func TestToolRegistry_CdPath(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	sub := "subdir"
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}

	r := NewToolRegistry()
	res, err := r.Call("cd_path", map[string]any{"path": sub})
	if err != nil {
		t.Fatalf("cd_path: %v", err)
	}
	if !strings.Contains(res, "now in") {
		t.Errorf("unexpected cd_path result: %s", res)
	}
	cwd, _ := os.Getwd()
	if !strings.HasSuffix(cwd, sub) {
		t.Errorf("cwd not updated: %s", cwd)
	}
}

func TestFindFilesFallback(t *testing.T) {
	r := NewToolRegistry()
	res, err := r.Call("find_files", map[string]any{"path": "."})
	if err != nil {
		t.Fatalf("find_files: %v", err)
	}
	if !strings.Contains(res, "find_files") {
		t.Fatalf("unexpected: %s", res)
	}
}

func TestRequestInteractiveApproval(t *testing.T) {
	r := NewToolRegistry()
	ok, denied := r.requestInteractiveApproval("install_modern_cli", "apt-get install -y fd")
	if ok || denied {
		t.Fatal("should require handler")
	}
	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) { return true, ApprovalOnce })
	ok, denied = r.requestInteractiveApproval("install_modern_cli", "apt-get install -y fd")
	if !ok || denied {
		t.Fatal("handler should approve")
	}
}
