package agent

import (
	"os"
	"strings"
	"testing"
)

func TestToolRegistry_ContextTools(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	r := NewToolRegistry()

	res, err := r.Call("dump_work_summary", map[string]any{})
	if err != nil || !strings.Contains(res, "summary is required") {
		t.Errorf("dump missing: %s", res)
	}

	res, err = r.Call("dump_work_summary", map[string]any{"summary": "test summary content", "session_id": "s1"})
	if err != nil || !strings.Contains(res, "dumped to") {
		t.Errorf("dump: %v %s", err, res)
	}

	res, err = r.Call("load_work_summary", map[string]any{})
	if err != nil || !strings.Contains(res, "path is required") {
		t.Errorf("load missing: %s", res)
	}

	_, _ = r.Call("load_work_summary", map[string]any{"path": "nonexistent.md"})
}
