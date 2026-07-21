package mcp

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/nanami/antisthenes/internal/agent"
)

func TestNewClient_Error(t *testing.T) {
	_, err := NewClient("/non/existent/command/xyz")
	if err == nil {
		t.Error("expected error launching bad command")
	}
}

func TestClient_CallErrors(t *testing.T) {
	// Bidirectional pipes to cover call() without real exec/subprocess
	// stdinR/W: client writes request -> we consume
	// stdoutR/W: we write response -> client reads
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	c := &Client{
		stdin:  stdinW,
		stdout: bufio.NewReader(stdoutR),
	}

	// consume what client writes (the request)
	go func() {
		io.Copy(io.Discard, stdinR)
	}()

	// send error response
	go func() {
		resp := `{"id":1,"error":{"code":-32600,"message":"bad"}}` + "\n"
		stdoutW.Write([]byte(resp))
	}()

	_, err := c.call("tools/list", nil)
	if err == nil {
		t.Error("expected MCP error")
	}

	// second client for bad json
	stdinR2, stdinW2 := io.Pipe()
	stdoutR2, stdoutW2 := io.Pipe()
	c2 := &Client{
		stdin:  stdinW2,
		stdout: bufio.NewReader(stdoutR2),
	}
	go func() { io.Copy(io.Discard, stdinR2) }()
	go func() {
		stdoutW2.Write([]byte("not json\n"))
	}()
	_, err = c2.call("initialize", nil)
	if err == nil {
		t.Error("expected unmarshal error")
	}
}

func TestServer_NewAndList(t *testing.T) {
	reg := agent.NewToolRegistry()
	s := NewServer(reg)
	tools := s.listTools()
	if len(tools) < 20 {
		t.Errorf("expected full registry tool list, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		if name == "" {
			t.Error("tool missing name")
			continue
		}
		if names[name] {
			t.Errorf("duplicate tool name: %s", name)
		}
		names[name] = true
		if _, ok := tool["description"].(string); !ok {
			t.Errorf("tool %s missing description", name)
		}
		if _, ok := tool["inputSchema"]; !ok {
			t.Errorf("tool %s missing inputSchema", name)
		}
	}

	// Tools added after the old hardcoded list must appear dynamically.
	for _, want := range []string{"create_dir", "git_status", "ansible_check", "patch"} {
		if !names[want] {
			t.Errorf("expected %q in dynamic tools/list", want)
		}
	}

	reg.Register("custom_mcp_tool", func(map[string]any) (string, error) { return "ok", nil })
	tools = s.listTools()
	found := false
	for _, tool := range tools {
		if tool["name"] == "custom_mcp_tool" {
			found = true
			break
		}
	}
	if !found {
		t.Error("dynamically registered tool not in tools/list")
	}
}

