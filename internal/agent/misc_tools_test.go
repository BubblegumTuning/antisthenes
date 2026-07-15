package agent

import (
	"strings"
	"testing"
)

func TestToolRegistry_MiscTools(t *testing.T) {
	r := NewToolRegistry()

	res, err := r.Call("get_current_time", map[string]any{})
	if err != nil || res == "" {
		t.Errorf("get_current_time: %v %s", err, res)
	}

	res, err = r.Call("echo", map[string]any{"message": "hello misc"})
	if err != nil || res != "hello misc" {
		t.Errorf("echo: %v %s", err, res)
	}

	res, err = r.Call("echo", map[string]any{})
	if err != nil || !strings.Contains(res, "no message") {
		t.Errorf("echo no msg: %s", res)
	}

	res, err = r.Call("get_env", map[string]any{"key": "PATH"})
	if err != nil || res == "" {
		t.Errorf("get_env PATH: %v %s", err, res)
	}

	res, err = r.Call("get_env", map[string]any{})
	if err != nil || !strings.Contains(res, "key is required") {
		t.Errorf("get_env missing: %s", res)
	}
}
