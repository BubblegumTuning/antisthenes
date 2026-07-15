package agent

import (
	"strings"
	"testing"
)

func TestToolRegistry_ApprovalTools(t *testing.T) {
	r := NewToolRegistry()

	res, err := r.Call("approve_tool", map[string]any{"command": "ls", "level": "once"})
	if err != nil || !strings.Contains(res, "Approved") {
		t.Errorf("approve_tool: %v %s", err, res)
	}

	res, err = r.Call("reset_approvals", map[string]any{})
	if err != nil || !strings.Contains(res, "reset") {
		t.Errorf("reset_approvals: %v %s", err, res)
	}
}
