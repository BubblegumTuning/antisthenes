package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanami/antisthenes/config"
)

func TestHandleSubcommand_Version(t *testing.T) {
	cfg := config.DefaultConfig()
	args := []string{"antisthenes", "version"}
	if !handleSubcommand(args, cfg) {
		t.Error("version should be handled")
	}
}

func TestHandleSubcommand_Config(t *testing.T) {
	cfg := config.DefaultConfig()
	args := []string{"antisthenes", "config"}
	if !handleSubcommand(args, cfg) {
		t.Error("config should be handled")
	}
}

func TestHandleSubcommand_Help(t *testing.T) {
	cfg := config.DefaultConfig()
	for _, h := range []string{"--help", "-h"} {
		args := []string{"antisthenes", h}
		if !handleSubcommand(args, cfg) {
			t.Errorf("%s should be handled", h)
		}
	}
}

func TestHandleSubcommand_Sessions(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	cfg := config.Config{
		DBPath: dbPath,
	}
	args := []string{"antisthenes", "sessions"}
	if !handleSubcommand(args, cfg) {
		t.Error("sessions should be handled")
	}
}

// MCP stdio must not emit non-JSON banners on stdout (protocol is JSON-RPC only).
func TestHandleSubcommand_MCP_StdoutClean(t *testing.T) {
	oldStdout := os.Stdout
	oldStdin := os.Stdin
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = wOut
	os.Stdin = rIn
	defer func() {
		os.Stdout = oldStdout
		os.Stdin = oldStdin
	}()

	// Immediate EOF so Server.Run returns without hanging.
	_ = wIn.Close()

	cfg := config.DefaultConfig()
	done := make(chan bool, 1)
	go func() {
		done <- handleSubcommand([]string{"antisthenes", "mcp"}, cfg)
	}()

	handled := <-done
	_ = wOut.Close()
	out := make([]byte, 4096)
	n, _ := rOut.Read(out)
	_ = rOut.Close()
	_ = rIn.Close()

	if !handled {
		t.Fatal("mcp should be handled")
	}
	if n > 0 {
		t.Fatalf("mcp subcommand wrote to stdout (must be JSON-RPC only): %q", string(out[:n]))
	}
}

func TestNewDefaultRegistryAndLoop(t *testing.T) {
	reg, loop := newDefaultRegistryAndLoop("dummy", "test-model", "http://example.invalid/v1", config.DefaultConfig())
	if reg == nil {
		t.Error("registry nil")
	}
	if loop == nil {
		t.Error("loop nil")
	}
	foundMCP := false
	for _, tool := range reg.ToOpenAITools() {
		if tool.Function != nil && tool.Function.Name == "mcp_call" {
			foundMCP = true
			break
		}
	}
	if !foundMCP {
		t.Error("mcp_call missing from default registry tool list")
	}
}

func TestNewToolRegistry_MCPServerOmitsAgentOnlyPacks(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := newToolRegistry(cfg, mcpServerRegistryOptions())
	names := map[string]bool{}
	for _, tool := range reg.ToOpenAITools() {
		if tool.Function != nil {
			names[tool.Function.Name] = true
		}
	}
	for _, forbidden := range []string{"mcp_call", "mcp_list_tools", "schedule_task", "list_tasks", "cancel_task", "list_aux_models", "complete_with_aux"} {
		if names[forbidden] {
			t.Errorf("MCP server registry must not include %s", forbidden)
		}
	}
	if !names["bash"] || !names["read_file"] {
		t.Error("MCP server registry missing core tools")
	}
}

func TestNewToolRegistry_AgentIncludesPacks(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := newToolRegistry(cfg, agentRegistryOptions())
	names := map[string]bool{}
	for _, tool := range reg.ToOpenAITools() {
		if tool.Function != nil {
			names[tool.Function.Name] = true
		}
	}
	for _, want := range []string{"mcp_call", "mcp_list_tools", "schedule_task", "list_tasks", "cancel_task"} {
		if !names[want] {
			t.Errorf("agent registry missing %s", want)
		}
	}
}

func TestTryRunOneShot_NoMatch(t *testing.T) {
	cfg := config.DefaultConfig()
	args := []string{"antisthenes", "foo"}
	if tryRunOneShot(args, cfg) {
		t.Error("should not handle non-prompt")
	}
}

func TestResolveOneShotPrompt_Inline(t *testing.T) {
	got, err := resolveOneShotPrompt("hello inline")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello inline" {
		t.Errorf("got %q, want %q", got, "hello inline")
	}
}

func TestResolveOneShotPrompt_FromFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "prompt.txt")
	if err := os.WriteFile(path, []byte("prompt from file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveOneShotPrompt("@" + path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "prompt from file" {
		t.Errorf("got %q, want %q", got, "prompt from file")
	}
}

func TestResolveOneShotPrompt_FromStdin(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte("prompt from stdin\n"))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	got, err := resolveOneShotPrompt("-")
	if err != nil {
		t.Fatal(err)
	}
	if got != "prompt from stdin" {
		t.Errorf("got %q, want %q", got, "prompt from stdin")
	}
}

