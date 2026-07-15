package agent

import (
	"strings"
	"testing"
	"time"
)

func TestToolRegistry_DelegateTaskTool(t *testing.T) {
	r := NewToolRegistry()

	res, err := r.Call("delegate_task", map[string]any{})
	if err != nil || !strings.Contains(res, "goal is required") {
		t.Errorf("delegate missing goal: %s %v", res, err)
	}

	done := make(chan struct {
		res string
		err error
	}, 1)
	go func() {
		res, err := r.Call("delegate_task", map[string]any{"goal": "test delegate via tool"})
		done <- struct {
			res string
			err error
		}{res, err}
	}()
	select {
	case d := <-done:
		if d.err == nil && d.res == "" {
			t.Error("delegate_task returned empty without err")
		}
	case <-time.After(1200 * time.Millisecond):
	}
}
