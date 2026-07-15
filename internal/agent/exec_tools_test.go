package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolRegistry_RunCommand(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	sub := filepath.Join(tmp, "work")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}

	r := NewToolRegistry()
	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) {
		return true, ApprovalOnce
	})

	relSub, err := filepath.Rel(tmp, sub)
	if err != nil {
		t.Fatal(err)
	}

	res, err := r.Call("run_command", map[string]any{
		"command": "pwd",
		"cwd":     relSub,
		"env":     map[string]any{"MARKER": "antisthenes"},
	})
	if err != nil {
		t.Fatalf("run_command: %v", err)
	}
	if !strings.Contains(res, "work") {
		t.Errorf("expected cwd in output: %s", res)
	}

	res, err = r.Call("run_command", map[string]any{
		"command": "echo $MARKER",
		"env":     map[string]any{"MARKER": "ok"},
	})
	if err != nil || !strings.Contains(res, "ok") {
		t.Fatalf("run_command env: %v %s", err, res)
	}

	res, err = r.Call("run_command", map[string]any{
		"command": "sleep 2",
		"timeout": 1,
	})
	if err != nil || !strings.Contains(res, "timed out") {
		t.Fatalf("run_command timeout: %v %s", err, res)
	}
}

func TestToolRegistry_RunCommandBackground(t *testing.T) {
	r := NewToolRegistry()
	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) {
		return true, ApprovalOnce
	})

	startRes, err := r.Call("run_command", map[string]any{
		"command":    "sleep 0.2 && echo bg-ok",
		"background": true,
	})
	if err != nil || !strings.Contains(startRes, "background job") {
		t.Fatalf("start: %v %s", err, startRes)
	}

	var jobID int
	if _, err := fmt.Sscanf(startRes, "run_command: started background job %d", &jobID); err != nil {
		t.Fatalf("parse job id from %q: %v", startRes, err)
	}

	listRes, err := r.Call("list_background_jobs", map[string]any{})
	if err != nil || !strings.Contains(listRes, "running") {
		t.Fatalf("list: %v %s", err, listRes)
	}

	waitRes, err := r.Call("wait_job", map[string]any{"job_id": jobID, "timeout": 5})
	if err != nil || !strings.Contains(waitRes, "bg-ok") {
		t.Fatalf("wait: %v %s", err, waitRes)
	}
}

func TestToolRegistry_ExecAndBash(t *testing.T) {
	r := NewToolRegistry()

	res, err := r.Call("bash", map[string]any{})
	if err != nil || !strings.Contains(res, "command is required") {
		t.Errorf("bash missing: %s", res)
	}

	res, err = r.Call("bash", map[string]any{"command": "echo hello exec"})
	if err != nil || !strings.Contains(res, "hello exec") {
		t.Errorf("bash safe: %v %s", err, res)
	}

	r.policy.Approve("rm -rf /", ApprovalOnce)
	res, err = r.Call("bash", map[string]any{"command": "rm -rf /"})
	if err != nil || !strings.Contains(res, "blocked") {
		t.Errorf("bash blocked: %s", res)
	}
}

func TestToolRegistry_BashApprovalHandler(t *testing.T) {
	r := NewToolRegistry()
	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) {
		return true, ApprovalOnce
	})
	res, err := r.Call("bash", map[string]any{"command": "rm -rf /tmp/testdir"})
	if err != nil || strings.Contains(res, "denied") || strings.Contains(res, "Approval required") {
		t.Errorf("expected handler approval: %v %s", err, res)
	}

	r2 := NewToolRegistry()
	r2.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) {
		return false, ApprovalOnce
	})
	res, err = r2.Call("bash", map[string]any{"command": "rm -rf /tmp/testdir"})
	if err != nil || !strings.Contains(res, "denied") {
		t.Errorf("expected user deny: %v %s", err, res)
	}
}

func TestToolRegistry_WaitJobTimeout(t *testing.T) {
	r := NewToolRegistry()
	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) {
		return true, ApprovalOnce
	})

	startRes, err := r.Call("run_command", map[string]any{
		"command":    "sleep 5",
		"background": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var jobID int
	fmt.Sscanf(startRes, "run_command: started background job %d", &jobID)

	res, err := r.Call("wait_job", map[string]any{"job_id": jobID, "timeout": 1})
	if err != nil || !strings.Contains(res, "still running") {
		t.Fatalf("wait timeout: %v %s", err, res)
	}
}
