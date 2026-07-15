package agent

import (
	"strings"
	"testing"
)

func TestToolRegistry_AnsibleCheck(t *testing.T) {
	r := NewToolRegistry()
	res, err := r.Call("ansible_check", map[string]any{})
	if err != nil || !strings.Contains(res, "ansible_check:") {
		t.Errorf("ansible_check: %v %s", err, res)
	}
	if !strings.Contains(res, "install_tool") && !strings.Contains(res, "ansible available") {
		t.Errorf("ansible_check should mention install_tool or availability: %s", res)
	}
}
