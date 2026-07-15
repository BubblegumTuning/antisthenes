package agent

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestToolRegistry_ListProcesses(t *testing.T) {
	r := NewToolRegistry()
	res, err := r.Call("list_processes", map[string]any{"max_results": 5})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "list_processes") {
		t.Fatalf("unexpected: %s", res)
	}
	lines := strings.Split(res, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected process lines: %s", res)
	}
}

func TestToolRegistry_KillProcess(t *testing.T) {
	cmd := exec.Command("sleep", "120")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	defer func() {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		_ = cmd.Wait()
	}()

	r := NewToolRegistry()
	res, err := r.Call("kill_process", map[string]any{"pid": pid})
	if err != nil || !strings.Contains(res, "approval required") {
		t.Fatalf("expected approval gate: %v %s", err, res)
	}

	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) {
		return true, ApprovalOnce
	})
	res, err = r.Call("kill_process", map[string]any{"pid": pid, "signal": "TERM"})
	if err != nil || !strings.Contains(res, "kill_process: sent") {
		t.Fatalf("kill: %v %s", err, res)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected process to exit")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit")
	}
}

func TestToolRegistry_KillProcessBlocksSelf(t *testing.T) {
	r := NewToolRegistry()
	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) {
		return true, ApprovalOnce
	})
	res, err := r.Call("kill_process", map[string]any{"pid": os.Getpid()})
	if err != nil || !strings.Contains(res, "agent process") {
		t.Fatalf("self kill block: %v %s", err, res)
	}
}
