package agent

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

func TestToolRegistry_DeleteMoveCopyFile(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	r := NewToolRegistry()
	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) {
		return true, ApprovalOnce
	})

	src := "source.txt"
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := r.Call("copy_file", map[string]any{"src": src, "dst": "copy.txt"})
	if err != nil || !strings.Contains(res, "Copied") {
		t.Fatalf("copy_file: %v %s", err, res)
	}
	data, err := os.ReadFile("copy.txt")
	if err != nil || string(data) != "hello" {
		t.Fatalf("copy not written: %v", err)
	}

	res, err = r.Call("move_file", map[string]any{"src": "copy.txt", "dst": "moved.txt"})
	if err != nil || !strings.Contains(res, "Moved") {
		t.Fatalf("move_file: %v %s", err, res)
	}
	if _, err := os.Stat("moved.txt"); err != nil {
		t.Fatalf("moved file missing: %v", err)
	}

	res, err = r.Call("delete_file", map[string]any{"path": "moved.txt"})
	if err != nil || !strings.Contains(res, "Deleted") {
		t.Fatalf("delete_file: %v %s", err, res)
	}
	if _, err := os.Stat("moved.txt"); !os.IsNotExist(err) {
		t.Fatalf("file should be deleted")
	}
}

func TestToolRegistry_DeleteFileRecursiveRequiresApproval(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	r := NewToolRegistry()
	res, err := r.Call("delete_file", map[string]any{"path": "tree", "recursive": true})
	if err != nil || !strings.Contains(res, "approval required") {
		t.Fatalf("expected approval gate for recursive delete: %v %s", err, res)
	}
}

func TestToolRegistry_FSToolsRejectUnsafePaths(t *testing.T) {
	r := NewToolRegistry()
	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) {
		return true, ApprovalOnce
	})

	for _, tool := range []struct {
		name string
		args map[string]any
	}{
		{"create_dir", map[string]any{"path": "/tmp/abs"}},
		{"delete_file", map[string]any{"path": "../escape"}},
		{"move_file", map[string]any{"src": "a", "dst": "/abs"}},
		{"copy_file", map[string]any{"src": "..", "dst": "b"}},
	} {
		res, err := r.Call(tool.name, tool.args)
		if err != nil {
			t.Fatalf("%s error: %v", tool.name, err)
		}
		if !strings.Contains(res, "unsafe") && !strings.Contains(res, "invalid") {
			t.Errorf("%s should reject unsafe path: %s", tool.name, res)
		}
	}
}

func TestToolRegistry_FileStatAndChmod(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	path := "perm.txt"
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := NewToolRegistry()
	r.SetApprovalHandler(func(ApprovalRequest) (bool, ApprovalLevel) {
		return true, ApprovalOnce
	})

	res, err := r.Call("file_stat", map[string]any{"path": path})
	if err != nil || !strings.Contains(res, "file_stat:") || !strings.Contains(res, "600") {
		t.Fatalf("file_stat: %v %s", err, res)
	}

	res, err = r.Call("chmod", map[string]any{"path": path, "mode": "0644"})
	if err != nil || !strings.Contains(res, "chmod:") {
		t.Fatalf("chmod: %v %s", err, res)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode got %o", info.Mode().Perm())
	}
}

func TestToolRegistry_BinaryFileRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	raw := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x00, 0xFF}
	encoded := base64.StdEncoding.EncodeToString(raw)

	r := NewToolRegistry()
	res, err := r.Call("write_file", map[string]any{
		"path":     "image.bin",
		"content":  encoded,
		"encoding": "base64",
		"mode":     "0644",
	})
	if err != nil || !strings.Contains(res, "encoding=base64") {
		t.Fatalf("write binary: %v %s", err, res)
	}

	res, err = r.Call("read_file", map[string]any{"path": "image.bin", "encoding": "text"})
	if err != nil || !strings.Contains(res, "binary file detected") {
		t.Fatalf("text read should reject binary: %v %s", err, res)
	}

	res, err = r.Call("read_file", map[string]any{"path": "image.bin", "encoding": "base64"})
	if err != nil || !strings.Contains(res, encoded) {
		t.Fatalf("read binary: %v %s", err, res)
	}

	data, err := os.ReadFile("image.bin")
	if err != nil || string(data) != string(raw) {
		t.Fatalf("file bytes mismatch: %v", err)
	}
}

func TestToolRegistry_ListDirNoBackendSuffix(t *testing.T) {
	r := NewToolRegistry()
	res, err := r.Call("list_dir", map[string]any{"path": "."})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res, "# listed via") {
		t.Errorf("list_dir should not include backend suffix: %s", res)
	}
}
