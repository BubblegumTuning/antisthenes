package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial")
	return dir
}

func TestGitTools_StatusLogShow(t *testing.T) {
	repo := initGitRepo(t)
	r := NewToolRegistry()

	res, err := r.Call("git_status", map[string]any{"cwd": repo})
	if err != nil || !strings.Contains(res, "git_status:") {
		t.Fatalf("status: %v %s", err, res)
	}

	res, err = r.Call("git_log", map[string]any{"cwd": repo, "count": 5})
	if err != nil || !strings.Contains(res, "initial") {
		t.Fatalf("log: %v %s", err, res)
	}

	res, err = r.Call("git_show", map[string]any{"cwd": repo, "stat": true})
	if err != nil || !strings.Contains(res, "initial") {
		t.Fatalf("show: %v %s", err, res)
	}
}

func TestGitTools_MutatingWithApproval(t *testing.T) {
	repo := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewToolRegistry()
	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) {
		return true, ApprovalOnce
	})

	res, err := r.Call("git_add", map[string]any{"cwd": repo, "paths": "new.txt"})
	if err != nil || !strings.Contains(res, "git_add:") {
		t.Fatalf("add: %v %s", err, res)
	}

	res, err = r.Call("git_commit", map[string]any{"cwd": repo, "message": "add new file"})
	if err != nil || !strings.Contains(res, "git_commit:") {
		t.Fatalf("commit: %v %s", err, res)
	}

	res, err = r.Call("git_branch", map[string]any{"cwd": repo, "name": "feature/test"})
	if err != nil || !strings.Contains(res, "git_branch:") {
		t.Fatalf("branch: %v %s", err, res)
	}

	res, err = r.Call("git_checkout", map[string]any{"cwd": repo, "ref": "feature/test"})
	if err != nil || !strings.Contains(res, "git_checkout:") {
		t.Fatalf("checkout: %v %s", err, res)
	}
}

func TestGitTools_Diff(t *testing.T) {
	repo := initGitRepo(t)
	r := NewToolRegistry()
	res, err := r.Call("git_diff", map[string]any{"cwd": repo})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "git_diff") {
		t.Fatalf("diff: %s", res)
	}
}