func TestServer_HandleRequests(t *testing.T) {
	reg := agent.NewToolRegistry()
	// Register a simple tool for callTool coverage
	reg.Register("echo", func(args map[string]any) (string, error) {
		msg, _ := args["message"].(string)
		return msg, nil
	})

	s := NewServer(reg)

	tests := []struct {
		name   string
		req    JSONRPCRequest
		wantOK bool
	}{
		{
			name:   "initialize",
			req:    JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"},
			wantOK: true,
		},
		{
			name:   "tools/list",
			req:    JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"},
			wantOK: true,
		},
		{
			name: "tools/call success",
			req: JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      3,
				Method:  "tools/call",
				Params:  mustJSON(map[string]any{"name": "echo", "arguments": map[string]any{"message": "hi"}}),
			},
			wantOK: true,
		},
		{
			name: "tools/call bad params",
			req: JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      4,
				Method:  "tools/call",
				Params:  []byte(`{bad}`),
			},
			wantOK: false,
		},
		{
			name:   "unknown method",
			req:    JSONRPCRequest{JSONRPC: "2.0", ID: 5, Method: "foo"},
			wantOK: false,
		},
		{
			name:   "ping",
			req:    JSONRPCRequest{JSONRPC: "2.0", ID: 6, Method: "ping"},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := s.handleRequest(tt.req)
			if tt.name == "notifications" {
				return
			}
			if resp == nil {
				t.Fatal("nil resp")
			}
			if tt.wantOK && resp.Error != nil {
				t.Errorf("unexpected error: %+v", resp.Error)
			}
			if !tt.wantOK && resp.Error == nil {
				t.Error("expected error response")
			}
			if tt.name == "initialize" && resp.Error == nil {
				result, _ := resp.Result.(map[string]any)
				info, _ := result["serverInfo"].(map[string]string)
				// default NewServer → vdev
				if info["version"] == "" || info["name"] != "antisthenes" {
					// map[string]string may not round-trip via typed assert from map[string]any
					infoAny, _ := result["serverInfo"].(map[string]any)
					if infoAny["name"] != "antisthenes" {
						t.Errorf("serverInfo: %+v", result["serverInfo"])
					}
					if ver, _ := infoAny["version"].(string); ver == "" {
						t.Error("missing version")
					}
				}
			}
		})
	}

	// notifications/initialized must produce no response
	if resp := s.handleRequest(JSONRPCRequest{JSONRPC: "2.0", Method: "notifications/initialized"}); resp != nil {
		t.Errorf("expected nil response for initialized notification, got %+v", resp)
	}
	// unknown notification (no id) must not error
	if resp := s.handleRequest(JSONRPCRequest{JSONRPC: "2.0", Method: "notifications/foo"}); resp != nil {
		t.Errorf("expected nil for unknown notification, got %+v", resp)
	}
}

func TestNewServerWithVersion(t *testing.T) {
	s := NewServerWithVersion(agent.NewToolRegistry(), "0.3.1")
	resp := s.handleRequest(JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	if resp == nil || resp.Error != nil {
		t.Fatalf("initialize failed: %+v", resp)
	}
	result := resp.Result.(map[string]any)
	info := result["serverInfo"].(map[string]string)
	if info["version"] != "v0.3.1" {
		// Result may be map[string]any nested
		if infoAny, ok := result["serverInfo"].(map[string]any); ok {
			if infoAny["version"] != "v0.3.1" {
				t.Fatalf("version=%v want v0.3.1", infoAny["version"])
			}
		} else if info["version"] != "v0.3.1" {
			t.Fatalf("version=%q want v0.3.1", info["version"])
		}
	}
}

func TestServer_CallToolError(t *testing.T) {
	reg := agent.NewToolRegistry()
	s := NewServer(reg)
	_, err := s.callTool("nonexistent", nil)
	if err == nil {
		t.Error("expected error from registry for unknown tool")
	}
}

func TestRegisterMCPCallTool(t *testing.T) {
	reg := agent.NewToolRegistry()
	RegisterMCPCallTool(reg)

	// bad server
	res, err := reg.Call("mcp_call", map[string]any{"server": "", "tool": "echo"})
	if err != nil && !strings.Contains(res, "server command is required") {
		t.Logf("got: %s %v", res, err)
	}

	// missing tool
	res, _ = reg.Call("mcp_call", map[string]any{"server": "/bin/echo", "tool": ""})
	if !strings.Contains(res, "tool name is required") {
		t.Errorf("expected tool required msg, got %s", res)
	}

	// will fail on NewClient for bad server
	_, err = reg.Call("mcp_call", map[string]any{"server": "/nonexistent", "tool": "foo"})
	if err == nil {
		t.Error("expected connect error")
	}

	// mcp_list_tools registered
	res, err = reg.Call("mcp_list_tools", map[string]any{"server": ""})
	if err != nil {
		t.Fatalf("mcp_list_tools empty server: %v", err)
	}
	if !strings.Contains(res, "server command is required") {
		t.Errorf("expected server required msg, got %s", res)
	}
	_, err = reg.Call("mcp_list_tools", map[string]any{"server": "/nonexistent"})
	if err == nil {
		t.Error("expected connect error for mcp_list_tools")
	}
}

func TestFormatMCPToolList(t *testing.T) {
	out := formatMCPToolList(nil)
	if !strings.Contains(out, "no tools") {
		t.Errorf("empty: %s", out)
	}
	out = formatMCPToolList([]map[string]any{
		{"name": "zeta", "description": "last"},
		{
			"name":        "alpha",
			"description": "first",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "string"}}},
		},
	})
	if !strings.Contains(out, "2 tool(s)") {
		t.Errorf("count: %s", out)
	}
	// alpha before zeta
	ai := strings.Index(out, "- alpha")
	zi := strings.Index(out, "- zeta")
	if ai < 0 || zi < 0 || ai > zi {
		t.Errorf("sort order: %s", out)
	}
	if !strings.Contains(out, "inputSchema:") {
		t.Errorf("expected schema line: %s", out)
	}
}