func TestReadPromptFile_Empty(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.txt")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPromptFile(path); err == nil {
		t.Error("expected error for empty prompt file")
	}
}

func TestTryRunOneShot_WithPrompt_Mock(t *testing.T) {
	ts, cfg := newOneShotMockServer(t)
	defer ts.Close()

	// Capture stdout to avoid polluting test output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	args := []string{"antisthenes", "--prompt", "hello from test"}
	handled := tryRunOneShot(args, cfg)

	// restore
	w.Close()
	os.Stdout = oldStdout

	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !handled {
		t.Error("tryRunOneShot should return true for --prompt")
	}
	if !strings.Contains(output, "mocked one-shot response") {
		t.Errorf("expected mocked response in output, got: %s", output)
	}
}

func TestTryRunOneShot_FromFile_Mock(t *testing.T) {
	ts, cfg := newOneShotMockServer(t)
	defer ts.Close()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "prompt.txt")
	if err := os.WriteFile(path, []byte("hello from file"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	args := []string{"antisthenes", "--prompt", "@" + path}
	handled := tryRunOneShot(args, cfg)

	w.Close()
	os.Stdout = oldStdout

	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !handled {
		t.Error("tryRunOneShot should return true for @file prompt")
	}
	if !strings.Contains(output, "mocked one-shot response") {
		t.Errorf("expected mocked response in output, got: %s", output)
	}
}

func TestTryRunOneShot_FromStdin_Mock(t *testing.T) {
	ts, cfg := newOneShotMockServer(t)
	defer ts.Close()

	oldStdin := os.Stdin
	sr, sw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = sr
	go func() {
		_, _ = sw.Write([]byte("hello from stdin"))
		sw.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	args := []string{"antisthenes", "-P", "-"}
	handled := tryRunOneShot(args, cfg)

	w.Close()
	os.Stdout = oldStdout

	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !handled {
		t.Error("tryRunOneShot should return true for stdin prompt")
	}
	if !strings.Contains(output, "mocked one-shot response") {
		t.Errorf("expected mocked response in output, got: %s", output)
	}
}

func TestTryRunOneShot_PromptFile_Mock(t *testing.T) {
	ts, cfg := newOneShotMockServer(t)
	defer ts.Close()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "prompt.txt")
	if err := os.WriteFile(path, []byte("hello from prompt-file"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	args := []string{"antisthenes", "--prompt-file", path}
	handled := tryRunOneShot(args, cfg)

	w.Close()
	os.Stdout = oldStdout

	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !handled {
		t.Error("tryRunOneShot should return true for --prompt-file")
	}
	if !strings.Contains(output, "mocked one-shot response") {
		t.Errorf("expected mocked response in output, got: %s", output)
	}
}

func newOneShotMockServer(t *testing.T) (*httptest.Server, config.Config) {
	t.Helper()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "chatcmpl-cmdtest",
			"object": "chat.completion",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "mocked one-shot response",
					},
					"finish_reason": "stop",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	ep := config.Endpoint{
		Name:    "test",
		Model:   "test-model",
		BaseURL: ts.URL,
		APIKey:  "sk-dummy",
	}
	cfg := config.Config{
		DBPath:         dbPath,
		ActiveEndpoint: "test",
		Endpoints:      []config.Endpoint{ep},
	}
	return ts, cfg
}

func TestHandleSubcommand_Index(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	cfg := config.DefaultConfig()
	args := []string{"antisthenes", "index"}

	// Ensure skills/ dir exists; GenerateIndex writes skills/index.json without creating parents (hermetic)
	if err := os.MkdirAll("skills", 0755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}

	if !handleSubcommand(args, cfg) {
		t.Error("index should be handled")
	}

	// Verify index file was generated (hermetic in tmp)
	if _, err := os.Stat("skills/index.json"); err != nil {
		t.Errorf("expected skills/index.json to be created: %v", err)
	}
}

func TestHandleSubcommand_Model(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	// Seed a config.json so Load inside configureModel has data
	initial := config.DefaultConfig()
	initial.AgentName = "InitialAgent"
	if err := config.Save(initial); err != nil {
		t.Fatalf("seed Save failed: %v", err)
	}

	// Scripted input for blank choices (covers default edit path):
	// choice (blank), newName (blank), newModel, newURL, newKey
	input := "\n\n\n\n\n"

	// Redirect stdin
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte(input))
		w.Close()
	}()

	// Capture stdout
	oldStdout := os.Stdout
	ro, wo, _ := os.Pipe()
	os.Stdout = wo

	cfg := config.DefaultConfig()
	args := []string{"antisthenes", "model"}
	handled := handleSubcommand(args, cfg)

	// Restore
	wo.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	var buf [4096]byte
	n, _ := ro.Read(buf[:])
	output := string(buf[:n])

	if !handled {
		t.Error("model should be handled")
	}
	if !strings.Contains(output, "=== Configure Model Endpoint ===") ||
		!strings.Contains(output, "Endpoint configuration saved.") {
		t.Errorf("expected configure output in stdout, got: %q", output)
	}

	// Verify save updated something (file should exist)
	if _, err := os.Stat("config.json"); err != nil {
		t.Errorf("config.json should exist after configure: %v", err)
	}
}
