package agent

import (
	"strings"
	"testing"
)

func TestToolStatus(t *testing.T) {
	r := NewToolRegistry()
	res, err := r.Call("tool_status", map[string]any{})
	if err != nil || !strings.Contains(res, "Distro:") || !strings.Contains(res, "rg:") || !strings.Contains(res, "nmap:") {
		t.Fatalf("tool_status all: %v %s", err, res)
	}
	res, err = r.Call("tool_status", map[string]any{"tool": "prefcli"})
	if err != nil || !strings.Contains(res, "fd:") || strings.Contains(res, "nmap:") {
		t.Fatalf("tool_status prefcli: %v %s", err, res)
	}
	res, err = r.Call("tool_status", map[string]any{"tool": "ripgrep"})
	if err != nil || !strings.Contains(res, "rg:") {
		t.Fatalf("tool_status rg: %v %s", err, res)
	}
	res, err = r.Call("tool_status", map[string]any{"tool": "unknown-widget"})
	if err != nil || !strings.Contains(res, "unknown tool") {
		t.Fatalf("tool_status unknown: %v %s", err, res)
	}
}

func TestInstallToolRequiresApproval(t *testing.T) {
	r := NewToolRegistry()
	res, err := r.Call("install_tool", map[string]any{"tool": "rg"})
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case strings.Contains(res, "approval required"):
	case strings.Contains(res, "Already available"):
	case strings.Contains(res, "unsupported or unknown distro"):
	default:
		t.Fatalf("unexpected: %s", res)
	}
}

func TestInstallToolMissingArg(t *testing.T) {
	r := NewToolRegistry()
	res, err := r.Call("install_tool", map[string]any{})
	if err != nil || !strings.Contains(res, "tool or tools is required") {
		t.Fatalf("missing arg: %v %s", err, res)
	}
}

func TestInstallToolUnknown(t *testing.T) {
	r := NewToolRegistry()
	res, err := r.Call("install_tool", map[string]any{"tool": "not-real"})
	if err != nil || !strings.Contains(res, "unknown tool") {
		t.Fatalf("unknown: %v %s", err, res)
	}
}

func TestInstallToolManualOnly(t *testing.T) {
	r := NewToolRegistry()
	res, err := r.Call("install_tool", map[string]any{"tool": "goban-cli"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "manual") && !strings.Contains(res, "Already available") {
		t.Fatalf("manual hint expected: %s", res)
	}
}

func TestInstallToolApproved(t *testing.T) {
	r := NewToolRegistry()
	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) { return true, ApprovalOnce })
	res, err := r.Call("install_tool", map[string]any{"tools": []any{"rg", "fd"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res, "approval required") {
		t.Fatalf("should have been approved: %s", res)
	}
	if strings.Contains(res, "unsupported or unknown distro") {
		t.Skip("host distro not detected; skipping live install path")
	}
}

func TestToOpenAIToolsIncludesInstallTools(t *testing.T) {
	r := NewToolRegistry()
	var foundStatus, foundInstall bool
	for _, tool := range r.ToOpenAITools() {
		if tool.Function.Name == "tool_status" {
			foundStatus = true
		}
		if tool.Function.Name == "install_tool" {
			foundInstall = true
		}
	}
	if !foundStatus || !foundInstall {
		t.Fatal("install tools missing from ToOpenAITools")
	}
}