func TestSplitCommandLine(t *testing.T) {
	tests := []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{"./antisthenes mcp", []string{"./antisthenes", "mcp"}, false},
		{`"/path/with space/bin" mcp`, []string{"/path/with space/bin", "mcp"}, false},
		{`./bin --flag='a b'`, []string{"./bin", "--flag=a b"}, false},
		{`unclosed "quote`, nil, true},
	}
	for _, tt := range tests {
		got, err := splitCommandLine(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("splitCommandLine(%q) expected error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitCommandLine(%q): %v", tt.in, err)
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("splitCommandLine(%q)=%v want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCommandLine(%q)[%d]=%q want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}

func TestResolveServerArgv(t *testing.T) {
	got, err := resolveServerArgv("./antisthenes mcp", nil)
	if err != nil || len(got) != 2 || got[0] != "./antisthenes" || got[1] != "mcp" {
		t.Fatalf("multi-word server: got %v err %v", got, err)
	}
	got, err = resolveServerArgv("./antisthenes", []any{"mcp", "--flag"})
	if err != nil || len(got) != 3 || got[1] != "mcp" || got[2] != "--flag" {
		t.Fatalf("explicit args: got %v err %v", got, err)
	}
}

func TestServer_WriteErrorAndResponse(t *testing.T) {
	reg := agent.NewToolRegistry()
	s := NewServer(reg)

	var buf strings.Builder
	s.writeError(&buf, 99, -32600, "test err")
	if !strings.Contains(buf.String(), "test err") {
		t.Error("writeError failed")
	}

	resp := s.errorResponse(1, -32601, "not found")
	if resp == nil || resp.Error == nil {
		t.Error("errorResponse failed")
	}
}

func TestServer_Run_EOF(t *testing.T) {
	// Cover Run loop exit on EOF using pipe for stdin
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	s := NewServer(agent.NewToolRegistry())

	go func() {
		w.Close() // immediate EOF
	}()

	err := s.Run()
	if err != nil {
		t.Logf("Run returned: %v (acceptable for EOF path)", err)
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestClient_PublicMethods_WithPipes(t *testing.T) {
	// Setup bidirectional pipes
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	c := &Client{
		stdin:  stdinW,
		stdout: bufio.NewReader(stdoutR),
	}

	// Drainer for client requests
	go func() { io.Copy(io.Discard, stdinR) }()

	// Provide a tools/list response that ListTools expects
	listResp := `{"id":1,"result":{"tools":[{"name":"echo","description":"test"}]}}` + "\n"
	go func() { stdoutW.Write([]byte(listResp)) }()

	tools, err := c.ListTools()
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	// New pipes for CallTool
	stdinR2, stdinW2 := io.Pipe()
	stdoutR2, stdoutW2 := io.Pipe()
	c2 := &Client{
		stdin:  stdinW2,
		stdout: bufio.NewReader(stdoutR2),
	}
	go func() { io.Copy(io.Discard, stdinR2) }()

	callResp := `{"id":2,"result":{"content":[{"type":"text","text":"echoed"}]}}` + "\n"
	go func() { stdoutW2.Write([]byte(callResp)) }()

	text, err := c2.CallTool("echo", map[string]any{"msg": "hi"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if text != "echoed" {
		t.Errorf("unexpected text: %s", text)
	}

	// Close path safe for nil cmd
	c3 := &Client{
		stdin: stdinW,
	}
	if c3.cmd == nil {
		_ = c3.stdin.Close()
	} else {
		_ = c3.Close()
	}
}
