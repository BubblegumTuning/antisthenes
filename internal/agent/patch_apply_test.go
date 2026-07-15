package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseUnifiedDiff_SingleHunk(t *testing.T) {
	diff := `--- a/foo.txt
+++ b/foo.txt
@@ -1,3 +1,3 @@
 alpha
-beta
+BETA
 gamma
`
	target, hunks, err := parseUnifiedDiff(diff)
	if err != nil {
		t.Fatal(err)
	}
	if target != "foo.txt" || len(hunks) != 1 || len(hunks[0].lines) != 4 {
		t.Fatalf("parse: target=%q hunks=%d lines=%d", target, len(hunks), len(hunks[0].lines))
	}
}

func TestApplyPatchUnified_SingleHunk(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "foo.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0644); err != nil {
		t.Fatal(err)
	}

	diff := `--- a/foo.txt
+++ b/foo.txt
@@ -1,3 +1,3 @@
 alpha
-beta
+BETA
 gamma
`
	if err := applyPatchUnified(diff, path); err != nil {
		t.Fatalf("apply: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "alpha\nBETA\ngamma\n"
	if string(data) != want {
		t.Fatalf("got %q want %q", string(data), want)
	}
}

func TestApplyPatchDiff_StringReplaceStillWorks(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	path := "sample.go"
	if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewToolRegistry()
	diff := `--- a/sample.go
+++ b/sample.go
@@ -1,1 +1,2 @@
 package main
+// added
`
	res, err := r.Call("patch", map[string]any{"diff": diff})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "patch:") {
		t.Fatalf("unexpected result: %s", res)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "// added") {
		t.Fatalf("diff not applied: %s", string(data))
	}
}

func TestApplyPatchDiff_OldTextNewText(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	path := "file.txt"
	os.WriteFile(path, []byte("hello world"), 0644)

	r := NewToolRegistry()
	res, err := r.Call("patch", map[string]any{
		"path":     path,
		"old_text": "world",
		"new_text": "universe",
	})
	if err != nil || !strings.Contains(res, "string replace") {
		t.Fatalf("replace: %v %s", err, res)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello universe" {
		t.Fatalf("got %q", string(data))
	}
}
