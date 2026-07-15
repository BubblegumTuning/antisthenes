package agent

import (
	"strings"
	"testing"
)

func TestToolRegistry_GobanCreateTicket(t *testing.T) {
	r := NewToolRegistry()

	res, err := r.Call("goban_create_ticket", map[string]any{})
	if err != nil || !strings.Contains(res, "title is required") {
		t.Errorf("goban missing: %s", res)
	}

	_, _ = r.Call("goban_create_ticket", map[string]any{"title": "t", "description": "d"})
}
