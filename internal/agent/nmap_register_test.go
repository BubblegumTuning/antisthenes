package agent

import (
	"testing"
)

func TestRegisterNmapToolsDisabled(t *testing.T) {
	r := NewToolRegistry()
	RegisterNmapTools(r, false)
	if _, err := r.Call("nmap_scan", map[string]any{"target": "127.0.0.1"}); err == nil {
		t.Fatal("expected unknown tool when nmap disabled")
	}
}

func TestRegisterNmapToolsEnabled(t *testing.T) {
	r := NewToolRegistry()
	RegisterNmapTools(r, true)
	res, err := r.Call("nmap_scan", map[string]any{})
	if err != nil || res == "" {
		t.Fatalf("nmap_scan registered: %v %s", err, res)
	}
}
